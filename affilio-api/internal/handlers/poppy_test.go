package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
