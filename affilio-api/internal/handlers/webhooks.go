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
			c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
			return
		}
	}

	status, conversionID := h.processEvent(c, providerName, event)
	response := gin.H{"status": status}
	if conversionID != "" {
		response["conversion_id"] = conversionID
	}
	if status == "error" || status == "unmatched" {
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *WebhooksHandler) processEvent(c *gin.Context, providerName string, event *providers.ConversionEvent) (status, conversionID string) {
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
			return "unattributed", ""
		}
		conversion, err := h.DB.RecordSaleConversion(ctx, event.Ref, providerName, event.EventID, event.ObjectID, event.Kind, event.Amount, event.Currency, event.OccurredAt)
		if err != nil {
			if errors.Is(err, database.ErrClickNotFound) {
				markProcessed("unattributed: ref does not match a click")
				return "unattributed", ""
			}
			log.Printf("webhooks.receive conversion failed provider=%s event=%s err=%v", providerName, event.EventID, err)
			return "error", ""
		}
		markProcessed("")
		if conversion == nil {
			return "duplicate", ""
		}
		return "recorded", conversion.ConversionID
	case "refund":
		refundID, err := h.DB.RecordRefund(ctx, providerName, event.EventID, event.ObjectID, event.OccurredAt)
		if err != nil {
			if errors.Is(err, database.ErrConversionNotFound) {
				return "unmatched", ""
			}
			log.Printf("webhooks.receive refund failed provider=%s event=%s err=%v", providerName, event.EventID, err)
			return "error", ""
		}
		markProcessed("")
		if refundID == "" {
			return "duplicate", ""
		}
		return "refunded", refundID
	default:
		markProcessed("")
		return "ignored", ""
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}
	event, err := adapter.ParseEvent(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported event"})
		return
	}
	if event.LiveMode != connection.LiveMode || ((event.Kind == "sale" || event.Kind == "refund") && event.Currency != connection.Currency) || (event.Kind == "sale" && event.ProductID != productID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event does not match product, mode or currency"})
		return
	}
	processed, canonical, err := h.DB.StoreProductWebhook(c.Request.Context(), productID, "stripe", event.EventID, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event storage failed"})
		return
	}
	if processed {
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
		return
	}
	event, err = adapter.ParseEvent(canonical)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending", "reason": "stored event needs reconciliation"})
		return
	}
	status, id, err := h.processProductEvent(c.Request.Context(), productID, event)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending", "reason": status})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "conversion_id": id})
}

func (h *WebhooksHandler) processProductEvent(ctx context.Context, productID string, event *providers.ConversionEvent) (string, string, error) {
	status, id := "ignored", ""
	var err error
	switch event.Kind {
	case "sale":
		if !uuidPattern.MatchString(event.Ref) {
			status = "unattributed"
			break
		}
		var valid bool
		valid, err = h.DB.ProductOwnsClick(ctx, productID, event.Ref, event.Currency, event.OccurredAt)
		if err == nil && !valid {
			status = "unattributed"
			break
		}
		if err == nil {
			var sale *database.SaleConversion
			sale, err = h.DB.RecordStripeSale(ctx, productID, event.Ref, event.EventID, event.ObjectID, event.AmountMinor, event.GrossMinor, event.Currency, event.LiveMode, event.OccurredAt)
			status = "duplicate"
			if sale != nil {
				status = "recorded"
				id = sale.ConversionID
			}
		}
	case "refund":
		var valid bool
		valid, err = h.DB.ProductOwnsPayment(ctx, productID, event.ObjectID, event.Currency)
		if err == nil && !valid {
			status = "rejected"
			break
		}
		if err == nil {
			id, err = h.DB.RecordPartialRefund(ctx, "stripe", event.EventID, event.ObjectID, event.RefundedMinor, event.OccurredAt)
			status = "refunded"
			if id == "" {
				status = "duplicate"
			}
		}
	}
	if err != nil {
		if retryErr := h.DB.RetryWebhook(ctx, "stripe", event.EventID, "awaiting reconciliation"); retryErr != nil {
			return "storage failure", "", retryErr
		}
		return "awaiting reconciliation", "", err
	}
	reason := ""
	if status == "unattributed" || status == "rejected" {
		reason = status
	}
	if err := h.DB.MarkWebhookProcessed(ctx, "stripe", event.EventID, reason); err != nil {
		return "storage failure", "", err
	}
	return status, id, nil
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
		if _, _, err = h.processProductEvent(ctx, stored.ProductID, event); err != nil && ctx.Err() != nil {
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
