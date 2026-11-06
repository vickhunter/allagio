package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"affilio-api/internal/dcp"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestAgentCardDiscovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := "http://127.0.0.1:8080"
	h := &HostedHandler{BaseURL: base, AssetsPath: t.TempDir(), DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	(&AgentEntry{BaseURL: base}).Mount(r)
	r.GET("/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	schema := mustAgentCardSchema(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content type %s", ct)
	}
	var card map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(card); err != nil {
		t.Fatalf("schema: %v\n%s", err, rec.Body.String())
	}
	if card["name"] != "Allagio" {
		t.Fatalf("name %v", card["name"])
	}
	assertAgentCardTruthful(t, base, card, rec.Body.String())
	assertCardURLsAnswer(t, r, card)
}

func TestAgentCardSignedIn(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	base := "http://127.0.0.1:8080"
	h := &HostedHandler{DB: db, BaseURL: base, AssetsPath: t.TempDir(), DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	_, cookie := testOwnerSession(t, h, "agent-card-owner", "card@example.com")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") || !strings.Contains(rec.Body.String(), `"name":"Allagio"`) {
		t.Fatalf("signed in %d %s", rec.Code, rec.Body.String())
	}
}

func assertAgentCardTruthful(t *testing.T, base string, card map[string]any, raw string) {
	t.Helper()
	if strings.Contains(raw, "onrender.com") || strings.Contains(raw, "JSONRPC") || strings.Contains(raw, "HTTP+JSON") || strings.Contains(raw, "GRPC") || strings.Contains(strings.ToLower(raw), "/a2a") {
		t.Fatalf("card claims an A2A task endpoint or a fixed host: %s", raw)
	}
	if strings.Contains(strings.ToLower(raw), "base protocol") && !strings.Contains(raw, "not a base protocol") {
		t.Fatal("card calls Allagio a base protocol")
	}
	if regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`).MatchString(raw) {
		t.Fatal("card contains an email address")
	}
	if regexp.MustCompile(`(?i)(api[_-]?key|bearer|secret|password)\s*[:=]\s*\S+`).MatchString(raw) {
		t.Fatal("card contains a secret assignment")
	}
	ifaces, _ := card["supportedInterfaces"].([]any)
	if len(ifaces) != 2 {
		t.Fatalf("interfaces %#v", card["supportedInterfaces"])
	}
	seen := map[string]string{}
	for _, item := range ifaces {
		row, _ := item.(map[string]any)
		seen[row["protocolBinding"].(string)] = row["url"].(string)
		if _, ok := row["protocolVersion"]; ok {
			t.Fatalf("protocolVersion claims an A2A version: %#v", row)
		}
	}
	if seen["MCP"] != base+"/mcp" || seen["HTTP"] != base+"/v1/health" {
		t.Fatalf("interfaces %v", seen)
	}
	if card["documentationUrl"] != base+"/llms.txt" {
		t.Fatalf("docs %v", card["documentationUrl"])
	}
	var want []string
	for _, tool := range dcp.AllTools() {
		want = append(want, tool.Name)
	}
	skills, _ := card["skills"].([]any)
	var got []string
	for _, item := range skills {
		row, _ := item.(map[string]any)
		got = append(got, row["id"].(string))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("skills drifted\ngot %v\nwant %v", got, want)
	}
}

func assertCardURLsAnswer(t *testing.T, h http.Handler, card map[string]any) {
	t.Helper()
	urls := cardURLs(card)
	if len(urls) == 0 {
		t.Fatal("card has no URLs")
	}
	for _, raw := range urls {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		method := http.MethodGet
		if strings.HasSuffix(parsed.Path, "/mcp") {
			method = http.MethodPost
		}
		req := httptest.NewRequest(method, parsed.RequestURI(), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusOK, http.StatusUnauthorized, http.StatusForbidden:
		default:
			t.Fatalf("%s %s: %d %s", method, raw, rec.Code, rec.Body.String())
		}
	}
}

func cardURLs(v any) []string {
	var out []string
	switch node := v.(type) {
	case map[string]any:
		for _, child := range node {
			out = append(out, cardURLs(child)...)
		}
	case []any:
		for _, child := range node {
			out = append(out, cardURLs(child)...)
		}
	case string:
		if strings.HasPrefix(node, "http://") || strings.HasPrefix(node, "https://") {
			out = append(out, node)
		}
	}
	return out
}

func mustAgentCardSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "a2a", "schema", "a2a.v1.0.0.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rewritten, err := rewriteA2ARefs(raw)
	if err != nil {
		t.Fatal(err)
	}
	var bundle map[string]any
	if err = json.Unmarshal(rewritten, &bundle); err != nil {
		t.Fatal(err)
	}
	defs, _ := bundle["definitions"].(map[string]any)
	rawCard, err := json.Marshal(defs["Agent Card"])
	if err != nil {
		t.Fatal(err)
	}
	var card map[string]any
	if err = json.Unmarshal(rawCard, &card); err != nil {
		t.Fatal(err)
	}
	card["$schema"] = bundle["$schema"]
	card["definitions"] = defs
	encoded, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func rewriteA2ARefs(raw []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	root, _ := doc.(map[string]any)
	defs, _ := root["definitions"].(map[string]any)
	byCompact := map[string]string{}
	for key, value := range defs {
		row, _ := value.(map[string]any)
		title, _ := row["title"].(string)
		if title == "" {
			title = key
		}
		byCompact[strings.ReplaceAll(title, " ", "")] = key
	}
	walkA2ARefs(doc, byCompact)
	return json.Marshal(doc)
}

func walkA2ARefs(node any, byCompact map[string]string) {
	switch value := node.(type) {
	case map[string]any:
		if ref, ok := value["$ref"].(string); ok && !strings.HasPrefix(ref, "#") {
			name := ref
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}
			name = strings.TrimSuffix(name, ".jsonschema.json")
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:]
			}
			if key, ok := byCompact[name]; ok {
				value["$ref"] = "#/definitions/" + strings.ReplaceAll(key, " ", "%20")
			}
		}
		for _, child := range value {
			walkA2ARefs(child, byCompact)
		}
	case []any:
		for _, child := range value {
			walkA2ARefs(child, byCompact)
		}
	}
}

func TestAgentCardMentionsAreOnce(t *testing.T) {
	const path = "/.well-known/agent-card.json"
	if strings.Count(llmsText, path) != 1 {
		t.Fatalf("llms mentions %d", strings.Count(llmsText, path))
	}
	if strings.Count(skillMD, path) != 1 {
		t.Fatalf("skill mentions %d", strings.Count(skillMD, path))
	}
	if !strings.Contains(llmsFullDocument(), "ORP, works over MCP") {
		t.Fatal("full text lost the MCP sentence")
	}
	root := repoRoot(t)
	for _, rel := range []string{
		"ui/landing/public/llms.txt",
		"affilio-wp/admin/build/landing-v9/llms.txt",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != llmsText || strings.Count(string(body), path) != 1 {
			t.Fatalf("%s drifted", rel)
		}
	}
	skill, err := os.ReadFile(filepath.Join(root, "affilio-wp/admin/build/landing-v9/skill.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(skill) != skillMD || strings.Count(string(skill), path) != 1 {
		t.Fatal("skill.md drifted")
	}
}
