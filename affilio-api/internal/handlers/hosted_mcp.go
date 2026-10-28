package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/dcp"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func hostedTools(g *database.AgentGrant) []dcp.Tool {
	if g != nil && g.Join {
		shown := *g
		if shown.SiteID == "" {
			shown.SiteID = "join"
		}
		if shown.MakerID == "" {
			shown.MakerID = "join"
		}
		return hostedToolList(&shown)
	}
	return hostedToolList(g)
}

func hostedToolList(g *database.AgentGrant) []dcp.Tool {
	tools := append([]dcp.Tool{}, dcp.PreviewTools...)
	tools = append(tools, dcp.MonetizationTools...)
	if g.MakerID != "" {
		for _, t := range dcp.Tools {
			if t.Name != "register_maker" {
				tools = append(tools, t)
			}
		}
	}
	if g.SiteID != "" {
		tools = append(tools, dcp.RevenueTools...)
		for _, t := range dcp.PublisherTools {
			if t.Name == "get_balance" || t.Name == "get_payout_history" {
				tools = append(tools, t)
			}
		}
	}
	if g.MakerID != "" || g.SiteID != "" {
		for _, t := range dcp.PublisherTools {
			if t.Name == "get_step_modes" || t.Name == "set_step_modes" {
				tools = append(tools, t)
			}
		}
	}
	return tools
}

const hostedMCPInstructions = "Use existing products and publishing tools. Prepare a placement. Ask me every time, send its owner approval URL to the human, and a person approves each placement before committing. In Approve within my rules and Approve anything that passes the checks, the owner's rule decides, and agent tools still cannot change that rule. Never ask for payment secrets in chat. Agent tools cannot connect Stripe or confirm receipt of money. suggest_monetization takes a draft and returns up to three offers with a tracked link and a disclosure line. It does not publish."

func newHostedMCPServer(grant *database.AgentGrant, loopbackURL, baseURL, bearer string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: dcp.ServerName, Title: dcp.ServerTitle, Version: "0.3.0"}, &mcp.ServerOptions{Instructions: hostedMCPInstructions})
	for _, tool := range hostedTools(grant) {
		readOnly := strings.HasPrefix(tool.Name, "get_") || strings.HasPrefix(tool.Name, "list_") || tool.Name == "find_opportunities" || tool.Name == "preview_revenue" || tool.Name == "suggest_monetization"
		destructive := strings.HasPrefix(tool.Name, "update_") || tool.Name == "undo_placement" || tool.Name == "restore_placement"
		openWorld := tool.Name == "run_assessment"
		s.AddTool(&mcp.Tool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, IdempotentHint: readOnly || tool.Name == "commit_placement", DestructiveHint: &destructive, OpenWorldHint: &openWorld}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := map[string]any{}
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return nil, err
				}
			}
			client := dcp.NewClient(loopbackURL, "")
			client.Bearer = bearer
			client.Context = ctx
			client.HTTP = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			out, err := dcp.CallTool(client, tool.Name, args)
			if err != nil {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
			}
			if tool.Name == "prepare_placement" || tool.Name == "get_placement" {
				if p, ok := out["placement"].(map[string]any); ok {
					if id, ok := p["id"].(string); ok {
						out["owner_approval_url"] = baseURL + "/owner/placements/" + id
					}
				}
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
		})
	}
	return s
}

func (h *HostedHandler) MCPTransport() http.Handler {
	transport := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		info := auth.TokenInfoFromContext(r.Context())
		if info == nil {
			return nil
		}
		g, ok := info.Extra["grant"].(*database.AgentGrant)
		if !ok {
			return nil
		}
		bearer := strings.Fields(r.Header.Get("Authorization"))[1]
		return newHostedMCPServer(g, h.LoopbackURL, h.BaseURL, bearer)
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	verify := func(ctx context.Context, token string, r *http.Request) (*auth.TokenInfo, error) {
		hash := services.HashAPIKey(token)
		g, err := h.DB.ResolveBearer(ctx, hash, token, h.resource())
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{Scopes: strings.Fields(g.Scope), Expiration: g.ExpiresAt, Extra: map[string]any{"grant": g}}, nil
	}
	return auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{ResourceMetadataURL: h.BaseURL + "/.well-known/oauth-protected-resource/mcp"})(transport)
}

// MCP is the gin entry point for the hosted stream. Rate limits use ClientIP
// so a configured trusted proxy supplies the visitor address instead of the
// platform connection address.
func (h *HostedHandler) MCP(c *gin.Context) {
	if origin := c.Request.Header.Get("Origin"); origin != "" && origin != h.BaseURL {
		http.Error(c.Writer, "origin not allowed", http.StatusForbidden)
		return
	}
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}
	if h.Limiter != nil && !h.Limiter.Allow(ip) {
		http.Error(c.Writer, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	h.MCPTransport().ServeHTTP(c.Writer, c.Request)
}
