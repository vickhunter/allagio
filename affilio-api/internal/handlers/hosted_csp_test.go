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

func TestHostedCSPAllowsEmbeddedFonts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "home.html"), []byte(`<!doctype html><title>{{.BaseURL}}</title>`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	(&HostedHandler{BaseURL: "https://pilot.example", AssetsPath: dir}).Mount(r)

	kept := []string{
		"default-src 'none'",
		"style-src 'self' 'unsafe-inline'",
		"script-src 'self'",
		"connect-src 'self'",
		"img-src 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"base-uri 'none'",
	}
	for _, path := range []string{"/", "/owner"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		csp := rec.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "font-src 'self' data:") {
			t.Fatalf("%s: hosted CSP must allow embedded OFL fonts, got %q", path, csp)
		}
		for _, directive := range kept {
			if !strings.Contains(csp, directive) {
				t.Fatalf("%s: directive %q missing from %q", path, directive, csp)
			}
		}
	}
}
