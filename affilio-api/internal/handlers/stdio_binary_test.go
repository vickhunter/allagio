package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"affilio-api/internal/dcp"
)

func buildMCP(t *testing.T, pkg string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), filepath.Base(pkg))
	build := exec.Command("go", "build", "-o", binary, pkg)
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v %s", pkg, err, output)
	}
	return binary
}

func (a *stdioAgent) request(t *testing.T, method string, params map[string]interface{}) map[string]interface{} {
	t.Helper()
	a.id++
	body := map[string]interface{}{"jsonrpc": "2.0", "id": a.id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.in.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	if !a.out.Scan() {
		t.Fatalf("MCP process produced no response: %v", a.out.Err())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(a.out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestLegacyStdioBinaryStarts(t *testing.T) {
	binary := buildMCP(t, "./cmd/distributoor-mcp")
	agent := startStdioAgent(t, binary, "http://127.0.0.1:9", "", "")
	resp := agent.request(t, "initialize", map[string]interface{}{"protocolVersion": "2025-06-18"})
	result, _ := resp["result"].(map[string]interface{})
	info, _ := result["serverInfo"].(map[string]interface{})
	if info["name"] != dcp.ServerName || info["title"] != dcp.ServerTitle {
		t.Fatalf("serverInfo %#v", info)
	}
}

func TestOfferRailsStdioUsesLegacyAPIURL(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/revenue/preview" {
			http.NotFound(w, r)
			return
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"preview"}`))
	}))
	defer srv.Close()
	binary := buildMCP(t, "./cmd/offer-rails-mcp")
	agent := startStdioAgent(t, binary, srv.URL, "", "")
	result := agent.call(t, "preview_revenue", map[string]interface{}{"context": "A short note about email inbox tools for a tutorial."})
	if result["status"] != "preview" || hits != 1 {
		t.Fatalf("status %v hits %d", result["status"], hits)
	}
}
