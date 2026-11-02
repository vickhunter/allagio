package handlers

import (
	"strings"

	"affilio-api/internal/dcp"

	"github.com/gin-gonic/gin"
)

// ServeDCPDiscovery is GET /.well-known/dcp.json. DCP v0.1 stays the live
// affiliate path. publicBase selects the hosted MCP transport when it is set.
func ServeDCPDiscovery(publicBase string) gin.HandlerFunc {
	return func(c *gin.Context) {
		mcpURL, mcpTransport := "/v1/mcp", "legacy-http-jsonrpc"
		if strings.TrimSpace(publicBase) != "" {
			mcpURL, mcpTransport = "/mcp", "streamable-http"
		}
		makerVerbs := make([]string, 0, len(dcp.Tools))
		for _, tool := range dcp.Tools {
			makerVerbs = append(makerVerbs, tool.Name)
		}
		publisherVerbs := make([]string, 0, len(dcp.PublisherTools))
		for _, tool := range append(dcp.PublisherTools, dcp.RevenueTools...) {
			publisherVerbs = append(publisherVerbs, tool.Name)
		}
		c.JSON(200, gin.H{
			"dcp_version": "0.1",
			"name":        dcp.ServerName,
			"description": "Allagio. Makers publish offers, publishers earn commissions, agents do the operational work.",
			"http_base":   "/v1",
			"mcp": gin.H{
				"transport": mcpTransport,
				"url":       mcpURL,
				"stdio": gin.H{
					"command": dcp.ServerName,
					"env":     []string{dcp.EnvAPIURL, dcp.EnvAPIKey, dcp.EnvSiteKey},
				},
			},
			"verbs": gin.H{
				"public":    []string{"preview_revenue"},
				"maker":     makerVerbs,
				"publisher": publisherVerbs,
			},
			"payout_methods": []string{"manual"},
		})
	}
}
