package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testEngine() *ReadinessEngine {
	return &ReadinessEngine{
		AllowPrivateHosts: true,
		Client:            &http.Client{Timeout: 5 * time.Second},
	}
}

func readyProductSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
<h1>PromptForge</h1>
<a href="/signup">Sign up</a> <a href="/privacy">Privacy Policy</a>
<script src="https://js.stripe.com/v3/"></script>
</body></html>`))
	})
	mux.HandleFunc("/pricing", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Pro plan: $19/month</body></html>`))
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>" + longDocs() + "</body></html>"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func longDocs() string {
	out := ""
	for i := 0; i < 60; i++ {
		out += "PromptForge lets you version, test, and ship prompt templates from the CLI. "
	}
	return out
}

func findCheck(t *testing.T, checks []ReadinessCheck, key string) ReadinessCheck {
	t.Helper()
	for _, c := range checks {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("check %s not found in %v", key, checks)
	return ReadinessCheck{}
}

func TestReadinessAssessPassesGatesForHealthySite(t *testing.T) {
	site := readyProductSite(t)
	engine := testEngine()

	result := engine.Assess(context.Background(), ReadinessProduct{
		Name:            "PromptForge",
		WebsiteURL:      site.URL,
		PricingURL:      site.URL + "/pricing",
		DocsURL:         site.URL + "/docs",
		PaymentProvider: "stripe",
	})
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s: %+v", result.Status, result.Checks)
	}
	for _, key := range []string{"site_reachable", "checkout_present", "ref_param_survives", "pricing_page", "docs_present", "privacy_policy", "onboarding_flow"} {
		check := findCheck(t, result.Checks, key)
		if check.Status != "pass" {
			t.Fatalf("expected %s to pass, got %s (%s)", key, check.Status, check.Evidence)
		}
	}
	if result.Score < 90 {
		t.Fatalf("expected high score, got %d", result.Score)
	}
}

func TestReadinessFailsWhenRefParamStripped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.URL.RawQuery != "" {
			// Simulate an app that redirects to a canonical URL and drops the query.
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		w.Write([]byte(`<html><body>Sign up <script src="https://js.stripe.com/v3/"></script></body></html>`))
	})
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>home</body></html>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: server.URL})
	if result.Status != "failing" {
		t.Fatalf("expected failing, got %s", result.Status)
	}
	check := findCheck(t, result.Checks, "ref_param_survives")
	if check.Status != "fail" || check.Remediation == "" {
		t.Fatalf("expected ref_param_survives fail with remediation, got %+v", check)
	}
}

func TestReadinessFailsWithoutCheckout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Sign up for our tool</body></html>`))
	}))
	defer server.Close()

	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: server.URL})
	if result.Status != "failing" {
		t.Fatalf("expected failing, got %s", result.Status)
	}
	check := findCheck(t, result.Checks, "checkout_present")
	if check.Status != "fail" || check.Remediation == "" {
		t.Fatalf("expected checkout_present fail with remediation, got %+v", check)
	}
}

func TestReadinessUnreachableSite(t *testing.T) {
	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: "http://127.0.0.1:1"})
	if result.Status != "failing" {
		t.Fatalf("expected failing, got %s", result.Status)
	}
	check := findCheck(t, result.Checks, "site_reachable")
	if check.Status != "fail" {
		t.Fatalf("expected site_reachable fail, got %+v", check)
	}
}

func TestReadinessBlocksPrivateHostsByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("internal"))
	}))
	defer server.Close()

	engine := &ReadinessEngine{Client: &http.Client{Timeout: 5 * time.Second}}
	result := engine.Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: server.URL})
	check := findCheck(t, result.Checks, "site_reachable")
	if check.Status != "fail" {
		t.Fatalf("expected private host probe to fail, got %+v", check)
	}
}

func TestReadinessAIRubricContributesChecksAndTaxonomy(t *testing.T) {
	site := readyProductSite(t)
	claude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rubric := map[string]interface{}{
			"onboarding_clarity": 85,
			"docs_quality":       35,
			"value_prop_clarity": 90,
			"primary_category":   "software",
			"keywords":           []string{"prompt", "cli"},
			"tags":               []string{"developer-tools"},
			"summary":            "Solid landing page; documentation is thin.",
		}
		text, _ := json.Marshal(rubric)
		resp := map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(text)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer claude.Close()

	engine := testEngine()
	engine.APIKey = "test-key"
	engine.Model = "test-model"
	engine.APIURL = claude.URL

	result := engine.Assess(context.Background(), ReadinessProduct{
		Name:       "PromptForge",
		WebsiteURL: site.URL,
		PricingURL: site.URL + "/pricing",
		DocsURL:    site.URL + "/docs",
	})
	if result.Category != "software" || len(result.Keywords) != 2 {
		t.Fatalf("expected taxonomy extraction, got category=%q keywords=%v", result.Category, result.Keywords)
	}
	if result.AISummary == "" {
		t.Fatalf("expected ai summary")
	}
	docsCheck := findCheck(t, result.Checks, "docs_quality")
	if docsCheck.Status != "fail" || docsCheck.Kind != "ai" {
		t.Fatalf("expected ai docs_quality fail, got %+v", docsCheck)
	}
	// AI checks never gate: probes all pass, so the product is still ready.
	if result.Status != "ready" {
		t.Fatalf("expected ready despite low ai score, got %s", result.Status)
	}
}
