package handlers

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type ArticlesHandler struct {
	DB *database.DB
}

func (h *ArticlesHandler) List(c *gin.Context) {
	siteID := c.GetString("site_id")
	articles, err := h.DB.ListArticles(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load articles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"articles": articles})
}

func (h *ArticlesHandler) Get(c *gin.Context) {
	siteID := c.GetString("site_id")
	article, err := h.DB.GetArticle(c.Request.Context(), siteID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	recs, _ := h.DB.ListRecommendations(c.Request.Context(), siteID, &article.ID)
	c.JSON(http.StatusOK, gin.H{
		"article":         article,
		"recommendations": recs,
	})
}
