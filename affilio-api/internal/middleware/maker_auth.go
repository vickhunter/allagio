package middleware

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func MakerKeyAuth(db *database.DB, resources ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(resources) > 0 && c.GetHeader("Authorization") != "" {
			g, ok := agentBearer(c, db, resources[0])
			if !ok {
				return
			}
			if g.MakerID == "" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "maker scope required"})
				return
			}
			c.Set("maker_id", g.MakerID)
			c.Set("agent_access", true)
			c.Next()
			return
		}
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
