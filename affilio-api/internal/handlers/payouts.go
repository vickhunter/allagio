package handlers

import (
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type PayoutsHandler struct {
	DB *database.DB
}

type payoutBatchRequest struct {
	MinPayout   float64 `json:"min_payout"`
	IncludeTest bool    `json:"include_test_mode"`
}

type markPaidRequest struct {
	ProviderReference string `json:"provider_reference"`
}

// Balance shows the publisher where their money is: pending (inside a refund
// window), cleared and awaiting the next payout batch, and already paid.
func (h *PayoutsHandler) Balance(c *gin.Context) {
	if _, err := h.DB.ClearMaturedLedgerEntries(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to settle ledger"})
		return
	}
	balances, err := h.DB.SiteBalances(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load balance"})
		return
	}
	out := make([]gin.H, 0, len(balances))
	for _, b := range balances {
		out = append(out, gin.H{
			"currency":         b.Currency,
			"mode":             b.Mode,
			"pending":          b.Pending,
			"cleared_unpaid":   b.ClearedUnpaid,
			"paid":             b.Paid,
			"reserved":         b.Reserved,
			"payment_reported": b.PaymentReported,
		})
	}
	c.JSON(http.StatusOK, gin.H{"balances": out})
}

func (h *PayoutsHandler) History(c *gin.Context) {
	siteID := siteIDFrom(c)
	payouts, err := h.DB.ListPayouts(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payouts"})
		return
	}
	entries, err := h.DB.ListLedgerHistory(c.Request.Context(), siteID, intQuery(c, "limit", 50))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ledger"})
		return
	}
	payoutList := make([]gin.H, 0, len(payouts))
	for _, p := range payouts {
		payoutList = append(payoutList, payoutJSON(p))
	}
	entryList := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		item := gin.H{
			"id":           e.ID,
			"kind":         e.Kind,
			"amount":       e.Amount,
			"currency":     e.Currency,
			"mode":         e.Mode,
			"status":       e.Status,
			"product_name": e.ProductName,
			"created_at":   e.CreatedAt.Format(time.RFC3339),
		}
		if e.ClearsAt != nil {
			item["clears_at"] = e.ClearsAt.Format(time.RFC3339)
		}
		entryList = append(entryList, item)
	}
	c.JSON(http.StatusOK, gin.H{"payouts": payoutList, "ledger": entryList})
}

// RunBatch is the admin's manual-payout trigger: clears matured holds, then
// creates one pending payout per site+currency at or above the minimum.
func (h *PayoutsHandler) RunBatch(c *gin.Context) {
	var req payoutBatchRequest
	_ = c.ShouldBindJSON(&req)
	if req.MinPayout <= 0 {
		req.MinPayout = 25
	}
	payouts, err := h.DB.RunPayoutBatch(c.Request.Context(), req.MinPayout, req.IncludeTest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payout batch failed"})
		return
	}
	out := make([]gin.H, 0, len(payouts))
	for _, p := range payouts {
		out = append(out, payoutJSON(p))
	}
	c.JSON(http.StatusOK, gin.H{"payouts": out, "min_payout": req.MinPayout})
}

func (h *PayoutsHandler) MarkPaid(c *gin.Context) {
	var req markPaidRequest
	if !uuidPattern.MatchString(c.Param("id")) || c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.ProviderReference) == "" || len(req.ProviderReference) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid payout id and payment reference required"})
		return
	}
	updated, err := h.DB.MarkPayoutPaid(c.Request.Context(), c.Param("id"), strings.TrimSpace(req.ProviderReference))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark payout paid"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "no pending payout with that id"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payout_id": c.Param("id"), "status": "paid"})
}

func (h *PayoutsHandler) MakerEarnings(c *gin.Context) {
	earnings, err := h.DB.MakerEarnings(c.Request.Context(), makerID(c), c.GetString("offer_scope_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load earnings"})
		return
	}
	offers := make([]gin.H, 0, len(earnings))
	totals := map[string]gin.H{}
	for _, e := range earnings {
		offers = append(offers, gin.H{
			"offer_id":     e.OfferID,
			"product_name": e.ProductName,
			"status":       e.Status,
			"clicks":       e.Clicks,
			"sales":        e.Sales,
			"refunds":      e.Refunds,
			"revenue":      e.Revenue,
			"commission":   e.Commission,
			"currency":     e.Currency,
			"mode":         e.Mode,
		})
		key := e.Currency + ":" + e.Mode
		total, ok := totals[key]
		if !ok {
			total = gin.H{"currency": e.Currency, "mode": e.Mode, "revenue": 0.0, "commission": 0.0}
			totals[key] = total
		}
		total["revenue"] = total["revenue"].(float64) + e.Revenue
		total["commission"] = total["commission"].(float64) + e.Commission
	}
	totalList := make([]gin.H, 0, len(totals))
	for _, t := range totals {
		totalList = append(totalList, t)
	}
	c.JSON(http.StatusOK, gin.H{"offers": offers, "totals": totalList})
}

func payoutJSON(p database.Payout) gin.H {
	item := gin.H{
		"id":          p.ID,
		"site_id":     p.SiteID,
		"amount":      p.Amount,
		"currency":    p.Currency,
		"maker_id":    p.MakerID,
		"mode":        p.Mode,
		"received_at": p.ReceivedAt,
		"method":      p.Method,
		"status":      p.Status,
		"created_at":  p.CreatedAt.Format(time.RFC3339),
	}
	if p.ProviderReference != "" {
		item["provider_reference"] = p.ProviderReference
	}
	if p.PaidAt != nil {
		item["paid_at"] = p.PaidAt.Format(time.RFC3339)
	}
	return item
}

func (h *PayoutsHandler) ConfirmReceived(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payout id"})
		return
	}
	updated, err := h.DB.ConfirmPayoutReceived(c.Request.Context(), siteIDFrom(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "confirmation failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusConflict, gin.H{"error": "payout not awaiting receipt confirmation"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "received", "payout_id": c.Param("id")})
}
