package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/services"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLoopbackRedirectIgnoresPort(t *testing.T) {
	registered := []string{"http://localhost:1111/callback?listen=1", "https://example.com/cb"}
	if !redirectRegistered(registered, "http://localhost:2222/callback?listen=1") {
		t.Fatal("a different loopback port was rejected")
	}
	if !redirectRegistered(registered, "http://LocalHost:80/callback?listen=1") {
		t.Fatal("localhost case was rejected")
	}
	if redirectRegistered(registered, "http://localhost:2222/callback") || redirectRegistered(registered, "http://127.0.0.1:2222/callback?listen=1") || redirectRegistered(registered, "https://example.com:444/cb") {
		t.Fatal("port was ignored for a different query, host, or https callback")
	}
}

func TestLocalhostRedirectAnyPort(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := hostedTestServer(t, db)

	for _, redirect := range []string{
		"http://localhost/callback",
		"http://localhost:1/callback",
		"http://localhost:65535/callback",
		"http://127.0.0.1/callback",
		"http://127.0.0.1:9/callback",
		"http://[::1]:9/callback",
	} {
		rec, _ := postJSON(t, r, http.MethodPost, "/oauth/register", map[string]any{"redirect_uris": []string{redirect}, "token_endpoint_auth_method": "none"}, nil)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: %d %s", redirect, rec.Code, rec.Body.String())
		}
	}
	for _, redirect := range []string{"http://localhost.evil.com/callback", "http://user:pass@localhost/callback", "http://127.0.0.1.nip.io/callback"} {
		rec, body := postJSON(t, r, http.MethodPost, "/oauth/register", map[string]any{"redirect_uris": []string{redirect}}, nil)
		if rec.Code != http.StatusBadRequest || body["error"] != "invalid_redirect_uri" {
			t.Fatalf("%s: %d %s", redirect, rec.Code, rec.Body.String())
		}
	}

	created, createdBody := postJSON(t, r, http.MethodPost, "/oauth/register", map[string]any{
		"client_name": "Claude Code", "redirect_uris": []string{"http://localhost:1111/callback"}, "token_endpoint_auth_method": "none",
	}, nil)
	clientID, _ := createdBody["client_id"].(string)
	if created.Code != http.StatusCreated || clientID == "" {
		t.Fatalf("register: %d %s", created.Code, created.Body.String())
	}
	ask := func(uri string) (int, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id="+url.QueryEscape(clientID)+"&redirect_uri="+url.QueryEscape(uri), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var parsed map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
		errText, _ := parsed["error"].(string)
		return rec.Code, errText
	}
	if code, errText := ask("http://localhost:2222/callback"); code != http.StatusBadRequest || errText != "invalid_request" {
		t.Fatalf("other port: %d %s", code, errText)
	}
	if code, errText := ask("http://localhost:2222/other"); code != http.StatusBadRequest || errText != "invalid_client_or_redirect" {
		t.Fatalf("other path: %d %s", code, errText)
	}
	if code, errText := ask("http://127.0.0.1:2222/callback"); code != http.StatusBadRequest || errText != "invalid_client_or_redirect" {
		t.Fatalf("other host: %d %s", code, errText)
	}
}

func TestAgentAccountKeyCallsMCP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, server := hostedTestServer(t, db)
	r.POST("/v1/agent-accounts", (&AgentEntry{DB: db, BaseURL: h.BaseURL}).CreateAgentAccount)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "MCP Publisher", "email": "mcp-publisher@example.com", "domain": "mcp-publisher.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated || body["api_key"] == "" || body["plugin_key"] == "" {
		t.Fatalf("publisher signup: %d %s", created.Code, created.Body.String())
	}
	siteID := body["site_id"].(string)
	siteKey := body["api_key"].(string)
	pluginKey := body["plugin_key"].(string)
	grant, err := db.AgentAccountBearer(context.Background(), services.HashAPIKey(siteKey), siteKey, h.resource())
	if err != nil || grant.Scope != "publisher" || grant.SiteID != siteID || grant.MakerID != "" {
		t.Fatalf("publisher grant: %+v %v", grant, err)
	}

	makerRec, makerBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "maker", "name": "MCP Maker", "email": "mcp-maker@example.com", "source": "cli",
	}, nil)
	if makerRec.Code != http.StatusCreated || makerBody["api_key"] == "" {
		t.Fatalf("maker signup: %d %s", makerRec.Code, makerRec.Body.String())
	}
	makerID := makerBody["maker_id"].(string)
	makerKey := makerBody["api_key"].(string)
	makerGrant, err := db.AgentAccountBearer(context.Background(), services.HashAPIKey(makerKey), makerKey, h.resource())
	if err != nil || makerGrant.Scope != "maker" || makerGrant.MakerID != makerID || makerGrant.SiteID != "" {
		t.Fatalf("maker grant: %+v %v", makerGrant, err)
	}
	if _, err = db.AgentAccountBearer(context.Background(), services.HashAPIKey(pluginKey), pluginKey, h.resource()); err == nil {
		t.Fatal("plugin key became a grant")
	}
	plainKey := "plain-site-key-not-an-agent"
	createTestSite(t, db, plainKey, "plain.example", "Plain")
	if _, err = db.AgentAccountBearer(context.Background(), services.HashAPIKey(plainKey), plainKey, h.resource()); err == nil {
		t.Fatal("ordinary site key became a grant")
	}
	for _, token := range []string{pluginKey, plainKey, ""} {
		if code := mcpBearerStatus(r, token); code != http.StatusUnauthorized {
			t.Fatalf("bearer %q: %d", token, code)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	publisherTools := mcpToolNames(t, ctx, server.URL+"/mcp", siteKey)
	if !publisherTools["find_opportunities"] || !publisherTools["get_balance"] || publisherTools["list_products"] || publisherTools["register_product"] {
		t.Fatalf("publisher tools: %v", publisherTools)
	}
	found := callMCPTool(t, ctx, server.URL+"/mcp", siteKey, "find_opportunities", map[string]any{"context": "Northstar reviews pull requests before merge."})
	if found["status"] != "no_match" {
		t.Fatalf("find_opportunities: %v", found)
	}

	makerTools := mcpToolNames(t, ctx, server.URL+"/mcp", makerKey)
	if !makerTools["list_products"] || makerTools["find_opportunities"] || makerTools["get_balance"] {
		t.Fatalf("maker tools: %v", makerTools)
	}
	listed := callMCPTool(t, ctx, server.URL+"/mcp", makerKey, "list_products", map[string]any{})
	if _, ok := listed["products"]; !ok {
		t.Fatalf("list_products: %v", listed)
	}
}

func mcpBearerStatus(r http.Handler, token string) int {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func mcpToolNames(t *testing.T, ctx context.Context, endpoint, token string) map[string]bool {
	t.Helper()
	session := connectMCP(t, ctx, endpoint, token)
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	return names
}

func callMCPTool(t *testing.T, ctx context.Context, endpoint, token, name string, args map[string]any) map[string]any {
	t.Helper()
	session := connectMCP(t, ctx, endpoint, token)
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("%s: %+v", name, result)
	}
	out := map[string]any{}
	if err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func connectMCP(t *testing.T, ctx context.Context, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-key", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}}, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
