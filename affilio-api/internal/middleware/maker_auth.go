package middleware

import (
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
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
			makerID, offerID, keyErr := db.ProjectOfferKey(c.Request.Context(), services.HashAPIKey(apiKey))
			if keyErr != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid maker key"})
				return
			}
			c.Set("maker_id", makerID)
			c.Set("offer_scope_id", offerID)
			c.Next()
			return
		}
		c.Set("maker_id", maker.ID)
		c.Next()
	}
}

// LimitOfferScope keeps an offer-scoped project key inside that one offer.
func LimitOfferScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		scoped := c.GetString("offer_scope_id")
		if scoped == "" {
			c.Next()
			return
		}
		path := c.FullPath()
		if strings.Contains(path, "/offers/:id") && c.Param("id") != scoped {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "key is scoped to a different offer"})
			return
		}
		if strings.Contains(path, "/products") || (strings.HasSuffix(path, "/offers") && c.Request.Method == http.MethodPost) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "key is scoped to one offer"})
			return
		}
		c.Next()
	}
}
