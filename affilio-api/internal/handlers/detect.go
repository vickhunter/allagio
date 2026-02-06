package handlers

import (
	"net/http"

	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type DetectHandler struct {
	Classifier *services.AIClassifier
}

type detectRequest struct {
	Content string `json:"content"`
}

func (h *DetectHandler) DetectLinks(c *gin.Context) {
	var req detectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	detections := services.DetectAffiliateLinks(req.Content)
	if h.Classifier != nil {
		if classified, err := h.Classifier.ClassifyAffiliateLinks(c.Request.Context(), detections); err == nil {
			detections = classified
		}
	}
	c.JSON(http.StatusOK, gin.H{"detections": detections})
}
