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
	LowConfidence      bool    `json:"low_confidence,omitempty"`
}

type previewSuggestion struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Line string `json:"line"`
	Note string `json:"note"`
}

type previewResponse struct {
	Title         string             `json:"title"`
	Author        string             `json:"author,omitempty"`
	Excerpt       string             `json:"excerpt,omitempty"`
	Image         string             `json:"image,omitempty"`
	Demo          bool               `json:"demo"`
	Offer         *previewOfferCard  `json:"offer"`
	Offers        []previewOfferCard `json:"offers,omitempty"`
	Slot          *string            `json:"slot"`
	LowConfidence bool               `json:"low_confidence,omitempty"`
	Suggestion    previewSuggestion  `json:"suggestion"`
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
	out, err := h.matchPage(c, page.Title, page.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "preview failed"})
		return
	}
	out.Author = page.Author
	out.Excerpt = previewExcerpt(page.Text)
	out.Image = page.Image
	out.Demo = true
	out.Suggestion = suggestLine(out)
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
	case errors.Is(err, preview.ErrUnreadable):
		c.JSON(http.StatusOK, gin.H{
			"result":  "unreadable_or_private",
			"message": "only public links work",
		})
	default:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "fetch failed"})
	}
}

func (h *PreviewHandler) matchPage(c *gin.Context, title, text string) (previewResponse, error) {
	out := previewResponse{Title: title}
	if h.DB == nil {
		return out, errors.New("no database")
	}
	offers, err := h.DB.ListMarketplaceOffers(c.Request.Context(), "00000000-0000-0000-0000-000000000000")
	if err != nil {
		return out, err
	}
	eligible, err := h.DB.EligibleRevenueOfferIDs(c.Request.Context(), false)
	if err != nil {
		return out, err
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
	ranked := services.RankPageOffers(text, terms)
	if len(ranked) == 0 {
		return out, nil
	}
	if ranked[0].Relevance >= services.RelevanceFloor {
		card := previewCard(cards, ranked[0])
		slot := ranked[0].Slot
		out.Offer = &card
		out.Slot = &slot
		return out, nil
	}
	n := 3
	if len(ranked) < n {
		n = len(ranked)
	}
	closest := make([]previewOfferCard, 0, n)
	for _, hit := range ranked[:n] {
		card := previewCard(cards, hit)
		card.LowConfidence = true
		closest = append(closest, card)
	}
	top := closest[0]
	slot := ranked[0].Slot
	out.Offer = &top
	out.Offers = closest
	out.Slot = &slot
	out.LowConfidence = true
	return out, nil
}

func previewCard(cards []previewOfferCard, hit services.DryRunHit) previewOfferCard {
	card := cards[hit.Index]
	card.Relevance = hit.Relevance
	card.Reason = hit.Reason
	return card
}

const excerptRunes = 320

func previewExcerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > excerptRunes {
		return string(runes[:excerptRunes]) + "…"
	}
	return text
}

// suggestLine always answers with the line Allagio would append to the post.
// A verified offer names itself; a low-confidence or absent match is labelled
// "not on Allagio yet" instead of pretending a program exists.
func suggestLine(out previewResponse) previewSuggestion {
	if out.Offer != nil && !out.LowConfidence {
		return previewSuggestion{
			Kind: "partner_mention",
			Name: out.Offer.Name,
			Line: "Sponsored — " + out.Offer.Name + ": " + out.Offer.Summary,
			Note: "A listed offer, labelled inline.",
		}
	}
	if out.Offer != nil {
		return previewSuggestion{
			Kind: "partner_mention",
			Name: out.Offer.Name,
			Line: "Sponsored — " + out.Offer.Name + ": " + out.Offer.Summary,
			Note: "Closest listed offer, low confidence.",
		}
	}
	return previewSuggestion{
		Kind: "partner_mention",
		Name: "Suggested partner",
		Line: "Sponsored — a partner whose product fits this post would take this line.",
		Note: "No listed program matched; a matching partner could fill this slot.",
	}
}
