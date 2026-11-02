package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicPagesKeepReturnToOnThePage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)

	cases := []struct {
		name  string
		query string
		want  string
		evil  string
	}{
		{name: "valid relative", query: "return_to=/owner", want: "/owner"},
		{name: "absolute URL", query: "return_to=https://evil.example/phish", evil: "evil.example"},
		{name: "malformed", query: "return_to=%ZZ"},
		{name: "none", query: ""},
	}
	pages := []struct {
		path   string
		marker string
	}{
		{path: "/publishers/setup", marker: "Publisher setup"},
		{path: "/publishers/setup/", marker: "Publisher setup"},
		{path: "/you", marker: "Your limits"},
		{path: "/you/", marker: "Your limits"},
	}
	for _, page := range pages {
		for _, item := range cases {
			t.Run(page.path+" "+item.name, func(t *testing.T) {
				if got := publicReturnFrom(item.query); got != item.want {
					t.Fatalf("return_to = %q, want %q", got, item.want)
				}
				req := httptest.NewRequest(http.MethodGet, page.path, nil)
				req.URL.RawQuery = item.query
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d %s", rec.Code, rec.Body.String())
				}
				if loc := rec.Header().Get("Location"); loc != "" {
					t.Fatalf("redirected to %s", loc)
				}
				body := rec.Body.String()
				if !strings.Contains(body, page.marker) {
					t.Fatalf("page missing %q", page.marker)
				}
				if item.evil != "" && strings.Contains(body, item.evil) {
					t.Fatalf("absolute return_to was written into the page")
				}
			})
		}
	}
}

func TestLoginAcceptsPublicPageReturn(t *testing.T) {
	for _, raw := range []string{"/you", "/you/", "/publishers/setup", "/publishers/setup/", "/publishers/setup?source=cursor", "/you/?source=panel"} {
		if _, ok := loginReturnPathAllowed(raw); !ok {
			t.Fatalf("refused same-origin return %s", raw)
		}
	}
	for _, raw := range []string{"https://evil.example", "//evil.example", "/you/%ZZ", "/%ZZ", `/you/\evil`} {
		if _, ok := loginReturnPathAllowed(raw); ok {
			t.Fatalf("allowed unsafe return %s", raw)
		}
	}
}
