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

// PublicOffers serves the listed-offer feed so an agent can browse supply
// before joining — the same terms list_marketplace_offers shows a site, minus
// per-site ranking and tracking fields.
func (h *AgentEntry) PublicOffers(c *gin.Context) {
	offers, err := h.DB.ListPublicOffers(c.Request.Context(), 50)
	if err != nil {
		c.Status(500)
		return
	}
	out := make([]gin.H, 0, len(offers))
	for _, offer := range offers {
		out = append(out, gin.H{
			"offer_id":             offer.OfferID,
			"product_name":         offer.ProductName,
			"product_tagline":      offer.ProductTagline,
			"website_url":          offer.WebsiteURL,
			"primary_category":     offer.PrimaryCategory,
			"keywords":             offer.Keywords,
			"tags":                 offer.Tags,
			"commission_type":      offer.CommissionType,
			"commission_value":     offer.CommissionValue,
			"commission_currency":  offer.CommissionCurrency,
			"cookie_duration_days": offer.CookieDurationDays,
			"refund_window_days":   offer.RefundWindowDays,
			"terms":                offer.Terms,
		})
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(200, gin.H{"offers": out})
}
