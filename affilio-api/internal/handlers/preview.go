package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/preview"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

const previewBodyLimit = 4096

// PreviewHandler serves POST /api/preview. It reads listed offers and does not write.
type PreviewHandler struct {
	DB      *database.DB
	Limiter *IPRateLimiter
	Fetch   preview.Fetcher
}

type previewOfferCard struct {
	OfferID            string  `json:"offer_id"`
	Name               string  `json:"name"`
	Summary            string  `json:"summary"`
	Category           string  `json:"category"`
	WebsiteURL         string  `json:"website_url"`
	Relevance          int     `json:"relevance"`
	Reason             string  `json:"reason"`
	CommissionType     string  `json:"commission_type"`
	CommissionValue    float64 `json:"commission_value"`
	CommissionCurrency string  `json:"commission_currency"`
}

type previewResponse struct {
	Title string            `json:"title"`
	Offer *previewOfferCard `json:"offer"`
	Slot  *string           `json:"slot"`
}

func (h *PreviewHandler) Preview(c *gin.Context) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}
	if h.Limiter != nil && !h.Limiter.Allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	rawURL, ok := previewURL(c)
	if !ok {
		return
	}
	page, err := h.Fetch.Fetch(c.Request.Context(), rawURL)
	if err != nil {
		writePreviewFetchError(c, err)
		return
	}
	card, slot, err := h.matchPage(c, page.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "preview failed"})
		return
	}
	out := previewResponse{Title: page.Title, Offer: card}
	if card != nil {
		out.Slot = &slot
	}
	c.JSON(http.StatusOK, out)
}

func previewURL(c *gin.Context) (string, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, previewBodyLimit)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req struct {
		URL string `json:"url"`
	}
	if err = dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	if strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return "", false
	}
	return strings.TrimSpace(req.URL), true
}

func writePreviewFetchError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, preview.ErrInvalidURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
	case errors.Is(err, preview.ErrBlocked):
		c.JSON(http.StatusForbidden, gin.H{"error": "blocked address"})
	default:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "fetch failed"})
	}
}

func (h *PreviewHandler) matchPage(c *gin.Context, text string) (*previewOfferCard, string, error) {
	if h.DB == nil {
		return nil, "", errors.New("no database")
	}
	offers, err := h.DB.ListMarketplaceOffers(c.Request.Context(), "00000000-0000-0000-0000-000000000000")
	if err != nil {
		return nil, "", err
	}
	eligible, err := h.DB.EligibleRevenueOfferIDs(c.Request.Context(), false)
	if err != nil {
		return nil, "", err
	}
	terms := make([]services.OfferTerms, 0, len(offers))
	cards := make([]previewOfferCard, 0, len(offers))
	for _, offer := range offers {
		if !eligible[offer.OfferID] {
			continue
		}
		terms = append(terms, services.OfferTerms{
			Name:     offer.ProductName,
			Tagline:  offer.ProductTagline,
			Category: offer.PrimaryCategory,
			Keywords: offer.Keywords,
			Tags:     offer.Tags,
		})
		cards = append(cards, previewOfferCard{
			OfferID:            offer.OfferID,
			Name:               offer.ProductName,
			Summary:            offer.ProductTagline,
			Category:           offer.PrimaryCategory,
			WebsiteURL:         offer.WebsiteURL,
			CommissionType:     offer.CommissionType,
			CommissionValue:    offer.CommissionValue,
			CommissionCurrency: offer.CommissionCurrency,
		})
	}
	hit, ok := services.DryRunMatch(text, terms)
	if !ok {
		return nil, "", nil
	}
	card := cards[hit.Index]
	card.Relevance = hit.Relevance
	card.Reason = hit.Reason
	return &card, hit.Slot, nil
}
