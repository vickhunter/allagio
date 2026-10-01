package handlers

import (
	"strings"

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
	ok, err := h.DB.ReportMerchantPayment(c.Request.Context(), ownerFrom(c).MakerID, c.Param("id"), req.Reference)
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
	if err := h.DB.SavePayoutInstructions(c.Request.Context(), ownerFrom(c).SiteID, req.Instructions); err != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"saved": true})
}
