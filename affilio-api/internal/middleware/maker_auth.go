package middleware

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func MakerKeyAuth(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-Maker-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing maker key"})
			return
		}
		maker, err := db.GetMakerByAnyAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid maker key"})
			return
		}
		c.Set("maker_id", maker.ID)
		c.Next()
	}
}
