package middleware

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func APIKeyAuth(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}
		site, err := db.GetSiteByAnyAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Set("api_key", apiKey)
		c.Set("site_id", site.ID)
		c.Next()
	}
}

func AdminKeyAuth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin key not configured"})
			return
		}
		if c.GetHeader("X-Admin-Key") != adminKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin key"})
			return
		}
		c.Next()
	}
}
