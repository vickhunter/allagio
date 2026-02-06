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
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *RecommendationsHandler) Get(c *gin.Context) {
	siteID := c.GetString("site_id")
	rec, err := h.DB.GetRecommendation(c.Request.Context(), siteID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
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
