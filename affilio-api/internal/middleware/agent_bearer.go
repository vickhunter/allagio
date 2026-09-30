package middleware

import (
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

func agentBearer(c *gin.Context, db *database.DB, resource string) (*database.AgentGrant, bool) {
	fields := strings.Fields(c.GetHeader("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return nil, false
	}
	g, err := db.AgentToken(c.Request.Context(), services.HashAPIKey(fields[1]), resource)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
		return nil, false
	}
	return g, true
}
