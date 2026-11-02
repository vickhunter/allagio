package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginAcceptsPublicPageReturn(t *testing.T) {
	for _, raw := range []string{"/you", "/you/", "/publishers/setup", "/publishers/setup/", "/publishers/setup?source=cursor", "/you/?source=panel"} {
		if _, ok := loginReturnPathAllowed(raw); !ok {
			t.Fatalf("refused same-origin return %s", raw)
		}
	}

	gin.SetMode(gin.TestMode)
	h := &HostedHandler{BaseURL: "http://127.0.0.1:8080"}
	r := gin.New()
	h.Mount(r)
	// These must be HTTP 400 from LoginStart. A looser allowlist (prefix match,
	// a dropped fragment check, or a dropped absolute-URL check) lets one through.
	refused := []string{
		"/publishers/setupx",
		"/you/evil",
		"/you#x",
		"//evil.com/you",
		"https://evil.com/you",
		"https://evil.com/you/",
	}
	for _, raw := range refused {
		if _, ok := loginReturnPathAllowed(raw); ok {
			t.Fatalf("allowed unsafe return %s", raw)
		}
		req := httptest.NewRequest(http.MethodGet, "/auth/login?return_to="+url.QueryEscape(raw), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid return path") {
			t.Fatalf("%s: %d %s", raw, rec.Code, rec.Body.String())
		}
	}
	// The %23 form is /you#x after the query is decoded.
	encoded := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	encoded.URL.RawQuery = "return_to=%2Fyou%23x"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, encoded)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid return path") {
		t.Fatalf("%%23 form: %d %s", rec.Code, rec.Body.String())
	}
	for _, raw := range []string{"https://evil.example", "//evil.example", "/you/%ZZ", "/%ZZ", `/you/\evil`} {
		if _, ok := loginReturnPathAllowed(raw); ok {
			t.Fatalf("allowed unsafe return %s", raw)
		}
	}
}
