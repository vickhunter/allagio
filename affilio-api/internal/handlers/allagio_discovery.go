package handlers

import (
	"strings"

	"affilio-api/internal/dcp"

	"github.com/gin-gonic/gin"
)

// ServeAllagioDiscovery is GET /.well-known/allagio.json: the one document a
// cold agent reads first. It names the protocols this host speaks (ORP for
// business semantics, DCP for the affiliate path, MCP for transport), the
// endpoints, the auth methods, and the capability lists. The per-protocol
// discovery documents stay authoritative for their own details.
func ServeAllagioDiscovery(publicBase string) gin.HandlerFunc {
	return func(c *gin.Context) {
		base := discoveryBase(c, publicBase)
		orpTools := make([]string, 0, len(dcp.ORPTools))
		for _, tool := range dcp.ORPTools {
			orpTools = append(orpTools, tool.Name)
		}
		auth := gin.H{
			"bearer": gin.H{
				"join":         base + "/orp/join",
				"agent_tokens": base + "/v1/agent-tokens",
				"note":         "POST an empty object to either endpoint; send the returned token as Authorization: Bearer.",
			},
		}
		mcp := gin.H{"url": base + "/mcp", "transport": "streamable-http", "legacy": gin.H{"url": base + "/v1/mcp", "transport": "http-jsonrpc"}}
		if strings.TrimSpace(publicBase) != "" {
			auth["oauth2"] = gin.H{
				"authorization_server": base + "/.well-known/oauth-authorization-server",
				"registration":         base + "/oauth/register",
				"resource":             base + "/.well-known/oauth-protected-resource/mcp",
				"note":                 "MCP clients that support OAuth + dynamic client registration sign in at /mcp with no manual key.",
			}
		}
		c.JSON(200, gin.H{
			"name":        "Allagio",
			"description": "ORP — the Open Rails Protocol — is how agents do business. Allagio hosts it: discovery, negotiation, execution, receipts and settlement records for partnerships between agents.",
			"protocols": gin.H{
				"orp": gin.H{
					"name":      "Open Rails Protocol",
					"versions":  []string{"0.3", "0.4"},
					"discovery": base + "/.well-known/orp.json",
					"endpoint":  base + "/orp",
					"spec":      base + "/orp/spec",
				},
				"dcp": gin.H{
					"name":      "Distribution Context Protocol",
					"version":   "0.1",
					"discovery": base + "/.well-known/dcp.json",
					"http_base": base + "/v1",
				},
				"mcp": gin.H{
					"name":      "Model Context Protocol",
					"transport": "streamable-http",
					"url":       base + "/mcp",
				},
				"a2a": gin.H{
					"name":       "Agent2Agent",
					"agent_card": base + "/.well-known/agent-card.json",
					"note":       "Discovery card only; A2A task transport is not implemented.",
				},
			},
			"endpoints": gin.H{
				"mcp":           mcp,
				"orp":           base + "/orp",
				"orp_join":      base + "/orp/join",
				"orp_suggest":   base + "/orp/suggest",
				"orp_inbox":     base + "/orp/inbox",
				"http_base":     base + "/v1",
				"agent_signup":  base + "/v1/agent-tokens",
				"install_guide": base + "/agents",
			},
			"auth": auth,
			"capabilities": gin.H{
				"orp_messages":   []string{"hello", "discover", "suggest", "space", "request", "offer", "counter", "accept", "decline", "deal", "leave", "execute", "receipt", "revoke", "dispute", "reputation", "activate"},
				"mcp_deal_tools": orpTools,
				"mcp_resources":  []string{"orp://inbox", "orp://deals/{deal_id}"},
				"roles":          []string{"offerer", "acceptor", "curator", "facilitator", "service", "signer", "member", "consumer"},
			},
			"docs": gin.H{
				"install": base + "/agents",
				"llms":    base + "/llms.txt",
				"spec":    base + "/orp/spec",
			},
		})
	}
}

// ServeAgentCard is GET /.well-known/agent-card.json (and the /agent.json
// alias): the A2A discovery card. It declares what this host can do so an
// A2A-native agent can find it. It does not implement A2A task transport —
// skills point at the MCP and ORP surfaces that actually execute.
func ServeAgentCard(publicBase string) gin.HandlerFunc {
	return func(c *gin.Context) {
		base := discoveryBase(c, publicBase)
		c.JSON(200, gin.H{
			"name":             "Allagio",
			"description":      "Agent-to-agent partnerships on the Open Rails Protocol: find counterparties, negotiate offers, execute, and record receipts and settlement obligations.",
			"url":              base,
			"version":          "0.1.0",
			"provider":         gin.H{"organization": "Allagio", "url": base},
			"documentationUrl": base + "/agents",
			"capabilities":     gin.H{"streaming": false, "pushNotifications": false},
			"skills": []gin.H{
				{
					"id":          "partner-discovery",
					"name":        "Partner discovery",
					"description": "Find counterparties whose offers or surfaces match a plain-text intent via POST /orp/suggest or the find_partners MCP tool.",
					"tags":        []string{"discovery", "offers", "partnerships"},
				},
				{
					"id":          "deal-negotiation",
					"name":        "Deal negotiation",
					"description": "Propose and accept ORP deals via POST /orp or the propose_deal/respond_to_offer/check_deals/approve_deal MCP tools.",
					"tags":        []string{"negotiation", "deals", "orp"},
				},
				{
					"id":          "deal-execution",
					"name":        "Execution and receipts",
					"description": "Execute obligations and read host-issued receipts and proof pages; settlement obligations are recorded, money moves on external rails.",
					"tags":        []string{"execution", "receipts", "settlement"},
				},
			},
			"securitySchemes": gin.H{
				"bearer": gin.H{"type": "http", "scheme": "bearer", "description": "Agent token from POST /orp/join or /v1/agent-tokens"},
			},
			"security": []gin.H{{"bearer": []string{}}},
		})
	}
}

func discoveryBase(c *gin.Context, publicBase string) string {
	if base := strings.TrimRight(strings.TrimSpace(publicBase), "/"); base != "" {
		return base
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
