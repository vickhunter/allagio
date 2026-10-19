package handlers

import (
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func (h *HostedHandler) MerchantPayments(c *gin.Context) {
	payouts, err := h.DB.MerchantPayouts(c.Request.Context(), ownerFrom(c).MakerID)
	if err != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"payouts": payouts})
}

func (h *HostedHandler) ReportMerchantPayment(c *gin.Context) {
	var req struct {
		Reference string `json:"reference"`
		Sent      bool   `json:"sent"`
	}
	if !uuidPattern.MatchString(c.Param("id")) || bindBoundedJSON(c, &req) != nil || !req.Sent || strings.TrimSpace(req.Reference) == "" || len(req.Reference) > 255 {
		c.JSON(400, gin.H{"error": "Confirm payment was sent and enter its payment reference."})
		return
	}
	owner := ownerFrom(c)
	var siteID string
	if err := h.DB.Pool.QueryRow(c.Request.Context(), `SELECT COALESCE(site_id::text,'') FROM payouts WHERE id=$1 AND maker_id=$2`, c.Param("id"), owner.MakerID).Scan(&siteID); err != nil {
		c.JSON(409, gin.H{"error": "A pending live payout owned by your account is required."})
		return
	}
	ref := strings.TrimSpace(req.Reference)
	if holdStep(c, h.DB, database.StepPayout, owner.ID, siteID, map[string]any{"action": "report_payment", "payout_id": c.Param("id"), "maker_id": owner.MakerID, "reference": ref}) {
		return
	}
	ok, err := h.DB.ReportMerchantPayment(c.Request.Context(), owner.MakerID, c.Param("id"), ref)
	if err != nil {
		c.Status(500)
		return
	}
	if !ok {
		c.JSON(409, gin.H{"error": "A pending live payout owned by your account is required."})
		return
	}
	c.JSON(200, gin.H{"status": "payment_reported"})
}

func (h *HostedHandler) PayoutProfile(c *gin.Context) {
	instructions, err := h.DB.PayoutInstructions(c.Request.Context(), ownerFrom(c).SiteID)
	if err != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"instructions": instructions})
}

func (h *HostedHandler) SavePayoutProfile(c *gin.Context) {
	var req struct {
		Instructions string `json:"instructions"`
	}
	if bindBoundedJSON(c, &req) != nil || len(req.Instructions) > 2000 {
		c.JSON(400, gin.H{"error": "Payment instructions must be at most 2000 characters."})
		return
	}
	owner := ownerFrom(c)
	if holdStep(c, h.DB, database.StepPayout, owner.ID, owner.SiteID, map[string]any{"action": "save_profile", "instructions": req.Instructions}) {
		return
	}
	if err := h.DB.SavePayoutInstructions(c.Request.Context(), owner.SiteID, req.Instructions); err != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"saved": true})
}
