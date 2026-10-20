package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthReportsCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("RENDER_GIT_COMMIT", "abc123def")
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "rail-ignored")

	r := gin.New()
	r.GET("/health", healthHandler(true))
	r.GET("/v1/health", healthHandler(false))

	for _, path := range []string{"/health", "/v1/health"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["status"] != "ok" || body["site_installation_registration"] != true || body["commit"] != "abc123def" {
			t.Fatalf("%s body: %s", path, rec.Body.String())
		}
		wantNative := path == "/health"
		if body["native_distribution"] != wantNative {
			t.Fatalf("%s native_distribution: %v", path, body["native_distribution"])
		}
	}

	t.Setenv("RENDER_GIT_COMMIT", "")
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["commit"] != "unknown" {
		t.Fatalf("fallback commit: %v", body["commit"])
	}
}

func TestDockerfilesBakeCommit(t *testing.T) {
	for _, path := range []string{"../../Dockerfile", "../../../Dockerfile.mvp"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "ARG GIT_COMMIT=unknown") || !strings.Contains(text, "-X affilio-api/internal/version.commit=${GIT_COMMIT}") {
			t.Fatalf("%s does not inject the commit ldflag", path)
		}
	}
}
