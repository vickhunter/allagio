package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// receiverPrefix maps receiver names to their mount prefix, per file. Gin
// group declarations are resolved by hand here; adding a file that registers
// routes means extending this table.
var openapiReceiverPrefix = map[string]map[string]string{
	"cmd/server/main.go": {
		"r": "", "v1": "/v1", "signals": "/v1/agent-signals", "secured": "/v1",
		"maker": "/v1/maker", "listings": "/v1/listings", "admin": "/v1/program-discovery",
		"offerAdmin": "/v1/offer-review", "payoutAdmin": "/v1/payout-batches",
	},
	"internal/handlers/hosted_auth.go": {"public": "", "owner": "/owner", "r": ""},
	"internal/handlers/workspace.go": {
		"public": "", "owner": "/owner", "api": "/owner/api",
		"maker": "/owner/api", "publisher": "/owner/api", "review": "/owner/api/review",
	},
	"internal/handlers/demo.go":       {"public": ""},
	"internal/handlers/agent_entry.go": {"r": ""},
	"internal/handlers/orp_host.go":    {"r": ""},
	"internal/handlers/orp_deal.go":    {"r": ""},
	"internal/handlers/a2a.go":         {"r": ""},
}

// openapiExclusions: registered routes that are not part of the JSON API —
// HTML pages, markdown twins, static assets, HTML form posts, and tool-only
// endpoints. HEAD is skipped wholesale: it mirrors GET. Non-literal paths
// (constants, concat) are listed in openapiConstantRoutes instead.
var openapiExclusions = map[string]string{
	"Any /mcp":      "documented as POST /mcp",
	"POST /orp/monetize": "constant path — see openapiConstantRoutes",

	"GET /": "landing page (HTML)", "GET /agents": "landing page (HTML)", "GET /agents/": "landing page (HTML)",
	"GET /makers": "landing page (HTML)", "GET /makers/": "landing page (HTML)", "GET /makers/{slug}": "directory profile (HTML)", "GET /makers/{slug}/": "directory profile (HTML)",
	"GET /publishers": "landing page (HTML)", "GET /publishers/": "landing page (HTML)", "GET /publishers/setup": "landing page (HTML)", "GET /publishers/setup/": "landing page (HTML)", "GET /publishers/{slug}": "directory profile (HTML)", "GET /publishers/{slug}/": "directory profile (HTML)",
	"GET /protocol": "landing page (HTML)", "GET /protocol/": "landing page (HTML)",
	"GET /you": "landing page (HTML)", "GET /you/": "landing page (HTML)",
	"GET /use-cases": "landing page (HTML)", "GET /use-cases/": "landing page (HTML)",
	"GET /privacy": "legal page (HTML)", "GET /privacy/": "legal page (HTML)",
	"GET /terms": "legal page (HTML)", "GET /terms/": "legal page (HTML)",
	"GET /reference": "docs page (HTML)", "GET /reference/": "docs page (HTML)",
	"GET /docs": "Scalar API reference (HTML)",
	"GET /claim/{code}": "claim page (HTML)", "POST /claim/{code}": "claim form post (HTML)",
	"GET /orp/approve/{id}": "human approval page (HTML)", "POST /orp/approve/{id}": "human approval form post (HTML)",
	"GET /orp/proof/{id}": "proof page (HTML)",
	"GET /device": "device page (HTML)", "POST /device": "device form post (HTML)",
	"GET /oauth/authorize": "consent page (HTML)",
	"GET /auth/login": "sign-in redirect", "GET /auth/callback": "sign-in callback",
	"GET /demo": "demo page (HTML)", "GET /demo/{page}": "demo page (HTML)", "POST /demo/operator": "demo form post (HTML)",
	"GET /owner": "dashboard page (HTML)", "GET /owner/connect-site": "dashboard page (HTML)", "GET /owner/placements/{id}": "dashboard page (HTML)", "GET /owner/products/{id}/connection": "dashboard page (HTML)",
	"POST /owner/accounts": "form post (HTML)", "POST /owner/authorize": "form post (HTML)", "POST /owner/connect-site": "form post (HTML)",
	"POST /owner/grants/{id}/revoke": "form post (HTML)", "POST /owner/human-approval": "form post (HTML)", "POST /owner/logout": "form post (HTML)",
	"POST /owner/digest": "form post (HTML)", "POST /owner/notify": "form post (HTML)", "POST /owner/offers/{id}/list": "form post (HTML)", "POST /owner/orp-payouts/{id}/sent": "form post (HTML)",
	"POST /owner/payouts/{id}/confirm-received": "form post (HTML)", "POST /owner/placements/{id}/approve": "form post (HTML)", "POST /owner/placements/{id}/revoke": "form post (HTML)",
	"POST /owner/products/{id}/connection": "form post (HTML)", "POST /owner/spaces/{id}/list": "form post (HTML)", "POST /owner/step-modes": "form post (HTML)",
	"POST /owner/telegram/code": "form post (HTML)", "POST /owner/telegram/unlink": "form post (HTML)",
	"GET /agents.md": "markdown twin", "GET /home.md": "markdown twin", "GET /makers.md": "markdown twin", "GET /privacy.md": "markdown twin",
	"GET /protocol.md": "markdown twin", "GET /publishers.md": "markdown twin", "GET /publishers/setup.md": "markdown twin", "GET /reference.md": "markdown twin",
	"GET /terms.md": "markdown twin", "GET /use-cases.md": "markdown twin", "GET /you.md": "markdown twin",
	"GET /sitemap.xml": "sitemap (XML)", "GET /assets/": "static asset", "GET /downloads/": "static asset", "GET /pilot": "POC report file (config-gated)",
}

