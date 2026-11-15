package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
	// Join bearers see the maker verbs even before a maker exists: their first
	// call on a /v1/maker route infers the role, so the guided path must be
	// visible at tools/list.
	if g.MakerID != "" || g.Join {
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
	for _, t := range dcp.ORPTools {
		if t.Name == "approve_deal" {
			continue
		}
		tools = append(tools, t)
	}
	return tools
}

const hostedMCPInstructions = "Call get_started first — it returns the ordered recipe and the exact next_step to call. Use existing products and publishing tools. Prepare a placement. " + ownerApprovalURL + " " + publishStepParagraph + " Agents cannot approve a placement. Never ask for payment secrets in chat. Agent tools cannot connect Stripe or confirm receipt of money. suggest_monetization takes a draft and returns up to three offers with a tracked link and a disclosure line. It does not publish."

func newHostedMCPServer(grant *database.AgentGrant, loopbackURL, baseURL, bearer string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: dcp.ServerName, Title: dcp.ServerTitle, Version: "0.3.0"}, &mcp.ServerOptions{Instructions: hostedMCPInstructions})
	for _, tool := range hostedTools(grant) {
		readOnly := strings.HasPrefix(tool.Name, "get_") || strings.HasPrefix(tool.Name, "list_") || tool.Name == "find_opportunities" || tool.Name == "preview_revenue" || tool.Name == "suggest_monetization"
		destructive := strings.HasPrefix(tool.Name, "update_") || tool.Name == "undo_placement" || tool.Name == "restore_placement"
		openWorld := tool.Name == "run_assessment"
		s.AddTool(&mcp.Tool{Name: tool.Name, Title: tool.Title(), Description: tool.Description, InputSchema: tool.InputSchema, OutputSchema: map[string]any{"type": "object"}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, IdempotentHint: readOnly || tool.Name == "commit_placement", DestructiveHint: &destructive, OpenWorldHint: &openWorld}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: out}, nil
		})
	}
	addSearchFetchTools(s, loopbackURL, bearer)
	return s
}

// addSearchFetchTools registers the OpenAI-compatible search/fetch pair: a
// read-only lookup over the listed offers the caller's token can already see.
// ids are offer ids and url is the product's website_url.
func addSearchFetchTools(s *mcp.Server, loopbackURL, bearer string) {
	loadOffers := func(ctx context.Context) ([]any, error) {
		client := dcp.NewClient(loopbackURL, "")
		client.Bearer = bearer
		client.Context = ctx
		client.HTTP = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		out, err := client.DoSite(http.MethodGet, "/v1/marketplace/offers", nil)
		if err != nil {
			return nil, err
		}
		items, _ := out["offers"].([]any)
		return items, nil
	}
	structured := func(v any) (*mcp.CallToolResult, error) {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: m}, nil
	}
	fail := func(err error) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
	}
	outputSchema := map[string]any{"type": "object"}

	s.AddTool(&mcp.Tool{
		Name:         "search",
		Title:        "Search offers",
		Description:  "Search listed offers by name, tagline, or category. Returns {results:[{id,title,url}]}.",
		InputSchema:  map[string]any{"type": "object", "required": []string{"query"}, "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Search query"}}},
		OutputSchema: outputSchema,
		Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		items, err := loadOffers(ctx)
		if err != nil {
			return fail(err)
		}
		query := strings.ToLower(strings.TrimSpace(args.Query))
		results := []any{}
		for _, item := range items {
			offer, _ := item.(map[string]any)
			if offer == nil {
				continue
			}
			haystack := strings.ToLower(fmt.Sprint(offer["product_name"], " ", offer["product_tagline"], " ", offer["primary_category"], " ", offer["terms"]))
			if query == "" || strings.Contains(haystack, query) {
				results = append(results, map[string]any{"id": offer["offer_id"], "title": offer["product_name"], "url": offer["website_url"]})
			}
		}
		return structured(map[string]any{"results": results})
	})

	s.AddTool(&mcp.Tool{
		Name:         "fetch",
		Title:        "Fetch offer",
		Description:  "Fetch one listed offer by id from search. Returns {id,title,text,url,metadata}.",
		InputSchema:  map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string", "description": "Offer id from a search result"}}},
		OutputSchema: outputSchema,
		Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		items, err := loadOffers(ctx)
		if err != nil {
			return fail(err)
		}
		for _, item := range items {
			offer, _ := item.(map[string]any)
			if id, _ := offer["offer_id"].(string); offer != nil && id == args.ID {
				text := fmt.Sprint(offer["product_name"], " — ", offer["product_tagline"], "\nCategory: ", offer["primary_category"], "\nCommission: ", offer["commission_value"], " ", offer["commission_currency"], " ", offer["commission_type"], "\nTerms: ", offer["terms"])
				return structured(map[string]any{
					"id":    id,
					"title": offer["product_name"],
					"text":  text,
					"url":   offer["website_url"],
					"metadata": map[string]any{
						"primary_category": offer["primary_category"],
						"commission_type":  offer["commission_type"],
						"joined":           offer["joined"],
					},
				})
			}
		}
		return fail(fmt.Errorf("no listed offer with id %q", args.ID))
	})
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
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 {
			return nil
		}
		return newHostedMCPServer(g, h.LoopbackURL, h.BaseURL, parts[1])
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
