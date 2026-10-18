package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func writeBuiltPage(t *testing.T, dir, name, body string) {
	t.Helper()
	page := filepath.Join(dir, "landing-v9", name)
	if err := os.MkdirAll(page, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(page, "index.html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mountBuiltLanding(t *testing.T, dir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    dir,
		DownloadsPath: t.TempDir(),
	}).Mount(r)
	(&AgentEntry{AssetsPath: dir}).Mount(r)
	return r
}

func TestBuiltLandingPagesAndLLMsTxt(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]string{
		"privacy": "<!doctype html><title>Privacy fixture</title>{{.BaseURL}}",
		"terms":   "<!doctype html><title>Terms fixture</title>{{.BaseURL}}",
		"agents":  "<!doctype html><title>Agents fixture</title>{{.BaseURL}}",
	}
	for name, body := range pages {
		writeBuiltPage(t, dir, name, body)
	}
	llmsPath := filepath.Join(dir, "landing-v9", "llms.txt")
	llms := []byte("# built llms\nunique-built-llms-token\n")
	if err := os.WriteFile(llmsPath, llms, 0o644); err != nil {
		t.Fatal(err)
	}
	wantLLMs, err := os.ReadFile(llmsPath)
	if err != nil {
		t.Fatal(err)
	}
	r := mountBuiltLanding(t, dir)

	checks := []struct {
		path    string
		ctype   string
		body    string
		noindex bool
	}{
		{"/privacy", "text/html; charset=utf-8", pages["privacy"], true},
		{"/privacy/", "text/html; charset=utf-8", pages["privacy"], true},
		{"/terms", "text/html; charset=utf-8", pages["terms"], true},
		{"/terms/", "text/html; charset=utf-8", pages["terms"], true},
		{"/agents", "text/html; charset=utf-8", pages["agents"], false},
		{"/agents/", "text/html; charset=utf-8", pages["agents"], false},
	}
	for _, check := range checks {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, check.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", check.path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Content-Type") != check.ctype {
			t.Fatalf("%s content-type %q", check.path, rec.Header().Get("Content-Type"))
		}
		if rec.Body.String() != check.body {
			t.Fatalf("%s body %q", check.path, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "404 page not found") {
			t.Fatalf("%s served the 404 page", check.path)
		}
		robots := rec.Header().Get("X-Robots-Tag")
		if check.noindex && robots != "noindex" {
			t.Fatalf("%s X-Robots-Tag %q", check.path, robots)
		}
		if !check.noindex && robots != "" {
			t.Fatalf("%s X-Robots-Tag %q", check.path, robots)
		}
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/llms.txt: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("/llms.txt content-type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(wantLLMs) {
		t.Fatalf("/llms.txt body %q", rec.Body.String())
	}

	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/no-such-page", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown path: %d %s", missing.Code, missing.Body.String())
	}
	if !strings.Contains(missing.Body.String(), "404 page not found") {
		t.Fatalf("unknown path body %q", missing.Body.String())
	}
}

func TestLegalPagesNoindexUntilApproved(t *testing.T) {
	if LEGAL_PAGES_APPROVED {
		t.Fatal("LEGAL_PAGES_APPROVED defaults to false")
	}
	dir := t.TempDir()
	writeBuiltPage(t, dir, "privacy", "<!doctype html><title>Privacy</title>")
	writeBuiltPage(t, dir, "terms", "<!doctype html><title>Terms</title>")
	writeBuiltPage(t, dir, "agents", "<!doctype html><title>Agents</title>")
	r := mountBuiltLanding(t, dir)

	robots := func(path string, status int) string {
		t.Helper()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != status {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Header().Get("X-Robots-Tag")
	}
	noindex := []string{
		"/privacy", "/privacy/", "/terms", "/terms/",
		"/landing-v9/privacy/", "/landing-v9/terms/",
	}
	redirects := []string{
		"/landing-v9/privacy", "/landing-v9/privacy/index.html",
		"/landing-v9/terms", "/landing-v9/terms/index.html",
	}
	// Extra slashes and dot segments still resolve to the draft pages.
	dirty := []string{
		"/landing-v9//privacy/", "/landing-v9/privacy//", "/landing-v9/privacy/./",
		"/landing-v9//privacy/index.html", "/landing-v9/privacy/./index.html",
		"/landing-v9//terms/", "/landing-v9/terms//", "/landing-v9/terms/./",
		"/landing-v9//terms/index.html", "/landing-v9/terms/./index.html",
	}
	for _, path := range noindex {
		if got := robots(path, http.StatusOK); got != "noindex" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}
	for _, path := range redirects {
		if got := robots(path, http.StatusMovedPermanently); got != "noindex" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}
	for _, path := range dirty {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK && rec.Code != http.StatusMovedPermanently {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if rec.Code == http.StatusOK && !strings.Contains(rec.Body.String(), "<title>") {
			t.Fatalf("%s did not serve the page: %s", path, rec.Body.String())
		}
		if got := rec.Header().Get("X-Robots-Tag"); got != "noindex" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}
	for _, path := range []string{"/landing-v9//agents/", "/landing-v9/agents/./", "/landing-v9/agents/./index.html"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Robots-Tag"); got != "" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}
	open := []string{"/agents", "/agents/", "/landing-v9/agents/"}
	for _, path := range open {
		if got := robots(path, http.StatusOK); got != "" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}
	openRedirects := []string{"/landing-v9/agents", "/landing-v9/agents/index.html"}
	for _, path := range openRedirects {
		if got := robots(path, http.StatusMovedPermanently); got != "" {
			t.Fatalf("%s X-Robots-Tag %q", path, got)
		}
	}

	LEGAL_PAGES_APPROVED = true
	t.Cleanup(func() { LEGAL_PAGES_APPROVED = false })
	for _, path := range append(noindex, open...) {
		if got := robots(path, http.StatusOK); got != "" {
			t.Fatalf("approved %s X-Robots-Tag %q", path, got)
		}
	}
	for _, path := range append(redirects, openRedirects...) {
		if got := robots(path, http.StatusMovedPermanently); got != "" {
			t.Fatalf("approved %s X-Robots-Tag %q", path, got)
		}
	}
	for _, path := range dirty {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Robots-Tag"); got != "" {
			t.Fatalf("approved %s X-Robots-Tag %q", path, got)
		}
	}
}

func TestBuiltLandingPagesMissingFilesStay404(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "landing-v9"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := mountBuiltLanding(t, dir)
	for _, path := range []string{"/privacy", "/terms", "/agents", "/no-such-page"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/docs/device-login.md") {
		t.Fatalf("generated llms: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("generated llms content-type %q", rec.Header().Get("Content-Type"))
	}
}
