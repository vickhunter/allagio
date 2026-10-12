package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

func APIKeyAuth(db *database.DB, audit *services.AuditLogger, resources ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(resources) > 0 && c.GetHeader("Authorization") != "" {
			g, ok := agentBearer(c, db, resources[0])
			if !ok {
				return
			}
			if g.SiteID == "" || !agentRouteAllowed(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "publisher scope is not authorized for this action"})
				return
			}
			c.Set("site_id", g.SiteID)
			c.Set("agent_access", true)
			c.Next()
			return
		}
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			if audit != nil {
				audit.Log(database.AuditEvent{
					EventType: "auth_failure_api_key",
					Severity:  "warn",
					IPAddress: clientIP(c),
					Endpoint:  c.FullPath(),
					Method:    c.Request.Method,
					Details:   map[string]interface{}{"reason": "missing api key"},
				})
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}
		keyHash := services.HashAPIKey(apiKey)
		keyPrefix := services.KeyPrefix(apiKey)
		site, err := db.GetSiteByAnyKeyHash(c.Request.Context(), keyHash, keyPrefix)
		if err != nil {
			agentSiteID, agentErr := db.AgentSite(c.Request.Context(), keyHash)
			if agentErr == nil {
				if !agentRouteAllowed(c) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "agent key is not authorized for this action"})
					return
				}
				c.Set("site_id", agentSiteID)
				c.Set("agent_access", true)
				c.Next()
				return
			}
			projectSiteID, projectErr := db.ProjectSiteKey(c.Request.Context(), keyHash)
			if projectErr == nil {
				if !agentRouteAllowed(c) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "project key is not authorized for this action"})
					return
				}
				c.Set("api_key_hash", keyHash)
				c.Set("site_id", projectSiteID)
				c.Set("key_kind", "project")
				c.Next()
				return
			}
			account, accountErr := db.ActiveAccountKey(c.Request.Context(), keyHash)
			if accountErr == nil {
				if account.SiteID == "" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this account has no publisher site"})
					return
				}
				c.Set("api_key_hash", keyHash)
				c.Set("site_id", account.SiteID)
				c.Set("owner_id", account.OwnerID)
				c.Set("key_kind", "account")
				c.Next()
				return
			}
		}
		if err != nil {
			if audit != nil {
				audit.Log(database.AuditEvent{
					EventType: "auth_failure_api_key",
					Severity:  "warn",
					IPAddress: clientIP(c),
					Endpoint:  c.FullPath(),
					Method:    c.Request.Method,
					Details:   map[string]interface{}{"reason": "invalid api key", "key_prefix": safePrefix(apiKey)},
				})
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Set("api_key_hash", keyHash)
		c.Set("site_id", site.ID)
		c.Next()
	}
}

func AdminKeyAuth(adminKey string, audit *services.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin key not configured"})
			return
		}
		provided := c.GetHeader("X-Admin-Key")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(adminKey)) != 1 {
			if audit != nil {
				audit.Log(database.AuditEvent{
					EventType: "auth_failure_admin_key",
					Severity:  "warn",
					IPAddress: clientIP(c),
					Endpoint:  c.FullPath(),
					Method:    c.Request.Method,
					Details:   map[string]interface{}{"reason": "invalid admin key"},
				})
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin key"})
			return
		}
		if audit != nil {
			audit.Log(database.AuditEvent{
				EventType: "admin_action",
				Severity:  "info",
				IPAddress: clientIP(c),
				Endpoint:  c.FullPath(),
				Method:    c.Request.Method,
			})
		}
		c.Next()
	}
}

func clientIP(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "unknown"
	}
	return ip
}

func safePrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "..."
}

func agentRouteAllowed(c *gin.Context) bool {
	switch c.FullPath() {
	case "/v1/revenue/opportunities", "/v1/placements", "/v1/placements/:id", "/v1/placements/:id/commit", "/v1/placements/:id/receipt", "/v1/placements/:id/removed":
		return c.Request.Method == http.MethodGet || c.Request.Method == http.MethodPost
	case "/v1/placements/:id/revoke", "/v1/account/pause", "/v1/sites/:id/kill-switch", "/v1/digest/undo":
		return c.Request.Method == http.MethodPost
	case "/v1/digest":
		return c.Request.Method == http.MethodGet
	case "/v1/sites/:id/mode", "/v1/offers/:id/mode":
		return c.Request.Method == http.MethodPut
	case "/v1/payouts/balance", "/v1/payouts/history":
		return c.Request.Method == http.MethodGet
	}
	return false
}
