package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"

	"affilio-api/internal/database"
	"affilio-api/internal/services/providers"
	"github.com/gin-gonic/gin"
)

type WebhooksHandler struct {
	DB      *database.DB
	Secrets map[string]string // provider name -> webhook signing secret
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Receive ingests a payment-provider webhook: verify signature, store the raw
// event (the idempotency barrier), then book the conversion or refund. Business
// outcomes that need no retry (unattributed, unmatched, ignored) return 200 so
// providers don't redeliver forever; only transport/auth problems are errors.
func (h *WebhooksHandler) Receive(c *gin.Context) {
	providerName := c.Param("provider")
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
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
		return
	}

	status, conversionID := h.processEvent(c, providerName, event)
	response := gin.H{"status": status}
	if conversionID != "" {
		response["conversion_id"] = conversionID
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
			markProcessed("processing failed")
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
				markProcessed("unmatched refund")
				return "unmatched", ""
			}
			log.Printf("webhooks.receive refund failed provider=%s event=%s err=%v", providerName, event.EventID, err)
			markProcessed("processing failed")
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
