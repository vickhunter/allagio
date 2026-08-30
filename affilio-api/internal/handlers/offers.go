package handlers

import (
	"errors"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type OffersHandler struct {
	DB *database.DB
}

type offerPayload struct {
	ProductID          *string  `json:"product_id"`
	CommissionType     *string  `json:"commission_type"`
	CommissionValue    *float64 `json:"commission_value"`
	CommissionUnit     *string  `json:"commission_unit"`
	CommissionCurrency *string  `json:"commission_currency"`
	CookieDurationDays *int     `json:"cookie_duration_days"`
	RefundWindowDays   *int     `json:"refund_window_days"`
	LandingURL         *string  `json:"landing_url"`
	RefParam           *string  `json:"ref_param"`
	Terms              *string  `json:"terms"`
	Status             *string  `json:"status"`
}

var validCommissionTypes = map[string]bool{
	"percentage": true, "flat": true, "recurring": true,
}

// Maker-driven transitions. Listing (submitted -> listed) happens only through
// the admin promote flow, never through a maker PATCH.
var offerStatusTransitions = map[string]map[string]bool{
	"draft":     {"submitted": true},
	"submitted": {"draft": true},
	"listed":    {"paused": true, "retired": true},
	"paused":    {"listed": true, "retired": true},
}

func (h *OffersHandler) Create(c *gin.Context) {
	var req offerPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.ProductID == nil || strings.TrimSpace(*req.ProductID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product_id is required"})
		return
	}
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), strings.TrimSpace(*req.ProductID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	offer := &database.Offer{
		ProductID:          product.ID,
		CommissionCurrency: "USD",
		CookieDurationDays: 30,
		RefundWindowDays:   30,
		RefParam:           "via",
		LandingURL:         product.WebsiteURL,
	}
	if err := applyOfferPayload(offer, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if offer.CommissionType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commission_type is required"})
		return
	}
	if offer.CommissionValue <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commission_value must be positive"})
		return
	}
	id, err := h.DB.CreateOffer(c.Request.Context(), offer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create offer"})
		return
	}
	created, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	c.JSON(http.StatusOK, offerJSON(created))
}

func (h *OffersHandler) List(c *gin.Context) {
	offers, err := h.DB.ListOffers(c.Request.Context(), makerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offers"})
		return
	}
	out := make([]gin.H, 0, len(offers))
	for i := range offers {
		out = append(out, offerJSON(&offers[i]))
	}
	c.JSON(http.StatusOK, gin.H{"offers": out})
}

func (h *OffersHandler) Get(c *gin.Context) {
	offer, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	c.JSON(http.StatusOK, offerJSON(offer))
}

func (h *OffersHandler) Patch(c *gin.Context) {
	offer, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	var req offerPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Status != nil {
		next := strings.TrimSpace(strings.ToLower(*req.Status))
		if !offerStatusTransitions[offer.Status][next] {
			c.JSON(http.StatusConflict, gin.H{"error": "invalid status transition", "from": offer.Status, "to": next})
			return
		}
		if err := h.DB.UpdateOfferStatus(c.Request.Context(), offer.ID, next); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update offer"})
			return
		}
		offer.Status = next
	}
	// Terms are only editable while the offer isn't live in the catalog.
	if offer.Status == "draft" {
		if err := applyOfferPayload(offer, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if _, err := h.DB.Pool.Exec(c.Request.Context(), `
UPDATE offers
SET commission_type=$2, commission_value=$3, commission_unit=$4, commission_currency=$5,
    cookie_duration_days=$6, refund_window_days=$7, landing_url=$8, ref_param=$9, terms=$10,
    updated_at=NOW()
WHERE id=$1`, offer.ID, offer.CommissionType, offer.CommissionValue, offer.CommissionUnit, offer.CommissionCurrency,
			offer.CookieDurationDays, offer.RefundWindowDays, offer.LandingURL, offer.RefParam, offer.Terms); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update offer"})
			return
		}
	}
	updated, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), offer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	c.JSON(http.StatusOK, offerJSON(updated))
}

func (h *OffersHandler) Submit(c *gin.Context) {
	offer, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	if offer.Status != "draft" {
		c.JSON(http.StatusConflict, gin.H{"error": "only draft offers can be submitted", "status": offer.Status})
		return
	}
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), offer.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	if product.ReadinessStatus != "ready" {
		c.JSON(http.StatusConflict, gin.H{
			"error":            "product has not passed readiness assessment",
			"readiness_status": product.ReadinessStatus,
			"hint":             "run POST /v1/maker/products/:id/assess and fix the failing checks",
		})
		return
	}
	if err := h.DB.UpdateOfferStatus(c.Request.Context(), offer.ID, "submitted"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit offer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": offer.ID, "status": "submitted"})
}

func applyOfferPayload(o *database.Offer, req *offerPayload) error {
	if req.CommissionType != nil {
		t := strings.TrimSpace(strings.ToLower(*req.CommissionType))
		if !validCommissionTypes[t] {
			return errors.New("invalid commission_type")
		}
		o.CommissionType = t
	}
	if req.CommissionValue != nil {
		if *req.CommissionValue <= 0 {
			return errors.New("commission_value must be positive")
		}
		if o.CommissionType == "percentage" && *req.CommissionValue > 100 {
			return errors.New("percentage commission cannot exceed 100")
		}
		o.CommissionValue = *req.CommissionValue
	}
	if req.CommissionUnit != nil {
		o.CommissionUnit = strings.TrimSpace(*req.CommissionUnit)
	}
	if req.CommissionCurrency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*req.CommissionCurrency))
		if len(currency) != 3 {
			return errors.New("commission_currency must be a 3-letter code")
		}
		o.CommissionCurrency = currency
	}
	if req.CookieDurationDays != nil {
		if *req.CookieDurationDays < 1 || *req.CookieDurationDays > 365 {
			return errors.New("cookie_duration_days must be between 1 and 365")
		}
		o.CookieDurationDays = *req.CookieDurationDays
	}
	if req.RefundWindowDays != nil {
		if *req.RefundWindowDays < 0 || *req.RefundWindowDays > 180 {
			return errors.New("refund_window_days must be between 0 and 180")
		}
		o.RefundWindowDays = *req.RefundWindowDays
	}
	if req.LandingURL != nil {
		u, err := normalizeHTTPURL(*req.LandingURL)
		if err != nil {
			return errors.New("landing_url must be a valid http(s) url")
		}
		o.LandingURL = u
	}
	if req.RefParam != nil {
		param := strings.TrimSpace(*req.RefParam)
		if param == "" || len(param) > 50 {
			return errors.New("ref_param must be 1-50 characters")
		}
		o.RefParam = param
	}
	if req.Terms != nil {
		o.Terms = strings.TrimSpace(*req.Terms)
	}
	return nil
}

func offerJSON(o *database.Offer) gin.H {
	return gin.H{
		"id":                   o.ID,
		"product_id":           o.ProductID,
		"status":               o.Status,
		"commission_type":      o.CommissionType,
		"commission_value":     o.CommissionValue,
		"commission_unit":      o.CommissionUnit,
		"commission_currency":  o.CommissionCurrency,
		"cookie_duration_days": o.CookieDurationDays,
		"refund_window_days":   o.RefundWindowDays,
		"landing_url":          o.LandingURL,
		"ref_param":            o.RefParam,
		"terms":                o.Terms,
		"program_id":           o.ProgramID,
		"created_at":           o.CreatedAt,
		"updated_at":           o.UpdatedAt,
	}
}
