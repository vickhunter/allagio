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

func TestLandingUnknownPathIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/not-a-page", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, needle := range []string{"That page is not on Offer Rails.", `aria-label="Main"`, `href="/agents"`, ">Home<"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("missing %s", needle)
		}
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatalf("public headers %v", rec.Header())
	}

	api := httptest.NewRecorder()
	r.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if api.Code != http.StatusServiceUnavailable || strings.Contains(api.Body.String(), "That page is not on Offer Rails.") || strings.Contains(api.Body.String(), `"items"`) || !strings.Contains(api.Body.String(), `"error":"directory unavailable"`) {
		t.Fatalf("api miss %d %s", api.Code, api.Body.String())
	}
}

func TestLandingTemplateErrorServesFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "home.html"), []byte("<html>{{if</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{BaseURL: "http://127.0.0.1:8080", AssetsPath: dir, DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assertLandingUnavailable(t, "Home", rec)
}

func TestLandingMissingFileIsPlainUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "home.html"), []byte("<!doctype html><title>Offer Rails</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{BaseURL: "http://127.0.0.1:8080", AssetsPath: dir, DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	for _, tc := range []struct{ path, name string }{
		{"/makers", "Makers"},
		{"/makers/", "Makers"},
		{"/protocol", "Protocol"},
		{"/protocol/", "Protocol"},
		{"/publishers/", "Publishers"},
		{"/publishers/setup/", "Publisher setup"},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		assertLandingUnavailable(t, tc.name, rec)
	}
}

func assertLandingUnavailable(t *testing.T, page string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusServiceUnavailable || rec.Body.Len() == 0 {
		t.Fatalf("%s: %d %q", page, rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if rec.Header().Get("Retry-After") != "60" || !strings.Contains(body, "<h1>"+page+"</h1>") || !strings.Contains(body, "This page is temporarily unavailable.") || !strings.Contains(body, `<nav><a href="/">Home</a></nav>`) {
		t.Fatalf("%s retry %q body %q", page, rec.Header().Get("Retry-After"), body)
	}
	lower := strings.ToLower(body)
	for _, banned := range []string{"distributoor", "affilio-api", "offerrails.com", "open source", "open-source"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("%s contains %q", page, banned)
		}
	}
}

func TestLandingStaticShellsRenderTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)
	for _, path := range []string{
		"/landing-v9",
		"/landing-v9/",
		"/landing-v9/index.html",
		"/landing-v9/not-found/index.html",
		"/landing-v9/not-found/",
		"/landing-v9/profile/index.html",
		"/landing-v9/profile",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body := rec.Body.String()
		if strings.Contains(body, "{{") || strings.Contains(body, "InstallerReady") {
			t.Fatalf("%s still serves template source, status %d", path, rec.Code)
		}
		if !strings.Contains(body, `name="or-plugin" content="0"`) || !strings.Contains(body, "That page is not on Offer Rails.") {
			t.Fatalf("%s missing rendered not-found page, status %d", path, rec.Code)
		}
		if path == "/landing-v9" || path == "/landing-v9/" || path == "/landing-v9/index.html" {
			if rec.Code != http.StatusNotFound || rec.Body.Len() == 0 {
				t.Fatalf("%s status %d len %d", path, rec.Code, rec.Body.Len())
			}
		}
	}
}
