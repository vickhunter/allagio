package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestPoppyDiscovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := "https://allagio.example"
	h := &HostedHandler{BaseURL: base, AssetsPath: t.TempDir(), DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	schema := mustPoppySchema(t)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/poppy.json", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("schema: %v\n%s", err, rec.Body.String())
	}
	if doc["protocol_version"] != "0.4" {
		t.Fatalf("version %v", doc["protocol_version"])
	}
	org, _ := doc["organization"].(map[string]any)
	if org["name"] != "Allagio" || org["domain"] != "allagio.example" {
		t.Fatalf("organization %#v", org)
	}
	auth, _ := doc["auth"].(map[string]any)
	if auth["disabled"] != true || auth["issuer"] != nil || auth["scopes"] != nil || auth["scope_hints"] != nil {
		t.Fatalf("auth %#v", auth)
	}
	for _, key := range []string{"agent", "web", "tasks", "extensions", "supported_versions"} {
		if _, ok := doc[key]; ok {
			t.Fatalf("document lists %s", key)
		}
	}
	apis, _ := doc["apis"].([]any)
	if len(apis) != 1 {
		t.Fatalf("apis %#v", doc["apis"])
	}
	api, _ := apis[0].(map[string]any)
	if api["type"] != "mcp" || api["url"] != base+"/mcp" {
		t.Fatalf("api %#v", api)
	}
	want := "POST /v1/agent-tokens with {} and no key. POST /mcp with Authorization: Bearer."
	if api["description"] != want || !strings.Contains(discoveryLine, want) {
		t.Fatalf("description %q", api["description"])
	}
	if strings.Contains(rec.Body.String(), "Poppy support") || strings.Contains(strings.ToLower(rec.Body.String()), "/v1/device") {
		t.Fatalf("document claims support or a device field: %s", rec.Body.String())
	}
	assertNoPoppyCopy(t)
}

func TestPoppyOAuthDiscoveryChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := "https://allagio.example"
	h := &HostedHandler{BaseURL: base, AssetsPath: t.TempDir(), DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	poppy := getJSONMap(t, r, "/.well-known/poppy.json")
	root := getJSONMap(t, r, "/.well-known/oauth-protected-resource")
	mcp := getJSONMap(t, r, "/.well-known/oauth-protected-resource/mcp")
	auth := getJSONMap(t, r, "/.well-known/oauth-authorization-server")
	domain, _ := poppy["organization"].(map[string]any)["domain"].(string)
	if err := poppyOAuthChainError(base, domain, root, mcp, auth); err != nil {
		t.Fatal(err)
	}
}

func TestPoppyOAuthDiscoveryChainRejectsAMismatch(t *testing.T) {
	base := "https://allagio.example"
	root := map[string]any{"resource": base + "/mcp"}
	mcp := map[string]any{"resource": base + "/mcp"}
	auth := map[string]any{"protected_resources": []any{base, base + "/mcp"}}
	if err := poppyOAuthChainError(base, "allagio.example", root, mcp, auth); err == nil {
		t.Fatal("root resource matching the MCP path was accepted")
	}
	root["resource"] = base
	auth["protected_resources"] = []any{base}
	if err := poppyOAuthChainError(base, "allagio.example", root, mcp, auth); err == nil {
		t.Fatal("protected_resources missing the MCP URL was accepted")
	}
	auth["protected_resources"] = []any{base, base + "/mcp"}
	if err := poppyOAuthChainError(base, "other.example", root, mcp, auth); err == nil {
		t.Fatal("organization.domain mismatch was accepted")
	}
}

func poppyOAuthChainError(base, domain string, root, mcp, auth map[string]any) error {
	origin := strings.TrimRight(base, "/")
	if root["resource"] != origin {
		return fmt.Errorf("root resource %v, want %s", root["resource"], origin)
	}
	if mcp["resource"] != origin+"/mcp" {
		return fmt.Errorf("mcp resource %v, want %s/mcp", mcp["resource"], origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() != domain {
		return fmt.Errorf("organization.domain %s does not match %s", domain, origin)
	}
	if parsed.Scheme == "https" && root["resource"] != "https://"+domain {
		return fmt.Errorf("root resource %v, want https://%s", root["resource"], domain)
	}
	found := map[string]bool{}
	switch listed := auth["protected_resources"].(type) {
	case []any:
		for _, item := range listed {
			text, _ := item.(string)
			found[text] = true
		}
	case []string:
		for _, item := range listed {
			found[item] = true
		}
	}
	if !found[origin] || !found[origin+"/mcp"] {
		return fmt.Errorf("protected_resources %#v", auth["protected_resources"])
	}
	return nil
}

func getJSONMap(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("%s %d %s", path, rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestPoppyRejectsAnUnknownCapability(t *testing.T) {
	schema := mustPoppySchema(t)
	doc := map[string]any{
		"protocol_version": "0.4",
		"organization":     map[string]any{"name": "Allagio", "domain": "allagio.example"},
		"auth":             map[string]any{"disabled": true},
		"apis": []any{
			map[string]any{"type": "mcp", "url": "https://allagio.example/mcp", "description": "MCP"},
		},
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatal(err)
	}
	doc["tasks"] = map[string]any{"endpoint": "https://allagio.example/poppy/tasks"}
	if err := schema.Validate(doc); err == nil {
		t.Fatal("tasks was accepted")
	}
	delete(doc, "tasks")
	apis := doc["apis"].([]any)
	apis[0].(map[string]any)["type"] = "a2a"
	if err := schema.Validate(doc); err == nil {
		t.Fatal("api type a2a was accepted")
	}
}

func TestPoppySignedInMatchesSignedOut(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, h, r, _ := mountJoinTokens(t, db, 8)
	_, cookie := testOwnerSession(t, h, "poppy-owner", "poppy@example.com")
	plain := httptest.NewRecorder()
	r.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/.well-known/poppy.json", nil))
	signed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/poppy.json", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(signed, req)
	if plain.Code != http.StatusOK || signed.Body.String() != plain.Body.String() || !strings.Contains(plain.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("signed %d %s plain %d %s", signed.Code, signed.Body.String(), plain.Code, plain.Body.String())
	}
}

func assertNoPoppyCopy(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	for _, rel := range []string{
		"ui/landing/public/llms.txt",
		"affilio-wp/admin/build/landing-v9/llms.txt",
		"affilio-wp/admin/build/landing-v9/skill.md",
		"cli/README.md",
		"README.md",
		"docs/protocol/README.md",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "poppy.json") || strings.Contains(text, "Poppy support") || strings.Contains(text, "Poppy") {
			t.Fatalf("%s mentions Poppy", rel)
		}
	}
}

func mustPoppySchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "poppy", "schema", "poppy-0.4.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
