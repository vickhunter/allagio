package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHostedMCPTransport(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// The wrapped API: the same routes the MCP verbs call, served over HTTP.
	api := newPayoutsRouter(db)
	makers := &MakersHandler{DB: db}
	api.POST("/v1/makers/register", makers.Register)
	apiServer := httptest.NewServer(api)
	defer apiServer.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	mcp := &MCPHandler{LoopbackURL: apiServer.URL}
	r.POST("/v1/mcp", mcp.Receive)

	post := func(body string, headers map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(body))
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		out := map[string]interface{}{}
		if rec.Body.Len() > 0 {
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
		}
		return rec, out
	}

	// Handshake needs no auth.
	rec, body := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil)
	if rec.Code != http.StatusOK || body["result"] == nil {
		t.Fatalf("initialize: expected 200 with result, got %d %v", rec.Code, body)
	}

	// Notifications are acknowledged with no body.
	rec, _ = post(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("notification: expected 202, got %d", rec.Code)
	}

	// A maker verb end to end: register_maker creates a real row.
	rec, body = post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"register_maker","arguments":{"email":"hosted@example.com","name":"Hosted"}}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("register_maker: expected 200, got %d", rec.Code)
	}
	result := body["result"].(map[string]interface{})
	if result["isError"] != false {
		t.Fatalf("register_maker: expected success, got %v", result)
	}
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM makers WHERE email='hosted@example.com'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected registered maker, count=%d err=%v", count, err)
	}

	// A publisher verb rides the X-API-Key header.
	siteKey := "site-key-mcp"
	createTestSite(t, db, siteKey, "mcpblog.example.com", "Blog")
	rec, body = post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_balance"}}`, map[string]string{"X-API-Key": siteKey})
	if rec.Code != http.StatusOK {
		t.Fatalf("get_balance: expected 200, got %d", rec.Code)
	}
	result = body["result"].(map[string]interface{})
	if result["isError"] != false {
		t.Fatalf("get_balance: expected success, got %v", result)
	}
	text := result["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, "balances") {
		t.Fatalf("unexpected balance payload: %s", text)
	}

	// Without a site key, the publisher verb explains what is missing.
	rec, body = post(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_balance"}}`, nil)
	result = body["result"].(map[string]interface{})
	if rec.Code != http.StatusOK || result["isError"] != true {
		t.Fatalf("expected isError without site key, got %d %v", rec.Code, result)
	}
}
