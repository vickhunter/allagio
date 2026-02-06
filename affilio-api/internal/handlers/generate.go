package handlers

import (
	"errors"
	"html"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type GenerateHandler struct {
	DB        *database.DB
	Generator *services.AIContentGenerator
}

type generateRequest struct {
	RecommendationID string `json:"recommendation_id"`
	InsertionType    string `json:"insertion_type"`
	ArticleContext   string `json:"article_context"`
	PreferredTone    string `json:"preferred_tone"`
	SiteLanguage     string `json:"site_language"`
}

type confirmRequest struct {
	RecommendationID string `json:"recommendation_id"`
}

func (h *GenerateHandler) GenerateContent(c *gin.Context) {
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	rec, err := h.DB.GetRecommendationContent(c.Request.Context(), siteID, req.RecommendationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
		return
	}
	if strings.TrimSpace(rec.ProgramName) == "" {
		rec.ProgramName = "Recommended tool"
	}
	link := strings.TrimSpace(rec.Link)
	if link == "" {
		link = "#"
	}
	blurb := ""
	placementReason := ""
	position := strings.TrimSpace(rec.SuggestedPosition)
	if position == "" {
		position = "mid_article"
	}
	insertionType := strings.TrimSpace(rec.InsertionType)
	if insertionType == "" {
		insertionType = strings.TrimSpace(req.InsertionType)
	}
	if h.Generator != nil {
		if plan, err := h.Generator.GeneratePlan(c.Request.Context(), services.ContentRequest{
			ProgramName:    rec.ProgramName,
			Company:        rec.Company,
			Reason:         rec.Reason,
			InsertionType:  insertionType,
			ProgramType:    rec.ProgramType,
			ArticleTitle:   rec.ArticleTitle,
			ArticleURL:     rec.ArticleURL,
			ArticleContext: req.ArticleContext,
			PreferredTone:  req.PreferredTone,
			SiteLanguage:   req.SiteLanguage,
		}); err == nil && plan != nil {
			blurb = strings.TrimSpace(plan.Blurb)
			if strings.TrimSpace(plan.SuggestedPosition) != "" {
				position = strings.TrimSpace(plan.SuggestedPosition)
			}
			placementReason = strings.TrimSpace(plan.PlacementReason)
		}
	}
	if blurb == "" {
		blurb = rec.Reason
	}
	if blurb == "" {
		blurb = "A solid option to consider."
	}
	displayName := html.EscapeString(rec.ProgramName)
	displayBlurb := html.EscapeString(blurb)
	displayLink := html.EscapeString(link)
	html := "<p><a href=\"" + displayLink + "\" rel=\"nofollow sponsored\">" + displayName + "</a> — " + displayBlurb + "</p>"
	position = normalizePosition(position)
	if position == "" {
		position = inferBestPosition(req.ArticleContext, rec.SuggestedPosition, rec.ProgramName, rec.Reason)
	}
	if placementReason == "" {
		placementReason = defaultPlacementReason(position)
	}
	c.JSON(http.StatusOK, gin.H{
		"html":               html,
		"generated_text":     blurb,
		"suggested_position": position,
		"placement_reason":   placementReason,
		"program_name":       rec.ProgramName,
		"link_url":           link,
	})
}

func (h *GenerateHandler) ConfirmInsert(c *gin.Context) {
	var req confirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	if err := h.DB.UpdateRecommendationStatus(c.Request.Context(), siteID, req.RecommendationID, "inserted"); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
			return
		}
		if errors.Is(err, database.ErrInvalidTransition) || errors.Is(err, database.ErrInvalidStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": "recommendation must be approved before insert"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
}

func defaultPlacementReason(position string) string {
	switch normalizePosition(position) {
	case "after_intro":
		return "Placed early where reader attention is highest."
	case "before_conclusion":
		return "Placed near decision phase before final takeaway."
	case "end_of_article":
		return "Placed at the end as a low-disruption fallback."
	default:
		return "Placed mid-article near the most relevant context."
	}
}

func normalizePosition(position string) string {
	switch strings.TrimSpace(strings.ToLower(position)) {
	case "after_intro":
		return "after_intro"
	case "mid_article":
		return "mid_article"
	case "before_conclusion":
		return "before_conclusion"
	case "end_of_article":
		return "end_of_article"
	default:
		return ""
	}
}

func inferBestPosition(articleContext, fallback, programName, reason string) string {
	if p := normalizePosition(fallback); p != "" {
		return p
	}
	paragraphs := splitParagraphs(articleContext)
	if len(paragraphs) < 2 {
		return inferPositionByLength(articleContext)
	}
	focus := buildFocusTokens(programName + " " + reason)
	if len(focus) == 0 {
		return inferPositionByLength(articleContext)
	}
	bestIdx := -1
	bestScore := -1.0
	for i, paragraph := range paragraphs {
		score := paragraphScore(paragraph, focus, i, len(paragraphs))
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return inferPositionByLength(articleContext)
	}
	ratio := float64(bestIdx+1) / float64(len(paragraphs))
	switch {
	case ratio <= 0.25:
		return "after_intro"
	case ratio <= 0.75:
		return "mid_article"
	default:
		return "before_conclusion"
	}
}

func inferPositionByLength(articleContext string) string {
	words := len(strings.Fields(strings.TrimSpace(articleContext)))
	switch {
	case words > 1400:
		return "mid_article"
	case words > 700:
		return "before_conclusion"
	case words > 250:
		return "after_intro"
	default:
		return "end_of_article"
	}
}

func splitParagraphs(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func buildFocusTokens(text string) map[string]struct{} {
	seed := []string{
		"review", "compare", "comparison", "pricing", "price", "best", "tool", "software", "plugin", "guide", "trial",
		"guida", "confronto", "prezzo", "migliore", "strumento", "prova", "scopri", "instagram", "seo", "marketing",
	}
	tokens := tokenize(text)
	out := map[string]struct{}{}
	for _, s := range seed {
		out[s] = struct{}{}
	}
	for _, t := range tokens {
		out[t] = struct{}{}
	}
	return out
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 4 {
			continue
		}
		out = append(out, f)
	}
	return out
}

func paragraphScore(paragraph string, focus map[string]struct{}, idx, total int) float64 {
	tokens := tokenize(paragraph)
	if len(tokens) == 0 {
		return -1
	}
	overlap := 0.0
	for _, t := range tokens {
		if _, ok := focus[t]; ok {
			overlap += 1
		}
	}
	score := overlap * 4.0
	if len(tokens) >= 35 && len(tokens) <= 220 {
		score += 2.0
	}
	text := strings.ToLower(paragraph)
	if strings.Contains(text, "review") || strings.Contains(text, "confronto") || strings.Contains(text, "guida") || strings.Contains(text, "best") || strings.Contains(text, "miglior") {
		score += 2.0
	}
	if strings.Contains(text, "http") {
		score += 0.8
	}
	middle := float64(total-1) / 2.0
	score -= absFloat(float64(idx)-middle) * 0.15
	return score
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
