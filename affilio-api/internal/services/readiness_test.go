package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadinessPublicTransportRejectsPrivateDestinations(t *testing.T) {
	e := &ReadinessEngine{}
	client := e.httpClient()
	for _, raw := range []string{"http://127.0.0.1/secret", "http://[::1]/secret", "http://169.254.169.254/latest", "http://10.0.0.1/secret", "http://224.0.0.1/secret", "file:///etc/passwd", "https://user:password@example.com"} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if client.CheckRedirect(&http.Request{URL: u}, nil) == nil {
			t.Fatalf("redirect accepted: %s", raw)
		}
	}
	for _, address := range []string{"127.0.0.1:80", "[::1]:80", "169.254.169.254:80", "10.0.0.1:80"} {
		if conn, err := dialPublicProbe(context.Background(), "tcp", address); err == nil {
			conn.Close()
			t.Fatalf("private dial accepted: %s", address)
		}
	}
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("public probes must not bypass destination checks through an environment proxy")
	}
}

func TestPublicProbeAddressesBlocksSharedRanges(t *testing.T) {
	blocked := []string{
		"100.64.0.1",
		"100.127.255.254",
		"127.0.0.1",
		"127.255.255.255",
		"::1",
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"fc00::1",
		"fd00::1",
		"169.254.169.254",
		"169.254.0.1",
		"fe80::1",
		"224.0.0.1",
		"239.255.255.255",
		"ff00::1",
		"ff02::1",
		"0.0.0.0",
		"::",
		"::ffff:127.0.0.1",
		"::ffff:10.1.2.3",
		"::ffff:172.16.5.5",
		"::ffff:192.168.0.1",
		"::ffff:169.254.169.254",
		"::ffff:100.64.1.1",
		"::ffff:224.0.0.1",
		"::ffff:0.0.0.0",
	}
	for _, ip := range blocked {
		if _, err := publicProbeAddresses(context.Background(), ip); err == nil {
			t.Fatalf("%s was allowed", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "100.63.255.255", "100.128.0.1", "203.0.113.10", "::ffff:8.8.8.8", "2001:4860:4860::8888"} {
		got, err := publicProbeAddresses(context.Background(), ip)
		if err != nil || len(got) != 1 {
			t.Fatalf("public %s: %v %+v", ip, err, got)
		}
	}
	e := &ReadinessEngine{LookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("100.64.0.1")}}, nil
	}}
	if _, err := e.resolvePublic(context.Background(), "cgnat.example"); err == nil {
		t.Fatal("dns answer in 100.64.0.0/10 was allowed")
	}
	e.LookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	if _, err := e.resolvePublic(context.Background(), "mixed.example"); err == nil {
		t.Fatal("a public answer mixed with loopback was allowed")
	}
}

func TestFetchPublicPageRejectsRedirectToLoopback(t *testing.T) {
	var hits atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	secret := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	})}
	go func() { _ = secret.Serve(ln) }()
	t.Cleanup(func() { _ = secret.Close() })

	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+ln.Addr().String()+"/secret", http.StatusFound)
	}))
	t.Cleanup(page.Close)

	var dialed []string
	e := &ReadinessEngine{
		Client: &http.Client{Timeout: 2 * time.Second},
		LookupIPAddr: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if ip := net.ParseIP(host); ip != nil {
				return []net.IPAddr{{IP: ip}}, nil
			}
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
		},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			host, _, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			if blockedProbeIP(net.ParseIP(host)) {
				return nil, errors.New("dialed blocked " + address)
			}
			return (&net.Dialer{}).DialContext(ctx, network, page.Listener.Addr().String())
		},
	}
	if _, _, ok := e.FetchPublicPage(context.Background(), "http://public.example/start"); ok || hits.Load() != 0 {
		t.Fatalf("redirect to loopback was fetched: ok=%v hits=%d dialed=%v", ok, hits.Load(), dialed)
	}
	for _, address := range dialed {
		host, _, _ := net.SplitHostPort(address)
		if blockedProbeIP(net.ParseIP(host)) {
			t.Fatalf("dialed blocked address %s", address)
		}
	}
}

func TestFetchPublicPageRejectsDNSRebindToLoopback(t *testing.T) {
	var hits atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	secret := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	})}
	go func() { _ = secret.Serve(ln) }()
	t.Cleanup(func() { _ = secret.Close() })

	var lookups atomic.Int32
	var dialed []string
	e := &ReadinessEngine{
		Client: &http.Client{Timeout: 2 * time.Second},
		LookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			if lookups.Add(1) == 1 {
				return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
			}
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			host, _, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			if blockedProbeIP(net.ParseIP(host)) {
				return (&net.Dialer{}).DialContext(ctx, network, ln.Addr().String())
			}
			return nil, errors.New("unexpected dial " + address)
		},
	}
	raw := fmt.Sprintf("http://rebind.example:%d/page", ln.Addr().(*net.TCPAddr).Port)
	if _, _, ok := e.FetchPublicPage(context.Background(), raw); ok || hits.Load() != 0 || lookups.Load() < 2 {
		t.Fatalf("rebind ok=%v hits=%d lookups=%d dialed=%v", ok, hits.Load(), lookups.Load(), dialed)
	}
}

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
<a href="/signup">Sign up</a> <a href="/privacy">Privacy Policy</a> <a href="/terms">Terms</a>
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

func TestRefParamCheckCatchesCTAThatDropsTheParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/start">Get started</a> <a href="/pricing">Pricing</a> <a href="/signup">Sign up</a></body></html>`))
	}))
	defer server.Close()
	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: server.URL})
	check := findCheck(t, result.Checks, "ref_param_survives")
	if check.Status != "fail" || check.Remediation == "" {
		t.Fatalf("a get-started link that drops the referral parameter must fail, got %+v", check)
	}
}

