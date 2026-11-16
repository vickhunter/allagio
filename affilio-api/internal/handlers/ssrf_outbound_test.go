package handlers

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/outbound"
	"affilio-api/internal/preview"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

// TestOutboundSSRFTargetsFail is the shared table for the three outbound
// paths: preview, federation, and the owner webhook poster. Each target is
// refused before a private, loopback, or metadata address is dialed.
func TestOutboundSSRFTargetsFail(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.2.3.4/secret", http.StatusFound)
	}))
	t.Cleanup(page.Close)

	targets := []struct {
		name     string
		raw      string
		redirect bool
	}{
		{"127.0.0.1", "http://127.0.0.1/secret", false},
		{"::1", "http://[::1]/secret", false},
		{"169.254.169.254", "http://169.254.169.254/latest/meta-data", false},
		{"10.x", "http://10.2.3.4/secret", false},
		{"redirect", "http://public.example/r", true},
	}
	paths := []string{"preview", "federation", "webhook", "article"}

	for _, path := range paths {
		for _, target := range targets {
			t.Run(path+"/"+target.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				var dialed []string
				dial := dialRecorder(t, page, &dialed)
				err := runOutboundPath(t, ctx, path, target.raw, target.redirect, dial)
				if err == nil {
					t.Fatal("request was accepted")
				}
				for _, address := range dialed {
					host, _, splitErr := net.SplitHostPort(address)
					if splitErr == nil && outbound.BlockedIP(net.ParseIP(host)) {
						t.Fatalf("dialed blocked address %s (all %v)", address, dialed)
					}
				}
				if target.redirect {
					if len(dialed) != 1 || !strings.HasPrefix(dialed[0], "203.0.113.10:") {
						t.Fatalf("dialed %v, want only the pinned public address", dialed)
					}
				} else if len(dialed) != 0 {
					t.Fatalf("literal target dialed %v", dialed)
				}
			})
		}
	}

	t.Run("deliver/10.x", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := deliverSignedWebhook(ctx, "https://10.2.3.4/secret", "secret", []byte(`{"type":"deal.waiting"}`))
		if !errors.Is(err, outbound.ErrBlocked) {
			t.Fatalf("deliverSignedWebhook: %v", err)
		}
	})
	t.Run("save/10.x", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		h := &HostedHandler{}
		r := gin.New()
		r.POST("/owner/notify", func(c *gin.Context) {
			c.Set("owner", &database.Owner{})
			h.SaveNotifyForm(c)
		})
		form := url.Values{"url": {"https://10.2.3.4/secret"}}
		req := httptest.NewRequest(http.MethodPost, "/owner/notify", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("SaveNotifyForm status %d body %s", rec.Code, rec.Body.String())
		}
	})
}

func runOutboundPath(t *testing.T, ctx context.Context, path, raw string, redirect bool, dial func(context.Context, string, string) (net.Conn, error)) error {
	t.Helper()
	switch path {
	case "preview":
		f := preview.Fetcher{DialContext: dial}
		if redirect {
			f.LookupIP = publicLookup
		}
		_, err := f.Fetch(ctx, raw)
		if err == nil {
			return nil
		}
		if redirect && !errors.Is(err, preview.ErrBlocked) {
			t.Fatalf("preview redirect: %v", err)
		}
		if !redirect && !errors.Is(err, preview.ErrBlocked) {
			t.Fatalf("preview: %v", err)
		}
		return err
	case "federation":
		engine := &services.ReadinessEngine{DialContext: dial}
		if redirect {
			engine.LookupIPAddr = publicLookupAddr
		}
		prev := federationEngine
		federationEngine = func() *services.ReadinessEngine { return engine }
		t.Cleanup(func() { federationEngine = prev })
		_, status, ok := defaultFedFetch(ctx, http.MethodGet, raw, nil)
		if ok || status != 0 {
			t.Fatalf("federation ok=%v status=%d", ok, status)
		}
		return errors.New("federation refused")
	case "webhook":
		guard := outbound.Resolver{DialContext: dial}
		if redirect {
			guard.LookupIP = publicLookup
		}
		resp, err := sendOwnerWebhook(ctx, guard, raw, []byte(`{"type":"deal.waiting"}`), nil)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if redirect && err == nil && resp != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return errors.New("webhook redirect")
		}
		if err == nil {
			return nil
		}
		if !errors.Is(err, outbound.ErrBlocked) {
			t.Fatalf("webhook: %v", err)
		}
		return err
	case "article":
		prev := articleResolver
		articleResolver = outbound.Resolver{DialContext: dial}
		if redirect {
			articleResolver.LookupIP = publicLookup
		}
		t.Cleanup(func() { articleResolver = prev })
		_, err := fetchArticleText(ctx, raw)
		if err == nil {
			return nil
		}
		return err
	default:
		t.Fatalf("unknown path %s", path)
		return errors.New("unknown path")
	}
}

