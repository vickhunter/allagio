package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins string) gin.HandlerFunc {
	trimmed := strings.TrimSpace(allowedOrigins)
	allowAll := trimmed == "" || trimmed == "*"
	allowed := map[string]struct{}{}
	if !allowAll {
		for _, origin := range strings.Split(trimmed, ",") {
			o := strings.TrimSpace(origin)
			if o != "" {
				allowed[o] = struct{}{}
			}
		}
	}

	return func(c *gin.Context) {
		// ORP discovery, suggest, and join are credential-free and must be
		// readable from any page, including when the request has no Origin
		// and when ALLOWED_ORIGINS is a restricted list. The envelope at
		// POST /orp is not in this set.
		if orpOpen(c.Request.URL.Path) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Cache-Control", "no-store")
		} else if origin := c.GetHeader("Origin"); origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,X-API-Key,X-Admin-Key,Authorization,X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func orpOpen(path string) bool {
	switch path {
	case "/.well-known/orp.json", "/orp/suggest", "/orp/join":
		return true
	default:
		return false
	}
}
