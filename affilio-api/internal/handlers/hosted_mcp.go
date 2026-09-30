package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/dcp"
	"affilio-api/internal/services"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func hostedTools(g *database.AgentGrant) []dcp.Tool {
	tools := append([]dcp.Tool{}, dcp.PreviewTools...)
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
	return tools
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
		s := mcp.NewServer(&mcp.Implementation{Name: "distributoor", Version: "0.3.0"}, &mcp.ServerOptions{Instructions: "Use existing products and publishing tools. Prepare a placement, send its owner approval URL to the human, and wait for approval before committing. Never ask for payment secrets in chat. Agents cannot approve placements, connect Stripe or confirm receipt of money."})
		bearer := strings.Fields(r.Header.Get("Authorization"))[1]
		for _, tool := range hostedTools(g) {
			readOnly := strings.HasPrefix(tool.Name, "get_") || strings.HasPrefix(tool.Name, "list_") || tool.Name == "find_opportunities" || tool.Name == "preview_revenue"
			destructive := strings.HasPrefix(tool.Name, "update_")
			openWorld := tool.Name == "run_assessment"
			s.AddTool(&mcp.Tool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, IdempotentHint: readOnly || tool.Name == "commit_placement", DestructiveHint: &destructive, OpenWorldHint: &openWorld}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := map[string]any{}
				if len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						return nil, err
					}
				}
				client := dcp.NewClient(h.LoopbackURL, "")
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
							out["owner_approval_url"] = h.BaseURL + "/owner/placements/" + id
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
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	verify := func(ctx context.Context, token string, r *http.Request) (*auth.TokenInfo, error) {
		g, err := h.DB.AgentToken(ctx, services.HashAPIKey(token), h.resource())
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{Scopes: strings.Fields(g.Scope), Expiration: g.ExpiresAt, Extra: map[string]any{"grant": g}}, nil
	}
	secured := auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{ResourceMetadataURL: h.BaseURL + "/.well-known/oauth-protected-resource/mcp"})(transport)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != h.BaseURL {
			http.Error(w, "origin not allowed", 403)
			return
		}
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if h.Limiter != nil && !h.Limiter.Allow(ip) {
			http.Error(w, "rate limit exceeded", 429)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		secured.ServeHTTP(w, r)
	})
}
