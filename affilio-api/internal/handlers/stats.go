package handlers

import (
	"net/http"
	"net/url"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

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

// PublicDeals is GET /v1/deals/recent — the live network wall behind the
// landing. Each row carries the deal kind, state, timing and a display label
// per party. A party is named only when the owner is directory-listed (a
// listed, live standing offer — the same consent the makers directory
// applies); everyone else is an anonymized role. Deal ids never leave:
// a public id would let anyone enumerate deal relationships.
func (h *AgentEntry) PublicDeals(c *gin.Context) {
	deals, err := h.DB.RecentORPDeals(c.Request.Context(), 24)
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	local := ""
	if parsed, err := url.Parse(h.BaseURL); err == nil {
		local = parsed.Host
	}
	ownerIDs := map[string]bool{}
	for _, deal := range deals {
		if deal.OffererOwner != "" {
			ownerIDs[deal.OffererOwner] = true
		}
		if deal.AcceptorOwner != "" {
			ownerIDs[deal.AcceptorOwner] = true
		}
	}
	ids := make([]string, 0, len(ownerIDs))
	for id := range ownerIDs {
		ids = append(ids, id)
	}
	faces, err := h.DB.PublicPartyFaces(c.Request.Context(), ids)
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	now := time.Now()
	out := make([]gin.H, 0, len(deals))
	for _, deal := range deals {
		terms, _ := decodeObject(deal.Terms)
		kind, _ := terms["kind"].(string)
		parties := 2
		if many, err := orp.DecodeMany(deal.Parties); err == nil && len(many.Seats) > 0 {
			parties = len(many.Seats)
		}
		// A deal with no acceptor seat stores the first seat's owner as a
		// fallback AcceptorOwner; without a party id there is no counterparty
		// to name, so the right side stays anonymous.
		acceptor := gin.H{"listed": false, "role": "agent"}
		if deal.AcceptorParty != "" {
			acceptor = partyFace(deal.AcceptorParty, deal.AcceptorOwner, faces, local, now)
		}
		out = append(out, gin.H{
			"kind":       kind,
			"state":      deal.State,
			"updated_at": deal.UpdatedAt.UTC().Format(time.RFC3339),
			"parties":    parties,
			"offerer":    partyFace(deal.OffererParty, deal.OffererOwner, faces, local, now),
			"acceptor":   acceptor,
		})
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"deals": out})
}

// partyFace turns one deal party into the label the wall may show. A listed
// offer names the party; foreign parties stay "an agent" with their host.
func partyFace(party, ownerID string, faces map[string]database.PublicPartyFace, local string, now time.Time) gin.H {
	if host, _, ok := orp.SplitParty(party); ok && host != "" && host != local {
		return gin.H{"listed": false, "role": "agent", "host": host}
	}
	face, known := faces[ownerID]
	if !known {
		return gin.H{"listed": false, "role": "agent"}
	}
	if terms, err := decodeObject(face.ListedTerms); err == nil && len(terms) > 0 && !orp.OfferExpired(terms, now) {
		if title := orp.TermsTitle(terms); title != "" {
			out := gin.H{"listed": true, "name": title}
			if site, ok := terms["url"].(string); ok && site != "" {
				out["url"] = site
			}
			return out
		}
	}
	switch {
	case face.HasSite:
		return gin.H{"listed": false, "role": "publisher"}
	case face.HasAnyOffer:
		return gin.H{"listed": false, "role": "maker"}
	default:
		return gin.H{"listed": false, "role": "agent"}
	}
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
