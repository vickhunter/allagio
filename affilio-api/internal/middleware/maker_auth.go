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
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "maker scope required", "hint": "join bearers (POST /orp/join or /v1/agent-tokens) get maker scope on any /v1/maker write; other callers mint a key with POST /v1/agent-accounts {\"role\":\"maker\"} and send it as X-Maker-Key"})
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
		keyHash := services.HashAPIKey(apiKey)
		maker, err := db.GetMakerByAPIKeyHash(c.Request.Context(), keyHash)
		if err != nil {
			makerID, offerID, keyErr := db.ProjectOfferKey(c.Request.Context(), keyHash)
			if keyErr == nil {
				c.Set("api_key_hash", keyHash)
				c.Set("maker_id", makerID)
				c.Set("offer_scope_id", offerID)
				c.Set("key_kind", "project")
				c.Next()
				return
			}
			account, accountErr := db.ActiveAccountKey(c.Request.Context(), keyHash)
			if accountErr == nil {
				if account.MakerID == "" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this account has no maker"})
					return
				}
				c.Set("api_key_hash", keyHash)
				c.Set("maker_id", account.MakerID)
				c.Set("owner_id", account.OwnerID)
				c.Set("key_kind", "account")
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid maker key"})
			return
		}
		c.Set("api_key_hash", keyHash)
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
