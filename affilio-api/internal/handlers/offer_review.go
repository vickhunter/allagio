package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type OfferReviewHandler struct {
	DB *database.DB
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
	slug := offerSlug(item.ProductName, offer.ID)
	programID, err := h.DB.PromoteOffer(c.Request.Context(), offer.ID, slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote offer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "listed", "offer_id": offer.ID, "program_id": programID, "slug": slug})
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
