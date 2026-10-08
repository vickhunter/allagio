package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

func TestServeDemoForE2E(t *testing.T) {
	if os.Getenv("DEMO_E2E") != "1" {
		t.Skip()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := ginEngine()
	h := &HostedHandler{BaseURL: "http://" + ln.Addr().String()}
	h.Mount(r)
	srv := &http.Server{Handler: r}
	go func() { _ = srv.Serve(ln) }()
	if file := os.Getenv("DEMO_E2E_ADDR_FILE"); file != "" {
		if err = os.WriteFile(file, []byte(ln.Addr().String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Fprintf(os.Stdout, "DEMO_READY %s\n", ln.Addr().String())
	_ = os.Stdout.Sync()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = srv.Close()
}

func TestDemoPagesAreReadableAndKeepReferralParams(t *testing.T) {
	r := ginEngine()
	h := &HostedHandler{BaseURL: "http://127.0.0.1"}
	h.Mount(r)
	server := httptest.NewServer(r)
	defer server.Close()

	head, err := http.Head(server.URL + "/demo")
	if err != nil {
		t.Fatal(err)
	}
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Fatalf("HEAD /demo: expected 200, got %d", head.StatusCode)
	}

	titles := map[string]string{}
	pages := []struct{ path, title string }{
		{"/demo", "Demo landing page builder"},
		{"/demo/start", "Get started"},
		{"/demo/pricing", "Pricing"},
		{"/demo/docs", "Documentation"},
		{"/demo/privacy", "Privacy"},
		{"/demo/terms", "Terms"},
	}
	for _, page := range pages {
		resp, err := http.Get(server.URL + page.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		html := string(body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", page.path, resp.StatusCode)
		}
		if !strings.Contains(html, `<html lang="en">`) {
			t.Fatalf("%s missing lang: %s", page.path, html)
		}
		if !strings.Contains(html, `<meta name="viewport" content="width=device-width, initial-scale=1">`) {
			t.Fatalf("%s missing viewport", page.path)
		}
		start := strings.Index(html, "<title>")
		end := strings.Index(html, "</title>")
		if start < 0 || end < start {
			t.Fatalf("%s missing title", page.path)
		}
		title := html[start+len("<title>") : end]
		if !strings.Contains(title, page.title) || strings.TrimSpace(title) == "" {
			t.Fatalf("%s title %q", page.path, title)
		}
		if prev, ok := titles[title]; ok {
			t.Fatalf("title %q shared by %s and %s", title, prev, page.path)
		}
		titles[title] = page.path
		if !strings.Contains(html, "overflow-wrap:anywhere") && !strings.Contains(html, "overflow-wrap: anywhere") {
			t.Fatalf("%s must wrap long lines", page.path)
		}
		if !strings.Contains(html, "Demo landing page builder") {
			t.Fatalf("%s hero missing demo landing page builder: %s", page.path, html)
		}
		if !strings.Contains(html, `>DEMO<`) {
			t.Fatalf("%s missing DEMO label", page.path)
		}
		if !strings.Contains(html, `<meta name="description" content="`) || !strings.Contains(html, "Demo landing page builder") {
			t.Fatalf("%s missing demo meta description", page.path)
		}
		for _, banned := range []string{"trusted by", "testimonial", "$19", "sk_live"} {
			if strings.Contains(strings.ToLower(html), banned) {
				t.Fatalf("%s contains %q", page.path, banned)
			}
		}
		if page.path != "/demo" && !strings.Contains(html, `href="/demo"`) && !strings.Contains(html, `href="/demo?`) {
			t.Fatalf("%s must link back to /demo", page.path)
		}
	}
	start, err := http.Get(server.URL + "/demo/start")
	if err != nil {
		t.Fatal(err)
	}
	startBody, _ := io.ReadAll(start.Body)
	start.Body.Close()
	if !strings.Contains(string(startBody), `href="/auth/login"`) || !strings.Contains(string(startBody), "Sign in") || !strings.Contains(string(startBody), "Register") {
		t.Fatalf("start page must link onward to sign-in and register: %s", startBody)
	}

	for _, probe := range []struct{ page, key string }{{"/demo", "ref"}, {"/demo", "via"}, {"/demo/pricing", "ref"}} {
		resp, err := http.Get(server.URL + probe.page + "?" + probe.key + "=test")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), "/demo/start?"+probe.key+"=test") && !strings.Contains(string(body), probe.key+"=test") {
			t.Fatalf("%s ?%s=test did not survive onto a start link: %s", probe.page, probe.key, body)
		}
		if !strings.Contains(string(body), `href="/demo/start?`+probe.key+`=test"`) && !strings.Contains(string(body), probe.key+`=test"`) {
			t.Fatalf("%s start link dropped %s: %s", probe.page, probe.key, body)
		}
	}

	xss, err := http.Get(server.URL + "/demo?ref=" + "%22%3E%3Cscript%3Ealert(1)%3C/script%3E&via=bad%20value")
	if err != nil {
		t.Fatal(err)
	}
	xssBody, _ := io.ReadAll(xss.Body)
	xss.Body.Close()
	if strings.Contains(string(xssBody), "<script>alert") || strings.Contains(string(xssBody), "bad value") || strings.Contains(string(xssBody), "bad+value") {
		t.Fatalf("referral params were reflected: %s", xssBody)
	}
	long := strings.Repeat("a", 80)
	capped, err := http.Get(server.URL + "/demo?ref=" + long)
	if err != nil {
		t.Fatal(err)
	}
	cappedBody, _ := io.ReadAll(capped.Body)
	capped.Body.Close()
	if strings.Contains(string(cappedBody), long) {
		t.Fatal("over-long referral param was reflected")
	}
}