// openapiConstantRoutes: routes registered with non-literal paths — verified
// by hand, listed here so spec paths are not flagged as phantom.
var openapiConstantRoutes = map[string]string{
	"POST /orp/monetize": "MonetizationSuggestPath in monetization_suggest.go",
}

var openapiRoutePat = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\.(GET|POST|PUT|PATCH|DELETE|HEAD|Any)\("([^"]*)"`)

func openapiSpecOps(t *testing.T) map[string]bool {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(openapiDocument, &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	ops := map[string]bool{}
	for path, item := range paths {
		obj, _ := item.(map[string]any)
		for method := range obj {
			ops[strings.ToUpper(method)+" "+path] = true
		}
	}
	return ops
}

func openapiCodeRoutes(t *testing.T) map[string]bool {
	t.Helper()
	root := "../.."
	routes := map[string]bool{}
	files := []string{filepath.Join(root, "cmd/server/main.go")}
	entries, err := filepath.Glob(filepath.Join(root, "internal/handlers/*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range entries {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		files = append(files, f)
	}
	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		prefixes, tracked := openapiReceiverPrefix[rel]
		if !tracked {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range openapiRoutePat.FindAllSubmatch(body, -1) {
			recv, method, path := string(m[1]), string(m[2]), string(m[3])
			prefix, known := prefixes[recv]
			if !known {
				continue
			}
			full := prefix + path
			if full == "" {
				full = "/"
			}
			for strings.Contains(full, "//") {
				full = strings.ReplaceAll(full, "//", "/")
			}
			full = regexp.MustCompile(`:\w+`).ReplaceAllStringFunc(full, func(s string) string { return "{" + s[1:] + "}" })
			full = regexp.MustCompile(`\*\w+`).ReplaceAllStringFunc(full, func(s string) string { return "{" + s[1:] + "}" })
			routes[method+" "+full] = true
		}
	}
	return routes
}

func TestOpenAPICoverage(t *testing.T) {
	spec := openapiSpecOps(t)
	code := openapiCodeRoutes(t)
	for route := range code {
		method := strings.SplitN(route, " ", 2)[0]
		if method == "HEAD" {
			continue
		}
		if spec[route] || openapiExclusions[route] != "" || openapiConstantRoutes[route] != "" {
			continue
		}
		if method == "Any" {
			// Any registers every verb; the JSON surface is POST.
			_, path, _ := strings.Cut(route, " ")
			if spec["POST "+path] {
				continue
			}
		}
		t.Errorf("route %s is registered in code but missing from openapi.json (add it, or list it in openapiExclusions)", route)
	}
	for route := range spec {
		if code[route] {
			continue
		}
		method, path, _ := strings.Cut(route, " ")
		covered := openapiConstantRoutes[route] != "" || (method != "Any" && code["Any "+path])
		if !covered {
			t.Errorf("openapi.json documents %s but no code registers it (phantom route)", route)
		}
	}
}

func TestOpenAPIDocumentShape(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(openapiDocument, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Errorf("openapi version %v, want 3.1.0", doc["openapi"])
	}
	info, _ := doc["info"].(map[string]any)
	if info["title"] == "" || info["version"] == "" || info["description"] == "" {
		t.Error("info must carry title, version, description")
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) == 0 {
		t.Fatal("no paths")
	}
	schemes, _ := doc["components"].(map[string]any)["securitySchemes"].(map[string]any)
	for path, item := range paths {
		for method, raw := range item.(map[string]any) {
			op, _ := raw.(map[string]any)
			label := method + " " + path
			if op["summary"] == "" {
				t.Errorf("%s: missing summary", label)
			}
			if op["responses"] == nil {
				t.Errorf("%s: missing responses", label)
			}
			tags, _ := op["tags"].([]any)
			if len(tags) == 0 {
				t.Errorf("%s: missing tags", label)
			}
			sec, _ := op["security"].([]any)
			for _, s := range sec {
				for name := range s.(map[string]any) {
					if schemes[name] == nil {
						t.Errorf("%s: unknown security scheme %q", label, name)
					}
				}
			}
		}
	}
}

func TestOpenAPIServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/openapi.json", ServeOpenAPI("https://allagio.example"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=300") {
		t.Errorf("cache-control %q", cc)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("public CORS header missing")
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["servers"].([]any)
	if servers[0].(map[string]any)["url"] != "https://allagio.example" {
		t.Errorf("servers[0].url = %v, want substituted base", servers[0])
	}
}
