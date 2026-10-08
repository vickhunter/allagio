package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func TestDemoProductSitePassesReadiness(t *testing.T) {
	r := ginEngine()
	h := &HostedHandler{BaseURL: "http://127.0.0.1"}
	h.Mount(r)
	server := httptest.NewServer(r)
	defer server.Close()

	for _, path := range []string{"/demo", "/demo/pricing", "/demo/docs", "/demo/privacy", "/demo/terms"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: expected 200, got %d", path, resp.StatusCode)
		}
		if path == "/demo" && !strings.Contains(strings.ToLower(string(body)), "get started") {
			t.Fatalf("demo landing needs a get-started call to action: %s", body)
		}
	}
	landing, err := http.Get(server.URL + "/demo?via=dtr-probe")
	if err != nil {
		t.Fatal(err)
	}
	landing.Body.Close()
	if landing.Request.URL.Query().Get("via") != "dtr-probe" {
		t.Fatalf("ref param dropped: %s", landing.Request.URL)
	}
	plain, err := http.Get(server.URL + "/demo")
	if err != nil {
		t.Fatal(err)
	}
	plainBody, _ := io.ReadAll(plain.Body)
	plain.Body.Close()
	if strings.Contains(strings.ToLower(string(plainBody)), "checkout") {
		t.Fatal("checkout placeholder must wait until a test provider exists")
	}

	engine := &services.ReadinessEngine{Client: server.Client(), PublicBaseURL: server.URL}
	result := engine.Assess(context.Background(), services.ReadinessProduct{
		Name:       "Offer Rails demo",
		WebsiteURL: server.URL + "/demo",
		PricingURL: server.URL + "/demo/pricing",
		DocsURL:    server.URL + "/demo/docs",
	})
	if result.Status != "ready" {
		t.Fatalf("demo must pass readiness, got %s %+v", result.Status, result.Checks)
	}
}

func ginEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}
