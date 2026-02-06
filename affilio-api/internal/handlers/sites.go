package handlers

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type SitesHandler struct {
	DB *database.DB
}

type siteRegisterRequest struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

func (h *SitesHandler) Register(c *gin.Context) {
	var req siteRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	_, err = h.DB.CreateSite(c.Request.Context(), apiKey, req.Domain, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register site"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_key": apiKey})
}

func (h *SitesHandler) Delete(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing api key"})
		return
	}
	_, err := h.DB.Pool.Exec(c.Request.Context(), `DELETE FROM sites WHERE api_key=$1`, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
