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
	if h.Generator != nil {
		if generated, err := h.Generator.GenerateSnippet(c.Request.Context(), services.ContentRequest{
			ProgramName:   rec.ProgramName,
			Company:       rec.Company,
			Reason:        rec.Reason,
			InsertionType: rec.InsertionType,
			ProgramType:   rec.ProgramType,
			ArticleTitle:  rec.ArticleTitle,
			ArticleURL:    rec.ArticleURL,
		}); err == nil {
			blurb = strings.TrimSpace(generated)
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
	html := "<p><a href=\"" + link + "\" rel=\"nofollow sponsored\">" + displayName + "</a> — " + displayBlurb + "</p>"
	position := strings.TrimSpace(rec.SuggestedPosition)
	if position == "" {
		position = "end_of_article"
	}
	c.JSON(http.StatusOK, gin.H{"html": html, "suggested_position": position})
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
