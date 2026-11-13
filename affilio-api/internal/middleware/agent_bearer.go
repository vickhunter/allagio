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
	g, err := db.ResolveBearer(c.Request.Context(), hash, token, resource)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
		return nil, false
	}
	if g.Join {
		if err = inferJoinSide(c, db, g); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "account update failed"})
			return nil, false
		}
	}
	return g, true
}

// inferJoinSide records a publisher when this call publishes a placement, and
// a maker when this call lists or creates an offer. A join token can do both.
func inferJoinSide(c *gin.Context, db *database.DB, g *database.AgentGrant) error {
	switch joinInferSide(c.Request.Method, c.FullPath()) {
	case "publisher":
		if g.SiteID != "" {
			return nil
		}
		siteID, scope, err := db.EnsureJoinPublisher(c.Request.Context(), g.OwnerID)
		if err != nil {
			return err
		}
		g.SiteID = siteID
		g.Scope = scope
	case "maker":
		if g.MakerID != "" {
			return nil
		}
		makerID, scope, err := db.EnsureJoinMaker(c.Request.Context(), g.OwnerID)
		if err != nil {
			return err
		}
		g.MakerID = makerID
		g.Scope = scope
	}
	return nil
}

func joinInferSide(method, path string) string {
	switch {
	case method == http.MethodPost && (path == "/v1/placements" || path == "/v1/placements/:id/commit" || path == "/v1/placements/:id/receipt"):
		return "publisher"
	case method == http.MethodGet && path == "/v1/maker/getting-started":
		return "maker"
	case path == "/v1/maker/offers" && (method == http.MethodGet || method == http.MethodPost):
		return "maker"
	case method == http.MethodPost && path == "/v1/maker/offers/:id/submit":
		return "maker"
	default:
		return ""
	}
}
