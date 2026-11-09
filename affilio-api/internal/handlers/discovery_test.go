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

func TestNoAgentCardSignedOut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{BaseURL: "http://127.0.0.1:8080", AssetsPath: t.TempDir(), DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	assertNoAgentCard(t, r, nil)
}

func TestNoAgentCardSignedIn(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, h, r, _ := mountJoinTokens(t, db, 8)
	_, cookie := testOwnerSession(t, h, "discovery-owner", "discovery@example.com")
	assertNoAgentCard(t, r, cookie)
}

func assertNoAgentCard(t *testing.T, h http.Handler, cookie *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code == http.StatusOK || strings.Contains(body, "supportedInterfaces") || strings.Contains(body, "protocolVersion") || strings.Contains(body, "protocolBinding") {
		t.Fatalf("agent card is served: %d %s", rec.Code, body)
	}
}

func TestDiscoveryCopyStaysNeutral(t *testing.T) {
	if strings.Count(llmsText, discoveryLine) != 1 || strings.Count(skillMD, discoveryLine) != 1 {
		t.Fatal("discovery sentence drifted in llms or skill")
	}
	if !strings.Contains(llmsFullDocument(), "ORP, works over MCP") {
		t.Fatal("full text lost the MCP sentence")
	}
	if strings.Contains(llmsFullDocument(), "A2A support is not implemented") {
		t.Fatal("full text grew a second discovery copy")
	}
	root := repoRoot(t)
	readme, err := os.ReadFile(filepath.Join(root, "docs/protocol/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	surfaces := []string{llmsText, skillMD, string(readme)}
	for _, rel := range []string{
		"ui/landing/public/llms.txt",
		"affilio-wp/admin/build/landing-v9/llms.txt",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != llmsText {
			t.Fatalf("%s drifted", rel)
		}
		surfaces = append(surfaces, string(body))
	}
	skill, err := os.ReadFile(filepath.Join(root, "affilio-wp/admin/build/landing-v9/skill.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(skill) != skillMD {
		t.Fatal("skill.md drifted")
	}
	surfaces = append(surfaces, string(skill))
	banned := []string{
		"Allagio Protocol",
		"protocolBinding",
		"protocolVersion",
		"supportedInterfaces",
		"JSONRPC",
		"HTTP+JSON",
		"agent-card",
		"/v1/health",
		"compared with",
		"better than",
	}
	for _, surface := range surfaces {
		for _, phrase := range banned {
			if strings.Contains(surface, phrase) {
				t.Fatalf("discovery copy contains %q", phrase)
			}
		}
		if regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`).MatchString(surface) {
			t.Fatal("discovery copy contains an email address")
		}
		if regexp.MustCompile(`(?i)(api[_-]?key|secret|password)\s*[:=]\s*\S+`).MatchString(surface) {
			t.Fatal("discovery copy contains a secret")
		}
	}
	if strings.Count(string(readme), discoveryLine) != 1 || !strings.Contains(string(readme), "does not serve an Agent Card") {
		t.Fatal("protocol README discovery drifted")
	}
	err = filepath.WalkDir(filepath.Join(root, "ui/landing/src"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, phrase := range []string{"Allagio Protocol", "compared with A2A", "better than A2A", "unlike A2A"} {
			if strings.Contains(text, phrase) {
				t.Fatalf("%s contains %q", path, phrase)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryEndpointsUseDocumentedMethod(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r, _ := mountJoinTokens(t, db, 8)
	const origin = "https://allagio.example"
	r.GET("/llms.txt", (&AgentEntry{BaseURL: origin}).LLMs)
	calls := discoveryCalls(t, discoveryLine)
	if len(calls) != 2 || calls[0].path != "/v1/agent-tokens" || calls[1].path != "/mcp" {
		t.Fatalf("calls %#v", calls)
	}
	var token string
	for _, call := range calls {
		if call.path == "/v1/health" || strings.Contains(call.path, "agent-card") {
			t.Fatalf("advertised %s", call.path)
		}
		var body string
		headers := map[string]string{}
		switch {
		case strings.Contains(call.note, "no key"):
			if call.method != http.MethodPost {
				t.Fatalf("token method %s", call.method)
			}
			body = "{}"
		case strings.Contains(call.note, "Authorization: Bearer"):
			if call.method != http.MethodPost || token == "" {
				t.Fatalf("mcp call %s before a token", call.method)
			}
			body = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
			headers["Authorization"] = "Bearer " + token
			headers["Accept"] = "application/json, text/event-stream"
		default:
			t.Fatalf("undocumented auth for %s %s", call.method, call.path)
		}
		req := httptest.NewRequest(call.method, call.path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if call.path == "/v1/agent-tokens" {
			if rec.Code != http.StatusCreated {
				t.Fatalf("token %d %s", rec.Code, rec.Body.String())
			}
			var doc map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			token, _ = doc["access_token"].(string)
			if token == "" || doc["token_type"] != "Bearer" {
				t.Fatalf("token body %s", rec.Body.String())
			}
			continue
		}
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"result"`) {
			t.Fatalf("mcp %d %s", rec.Code, rec.Body.String())
		}
	}
	served := httptest.NewRecorder()
	r.ServeHTTP(served, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if served.Code != http.StatusOK || !strings.Contains(served.Body.String(), "POST /v1/agent-tokens") || !strings.Contains(served.Body.String(), "POST /mcp") || !strings.Contains(served.Body.String(), "The configured origin is "+origin+".") || strings.Contains(served.Body.String(), "onrender.com") {
		t.Fatalf("served discovery %d %s", served.Code, served.Body.String())
	}
}

type discoveryCall struct {
	method string
	path   string
	note   string
}

func discoveryCalls(t *testing.T, line string) []discoveryCall {
	t.Helper()
	re := regexp.MustCompile(`(GET|POST|PATCH|DELETE) (/[A-Za-z0-9._/-]+)`)
	locs := re.FindAllStringSubmatchIndex(line, -1)
	if len(locs) == 0 {
		t.Fatal("discovery line has no URL")
	}
	var calls []discoveryCall
	for i, loc := range locs {
		end := len(line)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		calls = append(calls, discoveryCall{
			method: line[loc[2]:loc[3]],
			path:   line[loc[4]:loc[5]],
			note:   line[loc[1]:end],
		})
	}
	return calls
}
