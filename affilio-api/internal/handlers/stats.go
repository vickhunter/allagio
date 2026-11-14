package handlers

import "github.com/gin-gonic/gin"

// PublicStats serves the aggregate counters the landing page animates.
// Counts only — no offer, product or owner fields leave this endpoint.
func (h *AgentEntry) PublicStats(c *gin.Context) {
	deals, offers, products, err := h.DB.PublicStats(c.Request.Context())
	if err != nil {
		c.Status(500)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(200, gin.H{"deals": deals, "offers": offers, "products": products})
}
