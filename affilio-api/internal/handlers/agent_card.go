package handlers

import (
	"net/http"
	"strings"

	"affilio-api/internal/dcp"

	"github.com/gin-gonic/gin"
)

// AgentCard is GET /.well-known/agent-card.json.
// It is a discovery document. It does not implement A2A tasks or messages.
// protocolBinding is the open string from the A2A v1.0.0 Agent Interface schema.
// The values here are MCP and HTTP, not JSONRPC, GRPC, or HTTP+JSON.
// protocolVersion is omitted: that field is defined as the A2A version the URL
// exposes, and these URLs do not speak A2A.
func (h *HostedHandler) AgentCard(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.agentCard())
}

func (h *HostedHandler) agentCard() map[string]any {
	base := strings.TrimRight(strings.TrimSpace(h.BaseURL), "/")
	skills := make([]map[string]any, 0, len(dcp.AllTools()))
	for _, tool := range dcp.AllTools() {
		skills = append(skills, map[string]any{
			"id":          tool.Name,
			"name":        tool.Name,
			"description": tool.Description,
			"tags":        []string{"mcp"},
		})
	}
	return map[string]any{
		"name":        "Allagio",
		"description": "Allagio lists the MCP endpoint and the public HTTP API so a client can discover this host. This card does not accept A2A tasks or messages. ORP works over MCP. Allagio is not a base protocol.",
		"version":     "0.1.0",
		"capabilities": map[string]any{
			"streaming":         false,
			"pushNotifications": false,
			"extendedAgentCard": false,
		},
		"defaultInputModes":  []string{"text/plain", "application/json"},
		"defaultOutputModes": []string{"text/plain", "application/json"},
		"supportedInterfaces": []map[string]any{
			{"url": base + "/mcp", "protocolBinding": "MCP"},
			{"url": base + "/v1/health", "protocolBinding": "HTTP"},
		},
		"documentationUrl": base + "/llms.txt",
		"skills":           skills,
	}
}
