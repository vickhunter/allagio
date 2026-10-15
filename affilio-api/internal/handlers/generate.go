package handlers

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"regexp"
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
	RecommendationID string   `json:"recommendation_id"`
	InsertionType    string   `json:"insertion_type"`
	ArticleContext   string   `json:"article_context"`
	PreferredTone    string   `json:"preferred_tone"`
	SiteLanguage     string   `json:"site_language"`
	ArticleContent   string   `json:"article_content"`
	ArticleHeadings  []string `json:"article_headings"`
	ExistingLinks    []string `json:"existing_links"`
	ArticleWordCount int      `json:"article_word_count"`
	ArticleCategory  string   `json:"article_category"`
	Regenerate       bool     `json:"regenerate"`
}

type storedReaderPreview struct {
	Language        string  `json:"language"`
	HTML            string  `json:"html"`
	Text            string  `json:"text"`
	Position        string  `json:"position"`
	Transition      string  `json:"transition,omitempty"`
	PlacementReason string  `json:"placement_reason,omitempty"`
	ParagraphIndex  int     `json:"suggested_paragraph_index,omitempty"`
	Confidence      float64 `json:"confidence_score,omitempty"`
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
	language := services.NormalizeSiteLanguage(req.SiteLanguage)
	if !services.ReaderCopySafe(rec.ProgramName) || services.ContainsInternalCopy(rec.ProgramName, rec.Reason) ||
		!services.ReaderCopySafe(link) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no reader-facing copy"})
		return
	}
	if !req.Regenerate {
		if preview, ok := reusablePreview(rec.GeneratedHTML, language, rec.Reason, rec.ProgramName, link); ok {
			writeGenerated(c, preview.HTML, preview.Text, preview.Position, preview.PlacementReason, preview.Transition, preview.ParagraphIndex, preview.Confidence, rec.ProgramName, link)
			return
		}
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
	transitionSentence := ""
	suggestedParagraphIndex := -1
	confidenceScore := 0.0
	if h.Generator != nil {
		if plan, err := h.Generator.GeneratePlan(c.Request.Context(), services.ContentRequest{
			ProgramName:      rec.ProgramName,
			Company:          rec.Company,
			Reason:           rec.Reason,
			InsertionType:    insertionType,
			ProgramType:      rec.ProgramType,
			ArticleTitle:     rec.ArticleTitle,
			ArticleURL:       rec.ArticleURL,
			ArticleContext:   req.ArticleContext,
			PreferredTone:    req.PreferredTone,
			SiteLanguage:     language,
			ArticleContent:   req.ArticleContent,
			ArticleHeadings:  req.ArticleHeadings,
			ExistingLinks:    req.ExistingLinks,
			ArticleWordCount: req.ArticleWordCount,
			ArticleCategory:  req.ArticleCategory,
		}); err == nil && plan != nil {
			blurb = strings.TrimSpace(plan.Blurb)
			if strings.TrimSpace(plan.SuggestedPosition) != "" {
				position = strings.TrimSpace(plan.SuggestedPosition)
			}
			placementReason = strings.TrimSpace(plan.PlacementReason)
			transitionSentence = strings.TrimSpace(plan.TransitionSentence)
			suggestedParagraphIndex = plan.SuggestedParagraphIndex
			confidenceScore = plan.ConfidenceScore
		}
	}
	blurb, ok := services.SanitizeReaderCopy(blurb, rec.Reason)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no reader-facing copy"})
		return
	}
	if cleaned, transitionOK := services.SanitizeReaderCopy(transitionSentence, rec.Reason); transitionOK {
		transitionSentence = cleaned
	} else {
		transitionSentence = ""
	}
	if cleaned, placementOK := services.SanitizeReaderCopy(placementReason, rec.Reason); placementOK && !services.ContainsInternalCopy(cleaned, rec.Reason) {
		placementReason = cleaned
	} else {
		placementReason = ""
	}
	rendered := readerHTML(rec.ProgramName, link, blurb)
	if services.ContainsInternalCopy(rendered, rec.Reason) || services.ContainsInternalCopy(blurb, rec.Reason) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no reader-facing copy"})
		return
	}
	position = normalizePosition(position)
	if position == "" {
		position = inferBestPosition(req.ArticleContext, rec.SuggestedPosition, rec.ProgramName, rec.Reason)
	}
	if placementReason == "" {
		placementReason = defaultPlacementReason(position)
	}
	preview := storedReaderPreview{
		Language:        language,
		HTML:            rendered,
		Text:            blurb,
		Position:        position,
		Transition:      transitionSentence,
		PlacementReason: placementReason,
		ParagraphIndex:  suggestedParagraphIndex,
		Confidence:      confidenceScore,
	}
	if encoded, err := json.Marshal(preview); err == nil {
		_ = h.DB.SaveRecommendationGeneratedHTML(c.Request.Context(), siteID, req.RecommendationID, string(encoded))
	}
	writeGenerated(c, rendered, blurb, position, placementReason, transitionSentence, suggestedParagraphIndex, confidenceScore, rec.ProgramName, link)
}

func readerHTML(programName, link, blurb string) string {
	return "<p><a href=\"" + html.EscapeString(link) + "\" rel=\"nofollow sponsored\">" + html.EscapeString(programName) + "</a> — " + html.EscapeString(blurb) + "</p>"
}

func reusablePreview(raw, language, reason, programName, link string) (storedReaderPreview, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return storedReaderPreview{}, false
	}
	var preview storedReaderPreview
	if err := json.Unmarshal([]byte(raw), &preview); err != nil {
		return storedReaderPreview{}, false
	}
	if !strings.EqualFold(services.NormalizeSiteLanguage(preview.Language), services.NormalizeSiteLanguage(language)) {
		return storedReaderPreview{}, false
	}
	text := strings.TrimSpace(preview.Text)
	if text == "" || strings.TrimSpace(preview.HTML) == "" {
		return storedReaderPreview{}, false
	}
	if _, ok := services.SanitizeReaderCopy(text, reason); !ok {
		return storedReaderPreview{}, false
	}
	if services.ContainsInternalCopy(preview.Transition, reason) {
		return storedReaderPreview{}, false
	}
	if !services.ReaderCopySafe(programName) || services.ContainsInternalCopy(programName, reason) || !services.ReaderCopySafe(link) {
		return storedReaderPreview{}, false
	}
	rebuilt := readerHTML(programName, link, text)
	if rebuilt != preview.HTML {
		return storedReaderPreview{}, false
	}
	if services.ContainsInternalCopy(visibleOverlapText(preview.HTML), reason) {
		return storedReaderPreview{}, false
	}
	preview.HTML = rebuilt
	preview.Text = text
	return preview, true
}

var htmlCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

func visibleOverlapText(rawHTML string) string {
	withoutComments := htmlCommentPattern.ReplaceAllString(rawHTML, "")
	withoutTags := htmlTagPattern.ReplaceAllString(withoutComments, "")
	return html.UnescapeString(withoutTags)
}

func writeGenerated(c *gin.Context, rendered, blurb, position, placementReason, transition string, paragraph int, confidence float64, programName, link string) {
	resp := gin.H{
		"html":               rendered,
		"generated_text":     blurb,
		"suggested_position": position,
		"placement_reason":   placementReason,
		"program_name":       programName,
		"link_url":           link,
	}
	if transition != "" {
		resp["transition_sentence"] = transition
	}
	if paragraph >= 0 {
		resp["suggested_paragraph_index"] = paragraph
	}
	if confidence > 0 {
		resp["confidence_score"] = confidence
	}
	c.JSON(http.StatusOK, resp)
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