func publicLookup(_ context.Context, host string) ([]net.IP, error) {
	if host == "public.example" {
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	return nil, errors.New("nxdomain")
}

func publicLookupAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	ips, err := publicLookup(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]net.IPAddr, len(ips))
	for i, ip := range ips {
		out[i] = net.IPAddr{IP: ip}
	}
	return out, nil
}

func dialRecorder(t *testing.T, page *httptest.Server, dialed *[]string) func(context.Context, string, string) (net.Conn, error) {
	t.Helper()
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		*dialed = append(*dialed, address)
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if outbound.BlockedIP(net.ParseIP(host)) {
			return nil, errors.New("dialed blocked " + address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, page.Listener.Addr().String())
	}
}

func TestNotifyURLPolicy(t *testing.T) {
	prev := webhookGuard
	t.Cleanup(func() { webhookGuard = prev })
	webhookGuard.LookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		switch host {
		case "localhost":
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		case "example.com":
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		default:
			return nil, errors.New("nxdomain")
		}
	}
	ctx := context.Background()
	rejected := []string{
		"http://127.0.0.1/hook",
		"http://127.0.0.1:8080/hook",
		"http://localhost/hook",
		"http://localhost:9/hook",
		"http://[::1]/hook",
		"https://127.0.0.1/hook",
		"https://[::1]/hook",
		"https://10.2.3.4/hook",
		"https://169.254.169.254/latest/meta-data",
		"http://10.2.3.4/hook",
		"http://127.0.0.1.evil.com/hook",
		"http://localhost.evil.com/hook",
		"https://user:pass@example.com/hook",
		"file:///etc/passwd",
	}
	for _, raw := range rejected {
		if err := notifyTargetAllowed(ctx, raw, false); err == nil {
			t.Fatalf("production accepted %s", raw)
		}
	}
	if err := notifyTargetAllowed(ctx, "https://example.com/hook", false); err != nil {
		t.Fatalf("public https: %v", err)
	}
	if err := notifyTargetAllowed(ctx, "", false); err != nil {
		t.Fatal(err)
	}
	if validDomain("127.0.0.1") || validDomain("127.0.0.1:8080") || validDomain("localhost") || validDomain("localhost:9") || validDomain("blog.localhost") {
		t.Fatal("prod accepted a loopback domain")
	}
	if !validDomain("publisher.example") {
		t.Fatal("public domain rejected")
	}
	for _, raw := range []string{"http://127.0.0.1:8080/hook", "http://localhost/hook"} {
		if err := notifyTargetAllowed(ctx, raw, true); err != nil {
			t.Fatalf("test flag rejected %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"http://10.2.3.4/hook", "https://169.254.169.254/latest", "https://[::1]/hook", "http://[::1]/hook"} {
		if err := notifyTargetAllowed(ctx, raw, true); err == nil {
			t.Fatalf("test flag accepted %s", raw)
		}
	}
}
