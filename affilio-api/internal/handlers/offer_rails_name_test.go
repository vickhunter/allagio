package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/dcp"

	"github.com/gin-gonic/gin"
)

// TestOfferRailsNameOnLocalAPI starts the discovery and MCP handlers the
// hosted process serves, without a database. initialize, tools/list, and
// dcp.json must use the one Offer Rails server name. /version is the 404
// page whose meta tags are Go templates.
func TestOfferRailsNameOnLocalAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)
	r.GET("/.well-known/dcp.json", ServeDCPDiscovery("https://offer-rails.example"))
	r.POST("/v1/mcp", (&MCPHandler{LoopbackURL: "http://127.0.0.1:9"}).Receive)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	initBody := postRPC(t, server.URL+"/v1/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"local","version":"0"}}}`)
	info := initBody["result"].(map[string]any)["serverInfo"].(map[string]any)
	if info["name"] != dcp.ServerName || info["title"] != dcp.ServerTitle {
		t.Fatalf("serverInfo %+v", info)
	}

	listBody := postRPC(t, server.URL+"/v1/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := listBody["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(dcp.AllTools()) {
		t.Fatalf("tools %d want %d", len(tools), len(dcp.AllTools()))
	}
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		tool := item.(map[string]any)
		name, _ := tool["name"].(string)
		desc, _ := tool["description"].(string)
		if name == "" || strings.Contains(strings.ToLower(name+" "+desc), "distributoor") {
			t.Fatalf("tool %q %q", name, desc)
		}
		names = append(names, name)
	}
	encoded, err := json.MarshalIndent(listBody["result"], "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tools/list\n%s", encoded)
	t.Logf("tool names: %s", strings.Join(names, ", "))

	discovery := getJSON(t, server.URL+"/.well-known/dcp.json")
	if discovery["name"] != dcp.ServerName {
		t.Fatalf("dcp name %v", discovery["name"])
	}
	mcp := discovery["mcp"].(map[string]any)
	stdio := mcp["stdio"].(map[string]any)
	if stdio["command"] != dcp.ServerName {
		t.Fatalf("stdio command %v", stdio["command"])
	}
	env, _ := json.Marshal(stdio["env"])
	for _, want := range []string{dcp.EnvAPIURL, dcp.EnvAPIKey, dcp.EnvSiteKey} {
		if !strings.Contains(string(env), want) {
			t.Fatalf("stdio env %s missing %s", env, want)
		}
	}
	rawDiscovery, _ := json.Marshal(discovery)
	if strings.Contains(strings.ToLower(string(rawDiscovery)), "distributoor") {
		t.Fatalf("dcp.json still says distributoor: %s", rawDiscovery)
	}
	t.Logf("dcp.json name %s command %s env %s", discovery["name"], stdio["command"], env)

	page, err := http.Get(server.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	html, err := io.ReadAll(page.Body)
	page.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != http.StatusNotFound {
		t.Fatalf("/version %d", page.StatusCode)
	}
	body := string(html)
	if strings.Contains(body, "{{") || strings.Contains(body, "InstallerReady") {
		t.Fatalf("404 meta still has template source:\n%s", body)
	}
	if !strings.Contains(body, `<meta name="or-install" content="">`) || !strings.Contains(body, `<meta name="or-plugin" content="0">`) {
		t.Fatalf("404 meta tags:\n%s", body)
	}
}

func postRPC(t *testing.T, url, payload string) map[string]any {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %d %s", url, resp.StatusCode, raw)
	}
	var out map[string]any
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("rpc error %v", out["error"])
	}
	return out
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %d %s", url, resp.StatusCode, raw)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err = dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}
