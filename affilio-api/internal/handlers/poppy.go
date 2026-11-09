package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Poppy is GET /.well-known/poppy.json.
// It describes this host for Personal Agent Protocol draft 0.4:
// https://personalagentprotocol.org/docs/0.4/spec
// The host already publishes one OAuth authorization server for MCP clients.
// auth.disabled tells a Personal Agent not to start Poppy sign-in (spec 3.2).
// There is no guest-only field. Device sign-in is not a field of this document.
func (h *HostedHandler) Poppy(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.poppyDocument())
}

func (h *HostedHandler) poppyDocument() map[string]any {
	base := strings.TrimRight(strings.TrimSpace(h.BaseURL), "/")
	domain := ""
	if parsed, err := url.Parse(base); err == nil {
		domain = parsed.Hostname()
	}
	return map[string]any{
		"protocol_version": "0.4",
		"organization": map[string]any{
			"name":   "Allagio",
			"domain": domain,
		},
		"auth": map[string]any{
			"disabled": true,
		},
		"apis": []map[string]any{
			{
				"type":        "mcp",
				"url":         base + "/mcp",
				"description": poppyMCPDescription(),
			},
		},
	}
}

// poppyMCPDescription is the two calls already stated in discoveryLine.
func poppyMCPDescription() string {
	rest := strings.TrimPrefix(discoveryLine, "Discovery: ")
	if i := strings.Index(rest, " The configured origin"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}
