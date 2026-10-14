package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type RefundReportHandler struct {
	DB *database.DB
}

type reportSaleRequest struct {
	ClickID     string `json:"click_id"`
	EventID     string `json:"event_id"`
	ObjectID    string `json:"object_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	OccurredAt  string `json:"occurred_at"`
}

type reportRefundRequest struct {
	EventID     string `json:"event_id"`
	ObjectID    string `json:"object_id"`
	Currency    string `json:"currency"`
	AmountMinor *int64 `json:"amount_refunded_minor"`
	Full        *bool  `json:"full"`
	OccurredAt  string `json:"occurred_at"`
}

func (h *RefundReportHandler) ReportSale(c *gin.Context) {
	var req reportSaleRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if !uuidPattern.MatchString(req.ClickID) || !validReportID(req.EventID) || !validReportID(req.ObjectID) || req.AmountMinor <= 0 || !validReportCurrency(currency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "click, event, object, amount, and currency are required"})
		return
	}
	occurred, ok := reportTime(req.OccurredAt)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at must be RFC3339"})
		return
	}
	click, err := h.DB.ClickSaleContext(c.Request.Context(), req.ClickID)
	if errors.Is(err, database.ErrClickNotFound) || (err == nil && (click.MakerID != makerID(c) || (c.GetString("offer_scope_id") != "" && click.OfferID != c.GetString("offer_scope_id")))) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale was not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sale was not recorded"})
		return
	}
	if currency != strings.ToUpper(click.Currency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency does not match the click"})
		return
	}
	sale, err := h.DB.RecordReportedSale(c.Request.Context(), click.ProductID, req.ClickID, req.EventID, req.ObjectID, req.AmountMinor, currency, occurred)
	if errors.Is(err, database.ErrClickNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale was not found"})
		return
	}
	if err != nil || sale == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sale was not recorded"})
		return
	}
	status := "recorded"
	if sale.Duplicate {
		status = "duplicate"
	}
	body := gin.H{"status": status, "trust_level": sale.TrustLevel}
	if sale.ConversionID != "" {
		body["conversion_id"] = sale.ConversionID
	}
	c.JSON(http.StatusOK, body)
}

func (h *RefundReportHandler) ReportRefund(c *gin.Context) {
	var req reportRefundRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if !validReportID(req.EventID) || !validReportID(req.ObjectID) || !validReportCurrency(currency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event, object, and currency are required"})
		return
	}
	occurred, ok := reportTime(req.OccurredAt)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at must be RFC3339"})
		return
	}
	saleCurrency, found, err := h.DB.SaleForMaker(c.Request.Context(), makerID(c), req.ObjectID, c.GetString("offer_scope_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refund was not recorded"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale was not found"})
		return
	}
	if currency != strings.ToUpper(saleCurrency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency does not match the sale"})
		return
	}
	amount := int64(0)
	if req.AmountMinor != nil {
		amount = *req.AmountMinor
	}
	fullRefund := req.AmountMinor == nil && (req.Full == nil || *req.Full)
	out, err := h.DB.RecordPartialRefund(c.Request.Context(), database.ProviderSellerReport, req.EventID, req.ObjectID, amount, occurred, database.TrustSellerReported, makerID(c), c.GetString("offer_scope_id"), fullRefund)
	if errors.Is(err, database.ErrConversionNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale was not found"})
		return
	}
	if err != nil || out == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refund was not recorded"})
		return
	}
	c.JSON(http.StatusOK, refundResult(out, database.TrustSellerReported).json())
}

func validReportID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 255
}

func validReportCurrency(currency string) bool {
	return currency == "USD" || currency == "EUR" || currency == "GBP"
}

func reportTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC(), true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func conversionsJSON(rows []database.ConversionReport) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		var status any
		if row.RefundStatus != nil {
			status = *row.RefundStatus
		}
		out = append(out, gin.H{
			"id":            row.ID,
			"kind":          row.Kind,
			"trust_level":   row.TrustLevel,
			"refund_status": status,
			"reason":        row.Reason,
			"amount":        row.Amount,
			"currency":      row.Currency,
			"object_id":     row.ObjectID,
		})
	}
	return out
}
