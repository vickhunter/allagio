package handlers

import (
	"errors"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type RecommendationsHandler struct {
	DB *database.DB
}

type recommendationPatchRequest struct {
	Status string `json:"status"`
}

func (h *RecommendationsHandler) List(c *gin.Context) {
	siteID := c.GetString("site_id")
	recs, err := h.DB.ListRecommendations(c.Request.Context(), siteID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load recommendations"})
		return
	}
	for i := range recs {
		annotateRecommendation(&recs[i])
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *RecommendationsHandler) Get(c *gin.Context) {
	siteID := c.GetString("site_id")
	rec, err := h.DB.GetRecommendation(c.Request.Context(), siteID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	annotateRecommendation(rec)
	c.JSON(http.StatusOK, gin.H{"recommendation": rec})
}

func (h *RecommendationsHandler) Patch(c *gin.Context) {
	var req recommendationPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status != "approved" && req.Status != "dismissed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be approved or dismissed"})
		return
	}
	siteID := c.GetString("site_id")
	if err := h.DB.UpdateRecommendationStatus(c.Request.Context(), siteID, c.Param("id"), req.Status); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if errors.Is(err, database.ErrInvalidStatus) || errors.Is(err, database.ErrInvalidTransition) {
			c.JSON(http.StatusConflict, gin.H{"error": "invalid status transition"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func annotateRecommendation(rec *database.Recommendation) {
	if rec == nil {
		return
	}
	reason := strings.TrimSpace(rec.Reason)
	if reason == "" {
		reason = "Recommended for topical relevance."
	}
	rec.ReasonSummary = clampSentence(reason, 140)
	rec.ExpectedImpact = expectedImpactLabel(rec.RelevanceScore, rec.ProgramType)
	rec.PlacementWhy = placementWhy(rec.SuggestedPosition, rec.RelevanceScore, rec.ProgramType)
}

func clampSentence(input string, maxChars int) string {
	text := strings.TrimSpace(input)
	if len(text) <= maxChars {
		return text
	}
	cut := strings.TrimSpace(text[:maxChars])
	lastSpace := strings.LastIndex(cut, " ")
	if lastSpace > 30 {
		cut = strings.TrimSpace(cut[:lastSpace])
	}
	return cut + "..."
}

func expectedImpactLabel(score int, programType string) string {
	effective := score
	if strings.TrimSpace(strings.ToLower(programType)) == "user" {
		effective += 5
	}
	switch {
	case effective >= 75:
		return "high"
	case effective >= 45:
		return "medium"
	default:
		return "low"
	}
}

func placementWhy(position *string, score int, programType string) string {
	p := ""
	if position != nil {
		p = strings.TrimSpace(strings.ToLower(*position))
	}
	switch p {
	case "after_intro":
		return "Placed early to capture top-of-article attention."
	case "before_conclusion":
		return "Placed near decision points before the final takeaway."
	case "end_of_article":
		return "Placed at the end to reduce disruption."
	default:
		if expectedImpactLabel(score, programType) == "high" {
			return "Placed mid-article near high-intent context for better click likelihood."
		}
		return "Placed mid-article near relevant explanatory context."
	}
}
