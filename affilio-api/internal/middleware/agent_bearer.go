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
	token := fields[1]
	hash := services.HashAPIKey(token)
	g, err := db.AgentToken(c.Request.Context(), hash, resource)
	if err != nil {
		g, err = db.AgentAccountBearer(c.Request.Context(), hash, token, resource)
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
		return nil, false
	}
	return g, true
}
