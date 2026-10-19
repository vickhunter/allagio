package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type OfferReviewHandler struct {
	DB *database.DB
	// ClaimsCheck is the resolved CLAIMS_CHECK mode. Publish runs the same
	// claims check as submit: the page checker when real, otherwise the
	// numeric stub. A "90%" claim that the offer facts do not contain stays
	// submitted.
	ClaimsCheck string
}

type offerReviewPayload struct {
	Note  string `json:"note"`
	Actor string `json:"actor"`
}

var slugCleanPattern = regexp.MustCompile(`[^a-z0-9]+`)

func (h *OfferReviewHandler) Queue(c *gin.Context) {
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	items, err := h.DB.ListSubmittedOffers(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load queue"})
		return
	}
	queue := make([]gin.H, 0, len(items))
	for _, item := range items {
		queue = append(queue, gin.H{
			"id":                   item.ID,
			"product_id":           item.Offer.ProductID,
			"product_name":         item.ProductName,
			"product_tagline":      item.ProductTagline,
			"website_url":          item.WebsiteURL,
			"primary_category":     item.PrimaryCategory,
			"readiness_score":      item.ReadinessScore,
			"readiness_status":     item.ReadinessStatus,
			"maker_email":          item.MakerEmail,
			"maker_name":           item.MakerName,
			"commission_type":      item.CommissionType,
			"commission_value":     item.CommissionValue,
			"commission_currency":  item.CommissionCurrency,
			"cookie_duration_days": item.CookieDurationDays,
			"refund_window_days":   item.RefundWindowDays,
			"landing_url":          item.LandingURL,
			"terms":                item.Terms,
			"submitted_at":         item.Offer.UpdatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

func (h *OfferReviewHandler) Approve(c *gin.Context) {
	offer, item, err := h.loadSubmittedOffer(c)
	if err != nil {
		return
	}
	reason, message, blocked, err := h.claimsOnPublish(c, offer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote offer"})
		return
	}
	if blocked {
		body := gin.H{
			"error":         "offer claims are not ready to publish",
			"status":        "submitted",
			"review_reason": reason,
			"message":       message,
		}
		attachClaimsCheck(body, h.ClaimsCheck, "submitted", reason)
		c.JSON(http.StatusConflict, body)
		return
	}
	slug := offerSlug(item.ProductName, offer.ID)
	programID, err := h.DB.PromoteOffer(c.Request.Context(), offer.ID, slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote offer"})
		return
	}
	body := gin.H{"status": "listed", "offer_id": offer.ID, "program_id": programID, "slug": slug}
	attachClaimsCheck(body, h.ClaimsCheck, "listed", "")
	c.JSON(http.StatusOK, body)
}

func (h *OfferReviewHandler) Reject(c *gin.Context) {
	offer, _, err := h.loadSubmittedOffer(c)
	if err != nil {
		return
	}
	var payload offerReviewPayload
	_ = c.ShouldBindJSON(&payload)
	if err := h.DB.UpdateOfferStatus(c.Request.Context(), offer.ID, "draft"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject offer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "draft", "offer_id": offer.ID, "note": payload.Note})
}

// claimsOnPublish is the second claims check. Submit already held an untrusted
// offer. Publish runs again so a "90%" claim cannot be listed.
func (h *OfferReviewHandler) claimsOnPublish(c *gin.Context, offer *database.Offer) (string, string, bool, error) {
	var product database.Product
	err := h.DB.Pool.QueryRow(c.Request.Context(), `SELECT maker_id::text, name, COALESCE(tagline,''), COALESCE(website_url,''), COALESCE(primary_category,''), COALESCE(keywords,'{}'), COALESCE(tags,'{}')
FROM products WHERE id=$1`, offer.ProductID).Scan(&product.MakerID, &product.Name, &product.Tagline, &product.WebsiteURL, &product.PrimaryCategory, &product.Keywords, &product.Tags)
	if err != nil {
		return "", "", false, err
	}
	product.ID = offer.ProductID
	if claimsReal(h.ClaimsCheck) {
		reason, message, err := compareOfferClaims(c.Request.Context(), h.DB, product.MakerID, product.ID, *offer)
		if err != nil {
			return "", "", false, err
		}
		if reason == entry.PageReasonUnreachable {
			return reviewReasonHead, reviewHeadMessage, true, nil
		}
		if reason != "" {
			return reviewReasonClaims, message, true, nil
		}
		return "", "", false, nil
	}
	if (entry.StubClaimChecker{}).CheckClaims(offer.Terms, offerFacts(*offer, product)) != nil {
		message := reviewClaimsMessage
		if claimsHeld(h.ClaimsCheck) {
			message = reviewClaimsNotLive
		}
		return reviewReasonClaims, message, true, nil
	}
	return "", "", false, nil
}

func (h *OfferReviewHandler) loadSubmittedOffer(c *gin.Context) (*database.Offer, *database.OfferReviewItem, error) {
	offer, err := h.DB.GetOfferByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return nil, nil, err
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return nil, nil, err
	}
	if offer.Status != "submitted" {
		err := errors.New("offer not submitted")
		c.JSON(http.StatusConflict, gin.H{"error": "offer is not awaiting review", "status": offer.Status})
		return nil, nil, err
	}
	item := &database.OfferReviewItem{Offer: *offer}
	row := h.DB.Pool.QueryRow(c.Request.Context(), `
SELECT p.name FROM products p JOIN offers o ON o.product_id = p.id WHERE o.id=$1`, offer.ID)
	if err := row.Scan(&item.ProductName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return nil, nil, err
	}
	return offer, item, nil
}

type makerTrustPayload struct {
	Trusted *bool `json:"trusted"`
}

// SetMakerTrust toggles the trusted fast path: submissions from trusted makers
// list without waiting for the review queue.
func (h *OfferReviewHandler) SetMakerTrust(c *gin.Context) {
	var payload makerTrustPayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Trusted == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trusted (boolean) is required"})
		return
	}
	updated, err := h.DB.SetMakerTrusted(c.Request.Context(), c.Param("id"), *payload.Trusted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update maker"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "maker not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"maker_id": c.Param("id"), "trusted": *payload.Trusted})
}

