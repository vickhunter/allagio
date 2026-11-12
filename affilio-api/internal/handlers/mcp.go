package handlers

import (
	"io"
	"net/http"
	"strings"

	"affilio-api/internal/dcp"
	"github.com/gin-gonic/gin"
)

// MCPHandler is the hosted DCP transport: one streamable-HTTP MCP endpoint
// carrying the same JSON-RPC messages as the stdio server, so agents can speak
// DCP to the network without installing anything. Auth rides per request —
// X-Maker-Key (or Authorization: Bearer) for maker verbs, X-API-Key for
// publisher verbs — and flows into a per-request client that calls the API's
// own HTTP surface, keeping the MCP layer a thin wrapper over one contract.
type MCPHandler struct {
	// LoopbackURL is the base URL the wrapped verbs call — the API itself.
	LoopbackURL string
	RateLimiter *IPRateLimiter
}

func (h *MCPHandler) Receive(c *gin.Context) {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	if h.RateLimiter != nil && !h.RateLimiter.Allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1024*1024))
	if err != nil || len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected a JSON-RPC message body"})
		return
	}
	makerKey := c.GetHeader("X-Maker-Key")
	if makerKey == "" {
		if bearer, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
			makerKey = strings.TrimSpace(bearer)
		}
	}
	client := dcp.NewClient(h.LoopbackURL, makerKey)
	client.SiteKey = c.GetHeader("X-API-Key")
	client.Bearer = makerKey

	resp := dcp.HandleMessage(client, body)
	if resp == nil {
		// Notification or unparseable input: acknowledged, nothing to answer.
		c.Status(http.StatusAccepted)
		return
	}
	c.Data(http.StatusOK, "application/json", resp)
}

// MethodNotAllowed answers non-POST methods on /v1/mcp. The Streamable HTTP
// spec expects 405 here so clients learn the endpoint is POST-only.
func (h *MCPHandler) MethodNotAllowed(c *gin.Context) {
	c.Header("Allow", "POST")
	c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
}
