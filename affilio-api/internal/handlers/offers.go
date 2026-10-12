package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type OffersHandler struct {
	DB *database.DB
	// ClaimsCheck is stub, real, or off-local. Empty matches off-local so a
	// test router keeps today's four checks.
	ClaimsCheck string
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
	c.JSON(http.StatusOK, h.present(c, created))
}

func (h *OffersHandler) List(c *gin.Context) {
	out, err := h.reviewed(c, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"offers": out})
}

// Listings is GET /v1/listings. Each item is the offer plus its card.
// review_reason and message are null when the offer is listed.
func (h *OffersHandler) Listings(c *gin.Context) {
	out, err := h.reviewed(c, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load listings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"listings": out})
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
	c.JSON(http.StatusOK, h.present(c, offer))
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
	c.JSON(http.StatusOK, h.present(c, updated))
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
		reason, message := h.readinessReason(c.Request.Context(), product)
		c.JSON(http.StatusConflict, gin.H{
			"error":            readinessFailMessage,
			"reason":           reason,
			"message":          message,
			"readiness_status": product.ReadinessStatus,
		})
		return
	}
	// Trusted makers skip the human review queue: their submissions list directly.
	trusted, err := h.DB.IsMakerTrusted(c.Request.Context(), makerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit offer"})
		return
	}
	if trusted {
		h.listOffer(c, offer, product)
		return
	}
	if claimsHeld(h.ClaimsCheck) {
		if err := h.DB.UpdateOfferStatus(c.Request.Context(), offer.ID, "submitted"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit offer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":            offer.ID,
			"status":        "submitted",
			"review_reason": reviewReasonClaims,
			"message":       reviewClaimsNotLive,
		})
		return
	}
	listed, err := h.DB.CountListedOffers(c.Request.Context(), makerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit offer"})
		return
	}
	if reason, message, failed := failedListingCheck(*offer, *product, listed); failed {
		if err := h.DB.UpdateOfferStatus(c.Request.Context(), offer.ID, "submitted"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit offer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": offer.ID, "status": "submitted", "review_reason": reason, "message": message})
		return
	}
	h.listOffer(c, offer, product)
}

func (h *OffersHandler) listOffer(c *gin.Context, offer *database.Offer, product *database.Product) {
	programID, err := h.DB.PromoteOffer(c.Request.Context(), offer.ID, offerSlug(product.Name, offer.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list offer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": offer.ID, "status": "listed", "program_id": programID, "review_reason": nil, "message": nil})
}

// Matches ranks the network's publishers by topical fit with the offer's
// product, so a maker sees which sites are worth inviting.
func (h *OffersHandler) Matches(c *gin.Context) {
	offer, err := h.DB.GetOfferForMaker(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load offer"})
		return
	}
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), offer.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	sites, err := h.DB.ListMatchableSites(c.Request.Context(), offer.ID, 100, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load sites"})
		return
	}
	terms := services.OfferTerms{
		Name:     product.Name,
		Tagline:  product.Tagline,
		Category: product.PrimaryCategory,
		Keywords: product.Keywords,
		Tags:     product.Tags,
	}
	type rankedSite struct {
		site      database.MatchableSite
		relevance int
	}
	ranked := make([]rankedSite, 0, len(sites))
	for _, site := range sites {
		profile := services.BuildSiteProfile(site.Titles, nil)
		ranked = append(ranked, rankedSite{site: site, relevance: services.ScoreOfferForProfile(profile, terms)})
	}
	sort.SliceStable(ranked, func(a, b int) bool { return ranked[a].relevance > ranked[b].relevance })
	limit := intQuery(c, "limit", 20)
	if limit > len(ranked) {
		limit = len(ranked)
	}
	out := make([]gin.H, 0, limit)
	for _, r := range ranked[:limit] {
		out = append(out, gin.H{
			"site_domain": r.site.Domain,
			"site_name":   r.site.Name,
			"relevance":   r.relevance,
			"joined":      r.site.Joined,
		})
	}
	c.JSON(http.StatusOK, gin.H{"offer_id": offer.ID, "matches": out})
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
		u, err := normalizeHTTPSURL(*req.LandingURL)
		if err != nil {
			return errors.New("landing_url must be an https URL")
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

func (h *OffersHandler) present(c *gin.Context, offer *database.Offer) gin.H {
	listed, err := h.DB.CountListedOffers(c.Request.Context(), makerID(c))
	if err != nil {
		listed = 0
	}
	return h.presentWith(c, offer, listed, map[string]*database.Product{})
}

func (h *OffersHandler) presentWith(c *gin.Context, offer *database.Offer, listed int, products map[string]*database.Product) gin.H {
	reason, message := "", ""
	var product *database.Product
	if offer.Status == "submitted" {
		if claimsHeld(h.ClaimsCheck) {
			reason, message = reviewReasonClaims, reviewClaimsNotLive
		}
		product = products[offer.ProductID]
		if product == nil {
			loaded, err := h.DB.GetProduct(c.Request.Context(), makerID(c), offer.ProductID)
			if err == nil {
				product = loaded
				products[offer.ProductID] = loaded
			}
		}
		if product != nil && !claimsHeld(h.ClaimsCheck) {
			reason, message = listingReview(h.ClaimsCheck, *offer, *product, listed)
		}
	}
	item := offerJSON(offer, reason, message)
	if product == nil {
		if cached := products[offer.ProductID]; cached != nil {
			product = cached
		}
	}
	if product != nil {
		item["card"] = gin.H{
			"name":     product.Name,
			"summary":  product.Tagline,
			"category": product.PrimaryCategory,
		}
	}
	return item
}

func (h *OffersHandler) reviewed(c *gin.Context, withCard bool) ([]gin.H, error) {
	offers, err := h.DB.ListOffers(c.Request.Context(), makerID(c))
	if err != nil {
		return nil, err
	}
	listed, err := h.DB.CountListedOffers(c.Request.Context(), makerID(c))
	if err != nil {
		return nil, err
	}
	products := map[string]*database.Product{}
	out := make([]gin.H, 0, len(offers))
	for i := range offers {
		if scoped := c.GetString("offer_scope_id"); scoped != "" && offers[i].ID != scoped {
			continue
		}
		if withCard {
			if _, ok := products[offers[i].ProductID]; !ok {
				if loaded, err := h.DB.GetProduct(c.Request.Context(), makerID(c), offers[i].ProductID); err == nil {
					products[offers[i].ProductID] = loaded
				}
			}
		}
		item := h.presentWith(c, &offers[i], listed, products)
		if !withCard {
			delete(item, "card")
		}
		out = append(out, item)
	}
	return out, nil
}

func offerJSON(o *database.Offer, reason, message string) gin.H {
	reviewReason, reviewMessage := nullReview(reason, message)
	if o.Status == "listed" {
		reviewReason, reviewMessage = nil, nil
	}
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
		"review_reason":        reviewReason,
		"message":              reviewMessage,
	}
}
