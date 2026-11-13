package handlers

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

// GettingStarted is the guided maker recipe behind the get_started MCP tool:
// the ordered steps, which are already done for this key, and next_step with
// the exact tool and arguments to call — so an agent never has to infer the
// flow from documentation.
func (h *ProductsHandler) GettingStarted(c *gin.Context) {
	maker := makerID(c)
	// An offer-scoped project key must stay inside its one offer: answer for
	// that offer only, never the maker's other products.
	if offerID := c.GetString("offer_scope_id"); offerID != "" {
		h.gettingStartedOfferScope(c, offerID)
		return
	}
	products, err := h.DB.ListProducts(c.Request.Context(), maker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load products"})
		return
	}
	offers, err := h.DB.ListOffers(c.Request.Context(), maker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offers"})
		return
	}

	var product *database.Product
	if len(products) > 0 {
		product = &products[0]
	}
	// Offers are maker-wide; the recipe only follows the product it picked.
	var productOffer *database.Offer
	for i := range offers {
		if product != nil && offers[i].ProductID == product.ID {
			productOffer = &offers[i]
			break
		}
	}

	steps := []gin.H{
		{"tool": "register_maker", "done": true, "note": "this key authenticates as a maker"},
		{"tool": "register_product", "done": len(products) > 0},
		{"tool": "run_assessment", "done": false},
		{"tool": "create_offer", "done": productOffer != nil},
		{"tool": "submit_offer", "done": false},
	}

	next := gin.H{}
	switch {
	case product == nil:
		next = gin.H{
			"tool": "register_product",
			"args": gin.H{"name": "your product name", "website_url": "https://your-landing-page.example", "docs_url": "https://docs url", "pricing_url": "https://pricing url"},
			"why":  "no product registered yet — the assessment probes the live landing page",
		}
	case product.ReadinessStatus != "ready":
		steps[2]["note"] = "product readiness_status is " + product.ReadinessStatus
		next = gin.H{
			"tool": "run_assessment",
			"args": gin.H{"product_id": product.ID},
			"why":  "product \"" + product.Name + "\" is not ready — run_assessment returns fix_hints with what to fix, then run it again",
		}
	case productOffer == nil:
		steps[2]["done"] = true
		next = gin.H{
			"tool": "create_offer",
			"args": gin.H{"product_id": product.ID, "commission_type": "percentage", "commission_value": 30},
			"why":  "product is ready — create a draft offer with a commission on the first paid checkout",
		}
	default:
		steps[2]["done"] = true
		latest := productOffer
		switch latest.Status {
		case "draft":
			next = gin.H{
				"tool": "submit_offer",
				"args": gin.H{"offer_id": latest.ID},
				"why":  "the draft offer is ready to submit for review",
			}
		case "submitted":
			steps[4]["done"] = true
			steps[4]["note"] = "offer is in review"
			next = gin.H{
				"tool": "list_offers",
				"args": gin.H{},
				"why":  "the offer is in review — poll list_offers; a placement still needs the publisher's owner approval",
			}
		default:
			steps[4]["done"] = true
			next = gin.H{
				"tool": "list_offers",
				"args": gin.H{},
				"why":  "the offer is " + latest.Status + " — publishers can find it through list_marketplace_offers",
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"recipe":    "register_maker → register_product → run_assessment → create_offer → submit_offer",
		"steps":     steps,
		"next_step": next,
		"tip":       "call get_started again after each step to stay on the guided path",
	})
}

// gettingStartedOfferScope answers the recipe for an offer-scoped project key:
// the only step left is that offer's own state, so it never lists the maker's
// other products or offers.
func (h *ProductsHandler) gettingStartedOfferScope(c *gin.Context, offerID string) {
	offer, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), offerID)
	if err != nil || offer == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "key is scoped to a different offer"})
		return
	}
	var next gin.H
	switch offer.Status {
	case "draft":
		next = gin.H{
			"tool": "submit_offer",
			"args": gin.H{"offer_id": offer.ID},
			"why":  "the draft offer is ready to submit for review",
		}
	case "submitted":
		next = gin.H{
			"tool": "get_offer",
			"args": gin.H{"offer_id": offer.ID},
			"why":  "the offer is in review — poll get_offer; placements still need owner approval",
		}
	default:
		next = gin.H{
			"tool": "get_offer",
			"args": gin.H{"offer_id": offer.ID},
			"why":  "the offer is " + offer.Status,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"recipe":       "register_maker → register_product → run_assessment → create_offer → submit_offer",
		"offer_scoped": true,
		"next_step":    next,
		"tip":          "this key is scoped to one offer — a full maker key unlocks the whole recipe",
	})
}
