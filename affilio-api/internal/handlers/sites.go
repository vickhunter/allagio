package handlers

import (
	"log"
	"net/http"
	"strings"

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
	req.Domain = strings.TrimSpace(strings.ToLower(req.Domain))
	req.Name = strings.TrimSpace(req.Name)
	if req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("sites.register key generation failed domain=%s err=%v", req.Domain, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	_, err = h.DB.CreateSite(c.Request.Context(), apiKey, req.Domain, req.Name)
	if err != nil {
		log.Printf("sites.register db create failed domain=%s name=%s err=%v", req.Domain, req.Name, err)
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
