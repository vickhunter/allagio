package handlers

import (
	"net/http"
	"net/http/httptest"
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
	if api.Code != http.StatusNotFound || strings.Contains(api.Body.String(), "That page is not on Offer Rails.") {
		t.Fatalf("api miss %d %s", api.Code, api.Body.String())
	}
}