func (h *OfferReviewHandler) FlaggedConversions(c *gin.Context) {
	flagged, err := h.DB.ListFlaggedConversions(c.Request.Context(), intQuery(c, "limit", 50))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load flagged conversions"})
		return
	}
	out := make([]gin.H, 0, len(flagged))
	for _, f := range flagged {
		out = append(out, gin.H{
			"conversion_id": f.ID,
			"campaign_id":   f.CampaignID,
			"site_domain":   f.SiteDomain,
			"product_name":  f.ProductName,
			"provider":      f.Provider,
			"kind":          f.Kind,
			"amount":        f.Amount,
			"currency":      f.Currency,
			"commission":    f.Commission,
			"flag_reason":   f.FlagReason,
			"occurred_at":   f.OccurredAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"flagged": out})
}

// ClearConversionFlag releases a flagged conversion so its commission resumes
// the normal hold-and-clear lifecycle.
func (h *OfferReviewHandler) ClearConversionFlag(c *gin.Context) {
	cleared, err := h.DB.ClearConversionFlag(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear flag"})
		return
	}
	if !cleared {
		c.JSON(http.StatusNotFound, gin.H{"error": "no flagged conversion with that id"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversion_id": c.Param("id"), "flagged": false})
}

func offerSlug(productName, offerID string) string {
	base := slugCleanPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(productName)), "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "offer"
	}
	if len(base) > 60 {
		base = base[:60]
	}
	suffix := strings.ReplaceAll(offerID, "-", "")
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return base + "-" + suffix
}
