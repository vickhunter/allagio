package handlers

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"affilio-api/internal/services/providers"
	"github.com/gin-gonic/gin"
)

type WebhooksHandler struct {
	DB      *database.DB
	Secrets map[string]string // provider name -> webhook signing secret
	Cipher  *services.ConnectionCipher
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type webhookResult struct {
	Status       string
	ConversionID string
	TrustLevel   string
	RefundStatus string
	Reason       string
}

func (r webhookResult) json() gin.H {
	body := gin.H{"status": r.Status}
	if r.ConversionID != "" {
		body["conversion_id"] = r.ConversionID
	}
	if r.TrustLevel != "" {
		body["trust_level"] = r.TrustLevel
	}
	if r.RefundStatus != "" {
		body["refund_status"] = r.RefundStatus
	}
	if r.Reason != "" {
		body["reason"] = r.Reason
	}
	return body
}

func refundResult(out *database.RefundOutcome, fallbackTrust string) webhookResult {
	if out == nil {
		return webhookResult{Status: "duplicate", TrustLevel: fallbackTrust, RefundStatus: database.RefundApplied}
	}
	status := out.Status
	if status == database.RefundApplied {
		status = "refunded"
	}
	row := out.RowStatus
	if row == "" && status != "duplicate" {
		row = out.Status
	}
	trust := out.TrustLevel
	if trust == "" {
		trust = fallbackTrust
	}
	return webhookResult{Status: status, ConversionID: out.ID, TrustLevel: trust, RefundStatus: row, Reason: out.Reason}
}

func (h *WebhooksHandler) duplicateResult(ctx context.Context, provider, eventID, fallbackTrust string) (webhookResult, error) {
	result := webhookResult{Status: "duplicate", TrustLevel: fallbackTrust}
	id, trust, refundStatus, reason, ok, err := h.DB.ConversionByProviderEvent(ctx, provider, eventID)
	if err != nil || !ok {
		return result, err
	}
	result.ConversionID = id
	if trust != "" {
		result.TrustLevel = trust
	}
	result.RefundStatus = refundStatus
	result.Reason = reason
	return result, nil
}

func (h *WebhooksHandler) Receive(c *gin.Context) {
	providerName := c.Param("provider")
	if c.Param("product_id") != "" {
		h.receiveProduct(c)
		return
	}
	if providerName == "stripe" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stripe requires a product-scoped connection"})
		return
	}
	adapter := providers.ByName(providerName)
	if adapter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}
	secret := h.Secrets[providerName]
	if secret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider webhook not configured"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1024*1024))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	if !adapter.VerifySignature(c.Request.Header, body, secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}
	event, err := adapter.ParseEvent(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unparseable event"})
		return
	}
	inserted, err := h.DB.InsertWebhookEvent(c.Request.Context(), providerName, event.EventID, body, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store event"})
		return
	}
	if !inserted {
		var processed bool
		if err := h.DB.Pool.QueryRow(c.Request.Context(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider=$1 AND provider_event_id=$2`, providerName, event.EventID).Scan(&processed); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "event lookup failed"})
			return
		}
		if processed {
			result, err := h.duplicateResult(c.Request.Context(), providerName, event.EventID, database.TrustSellerSignedWebhook)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "event lookup failed"})
				return
			}
			c.JSON(http.StatusOK, result.json())
			return
		}
	}

	result := h.processEvent(c, providerName, event)
	if result.Status == "error" || result.Status == "unmatched" {
		c.JSON(http.StatusServiceUnavailable, result.json())
		return
	}
	c.JSON(http.StatusOK, result.json())
}

func (h *WebhooksHandler) processEvent(c *gin.Context, providerName string, event *providers.ConversionEvent) webhookResult {
	ctx := c.Request.Context()
	markProcessed := func(errText string) {
		if err := h.DB.MarkWebhookProcessed(ctx, providerName, event.EventID, errText); err != nil {
			log.Printf("webhooks.receive mark processed failed provider=%s event=%s err=%v", providerName, event.EventID, err)
		}
	}
	switch event.Kind {
	case "sale", "renewal":
		if !uuidPattern.MatchString(event.Ref) {
			markProcessed("unattributed: missing or invalid ref")
			return webhookResult{Status: "unattributed"}
		}
		conversion, err := h.DB.RecordSaleConversion(ctx, event.Ref, providerName, event.EventID, event.ObjectID, event.Kind, event.Amount, event.Currency, event.OccurredAt)
		if err != nil {
			if errors.Is(err, database.ErrClickNotFound) {
				markProcessed("unattributed: ref does not match a click")
				return webhookResult{Status: "unattributed"}
			}
			log.Printf("webhooks.receive conversion failed provider=%s event=%s err=%v", providerName, event.EventID, err)
			return webhookResult{Status: "error"}
		}
		markProcessed("")
		if conversion == nil || conversion.Duplicate {
			trust := database.TrustSellerSignedWebhook
			if conversion != nil && conversion.TrustLevel != "" {
				trust = conversion.TrustLevel
			}
			return webhookResult{Status: "duplicate", TrustLevel: trust}
		}
		return webhookResult{Status: "recorded", ConversionID: conversion.ConversionID, TrustLevel: conversion.TrustLevel}
	case "refund":
		out, err := h.DB.RecordRefund(ctx, providerName, event.EventID, event.ObjectID, event.OccurredAt, database.TrustSellerSignedWebhook)
		if err != nil {
			if errors.Is(err, database.ErrConversionNotFound) {
				return webhookResult{Status: "unmatched"}
			}
			log.Printf("webhooks.receive refund failed provider=%s event=%s err=%v", providerName, event.EventID, err)
			return webhookResult{Status: "error"}
		}
		markProcessed("")
		return refundResult(out, database.TrustSellerSignedWebhook)
	default:
		markProcessed("")
		return webhookResult{Status: "ignored"}
	}
}

func (h *WebhooksHandler) receiveProduct(c *gin.Context) {
	productID := c.Param("product_id")
	if c.Param("provider") != "stripe" || !uuidPattern.MatchString(productID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown connection"})
		return
	}
	if h.Cipher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "connections are not configured"})
		return
	}
	connection, err := h.DB.ProductConnection(c.Request.Context(), productID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown connection"})
		return
	}
	secret, err := h.Cipher.Decrypt(productID, connection.SecretCiphertext)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "connection unavailable"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "invalid or oversized body"})
		return
	}
	adapter := &providers.Stripe{}
	if !adapter.VerifySignature(c.Request.Header, body, secret) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}
	event, err := adapter.ParseEvent(body)
	if err != nil {
		log.Printf("webhooks.stripe ignored product=%s reason=%v", productID, err)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}
	if event.LiveMode != connection.LiveMode {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event does not match product, mode or currency"})
		return
	}
	if !stripeEventIsOurs(event, productID, connection.Currency) {
		log.Printf("webhooks.stripe ignored product=%s event=%s kind=%s", productID, event.EventID, event.Kind)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}
	processed, canonical, err := h.DB.StoreProductWebhook(c.Request.Context(), productID, "stripe", event.EventID, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event storage failed"})
		return
	}
	if processed {
		result, err := h.duplicateResult(c.Request.Context(), "stripe", event.EventID, database.TrustProviderVerified)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "event lookup failed"})
			return
		}
		c.JSON(http.StatusOK, result.json())
		return
	}
	event, err = adapter.ParseEvent(canonical)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending", "reason": "stored event needs reconciliation"})
		return
	}
	result, err := h.processProductEvent(c.Request.Context(), productID, event)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending", "reason": result.Status})
		return
	}
	c.JSON(http.StatusOK, result.json())
}

func (h *WebhooksHandler) processProductEvent(ctx context.Context, productID string, event *providers.ConversionEvent) (webhookResult, error) {
	result := webhookResult{Status: "ignored"}
	var err error
	switch event.Kind {
	case "sale":
		if !uuidPattern.MatchString(event.Ref) {
			result.Status = "unattributed"
			break
		}
		var valid bool
		valid, err = h.DB.ProductOwnsClick(ctx, productID, event.Ref, event.Currency, event.OccurredAt)
		if err == nil && !valid {
			result.Status = "unattributed"
			break
		}
		if err == nil {
			var sale *database.SaleConversion
			sale, err = h.DB.RecordStripeSale(ctx, productID, event.Ref, event.EventID, event.ObjectID, event.AmountMinor, event.GrossMinor, event.Currency, event.LiveMode, event.OccurredAt)
			result.Status = "duplicate"
			result.TrustLevel = database.TrustProviderVerified
			if sale != nil && !sale.Duplicate {
				result.Status = "recorded"
				result.ConversionID = sale.ConversionID
				result.TrustLevel = sale.TrustLevel
			} else if sale != nil && sale.TrustLevel != "" {
				result.TrustLevel = sale.TrustLevel
			}
		}
	case "refund":
		var valid bool
		valid, err = h.DB.ProductOwnsPayment(ctx, productID, event.ObjectID, event.Currency)
		if err == nil && !valid {
			result.Status = "rejected"
			break
		}
		if err == nil {
			var out *database.RefundOutcome
			var makerID string
			makerID, err = h.DB.MakerIDForProduct(ctx, productID)
			if err == nil {
				out, err = h.DB.RecordPartialRefund(ctx, "stripe", event.EventID, event.ObjectID, event.RefundedMinor, event.OccurredAt, database.TrustProviderVerified, makerID, "")
			}
			if err == nil {
				result = refundResult(out, database.TrustProviderVerified)
			}
		}
	}
	if err != nil {
		if retryErr := h.DB.RetryWebhook(ctx, "stripe", event.EventID, "awaiting reconciliation"); retryErr != nil {
			return webhookResult{Status: "storage failure"}, retryErr
		}
		return webhookResult{Status: "awaiting reconciliation"}, err
	}
	reason := ""
	if result.Status == "unattributed" || result.Status == "rejected" || result.Status == "pending_review" {
		reason = result.Status
		if result.Reason != "" {
			reason = result.Reason
		}
	}
	if err := h.DB.MarkWebhookProcessed(ctx, "stripe", event.EventID, reason); err != nil {
		return webhookResult{Status: "storage failure"}, err
	}
	return result, nil
}

func stripeEventIsOurs(event *providers.ConversionEvent, productID, currency string) bool {
	switch event.Kind {
	case "sale":
		return event.ProductID == productID && event.Currency == currency
	case "refund":
		return event.Currency == currency
	default:
		return false
	}
}

func (h *WebhooksHandler) ReconcilePending(ctx context.Context) error {
	events, err := h.DB.PendingWebhooks(ctx)
	if err != nil {
		return err
	}
	for _, stored := range events {
		event, err := (&providers.Stripe{}).ParseEvent(stored.Payload)
		if err != nil {
			if err := h.DB.RetryWebhook(ctx, stored.Provider, stored.EventID, "unsupported stored event"); err != nil {
				return err
			}
			continue
		}
		if _, err = h.processProductEvent(ctx, stored.ProductID, event); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

func (h *WebhooksHandler) RunWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := h.ReconcilePending(jobCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("payment reconciliation failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