func TestRefParamCheckAllowsCTAThatKeepsTheParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><a href="/start?via=dtr-probe">Get started</a> <a href="/pricing?via=dtr-probe">Pricing</a> <a href="/signup">Sign up</a></body></html>`)
	}))
	defer server.Close()
	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "App", WebsiteURL: server.URL})
	check := findCheck(t, result.Checks, "ref_param_survives")
	if check.Status != "pass" {
		t.Fatalf("a get-started link that keeps the referral parameter must pass, got %+v", check)
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

func agnosticSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := "/start"
		if r.URL.RawQuery != "" {
			start += "?" + r.URL.RawQuery
		}
		fmt.Fprintf(w, `<html><body>
<a href="%s">Get started</a>
<a href="/privacy">Privacy Policy</a>
<a href="/terms">Terms</a>
</body></html>`, start)
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

func TestReadinessCheckoutDoesNotRequireAKnownProvider(t *testing.T) {
	site := agnosticSite(t)
	result := testEngine().Assess(context.Background(), ReadinessProduct{
		Name:       "Northstar",
		WebsiteURL: site.URL,
		PricingURL: site.URL + "/pricing",
		DocsURL:    site.URL + "/docs",
	})
	checkout := findCheck(t, result.Checks, "checkout_present")
	if !checkout.Gate || checkout.Status != "pass" {
		t.Fatalf("a get-started link must pass checkout_present, got %+v", checkout)
	}
	if strings.Contains(strings.ToLower(checkout.Evidence+checkout.Remediation), "stripe") {
		t.Fatalf("passing without a known provider must not mention Stripe, got %+v", checkout)
	}
	warning := findCheck(t, result.Checks, "payment_provider")
	if warning.Gate || warning.Status != "warn" || !strings.Contains(strings.ToLower(warning.Evidence), "no known payment provider") {
		t.Fatalf("missing provider must be a non-gating warning, got %+v", warning)
	}
	for _, key := range []string{"site_reachable", "pricing_page", "docs_present", "privacy_policy", "terms"} {
		check := findCheck(t, result.Checks, key)
		if !check.Gate || check.Status != "pass" {
			t.Fatalf("%s must stay a passing gate, got %+v", key, check)
		}
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready without a known provider, got %s %+v", result.Status, result.Checks)
	}
}

func TestReadinessReportsKnownProviderWithoutLockIn(t *testing.T) {
	site := agnosticSite(t)
	result := testEngine().Assess(context.Background(), ReadinessProduct{
		Name:        "Northstar",
		WebsiteURL:  site.URL,
		PricingURL:  site.URL + "/pricing",
		DocsURL:     site.URL + "/docs",
		CheckoutURL: site.URL + "/checkout",
	})
	checkout := findCheck(t, result.Checks, "checkout_present")
	if checkout.Status != "pass" || !strings.Contains(strings.ToLower(checkout.Evidence), "checkout") {
		t.Fatalf("declared checkout URL must pass, got %+v", checkout)
	}
	marked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="https://buy.stripe.com/test">Buy</a><a href="/privacy">Privacy</a><a href="/terms">Terms</a></body></html>`))
	}))
	t.Cleanup(marked.Close)
	detected := testEngine().Assess(context.Background(), ReadinessProduct{Name: "Paid", WebsiteURL: marked.URL, PricingURL: marked.URL, DocsURL: marked.URL})
	found := findCheck(t, detected.Checks, "checkout_present")
	if found.Status != "pass" || !strings.Contains(strings.ToLower(found.Evidence), "stripe") {
		t.Fatalf("known marker must be reported, got %+v", found)
	}
	for _, check := range detected.Checks {
		if check.Key == "payment_provider" && check.Status == "warn" {
			t.Fatalf("a detected provider must not warn, got %+v", check)
		}
	}
}

func TestReadinessPageGatesFailClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="/start">Get started</a></body></html>`))
	}))
	defer server.Close()
	result := testEngine().Assess(context.Background(), ReadinessProduct{Name: "Thin", WebsiteURL: server.URL})
	if result.Status != "failing" {
		t.Fatalf("expected failing, got %s", result.Status)
	}
	for _, key := range []string{"pricing_page", "docs_present", "privacy_policy", "terms"} {
		check := findCheck(t, result.Checks, key)
		if !check.Gate || check.Status != "fail" || check.Remediation == "" {
			t.Fatalf("%s must stay a failing gate, got %+v", key, check)
		}
	}
	checkout := findCheck(t, result.Checks, "checkout_present")
	if strings.Contains(checkout.Remediation, "Lemon Squeezy") || strings.Contains(checkout.Remediation, "Polar") {
		t.Fatalf("remediation must not lock the product to one provider, got %s", checkout.Remediation)
	}
}

func TestReadinessAllowsOnlyTheConfiguredPublicOrigin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="/start">Get started</a><a href="/privacy">Privacy</a><a href="/terms">Terms</a></body></html>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	engine := &ReadinessEngine{Client: &http.Client{Timeout: 5 * time.Second}, PublicBaseURL: server.URL}
	result := engine.Assess(context.Background(), ReadinessProduct{Name: "Self", WebsiteURL: server.URL})
	check := findCheck(t, result.Checks, "site_reachable")
	if check.Status != "pass" {
		t.Fatalf("PUBLIC_BASE_URL must be probeable, got %+v", check)
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer other.Close()
	blocked := engine.Assess(context.Background(), ReadinessProduct{Name: "Other", WebsiteURL: other.URL})
	if findCheck(t, blocked.Checks, "site_reachable").Status != "fail" {
		t.Fatal("a different loopback origin must stay blocked")
	}
}
