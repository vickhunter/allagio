package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type MarketplaceHandler struct {
	DB *database.DB
}

func siteIDFrom(c *gin.Context) string {
	return c.GetString("site_id")
}

// ListOffers returns listed offers ranked by fit with this site's topical
// profile (built from its scanned articles); sites without scanned content
// fall back to recency order with relevance 0.
func (h *MarketplaceHandler) ListOffers(c *gin.Context) {
	siteID := siteIDFrom(c)
	offers, err := h.DB.ListMarketplaceOffers(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offers"})
		return
	}
	profileData, err := h.DB.GetSiteProfileData(c.Request.Context(), siteID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load site profile"})
		return
	}
	profile := services.BuildSiteProfile(profileData.Titles, profileData.Categories)
	terms := make([]services.OfferTerms, len(offers))
	for i, offer := range offers {
		terms[i] = services.OfferTerms{
			Name:     offer.ProductName,
			Tagline:  offer.ProductTagline,
			Category: offer.PrimaryCategory,
			Keywords: offer.Keywords,
			Tags:     offer.Tags,
		}
	}
	order, scores := services.RankOffersForProfile(profile, terms)

	out := make([]gin.H, 0, len(offers))
	for _, idx := range order {
		offer := offers[idx]
		item := gin.H{
			"relevance": scores[idx],
			"offer_id":             offer.OfferID,
			"product_name":         offer.ProductName,
			"product_tagline":      offer.ProductTagline,
			"website_url":          offer.WebsiteURL,
			"primary_category":     offer.PrimaryCategory,
			"commission_type":      offer.CommissionType,
			"commission_value":     offer.CommissionValue,
			"commission_currency":  offer.CommissionCurrency,
			"cookie_duration_days": offer.CookieDurationDays,
			"refund_window_days":   offer.RefundWindowDays,
			"terms":                offer.Terms,
			"program_id":           offer.ProgramID,
			"joined":               offer.TrackingCode != nil && offer.CampaignStatus != nil && *offer.CampaignStatus == "active",
		}
		if offer.TrackingCode != nil {
			item["tracking_code"] = *offer.TrackingCode
			item["tracked_link"] = "/v1/go/" + *offer.TrackingCode
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"offers": out})
}

func (h *MarketplaceHandler) JoinOffer(c *gin.Context) {
	offer, err := h.DB.GetOfferByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	if offer.Status != "listed" {
		c.JSON(http.StatusConflict, gin.H{"error": "offer is not listed", "status": offer.Status})
		return
	}
	code, err := generateTrackingCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tracking code"})
		return
	}
	campaignID, code, err := h.DB.JoinOffer(c.Request.Context(), offer.ID, siteIDFrom(c), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join offer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"campaign_id":   campaignID,
		"offer_id":      offer.ID,
		"tracking_code": code,
		"tracked_link":  "/v1/go/" + code,
	})
}

func generateTrackingCode() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
