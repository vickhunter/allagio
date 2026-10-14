package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func TestHostedMCPRateLimitUsesTrustedClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL: "https://pilot.example",
		Limiter: NewIPRateLimiter(1, time.Hour),
	}
	trusted := gin.New()
	if err := middleware.ConfigureTrustedProxies(trusted, "192.0.2.1"); err != nil {
		t.Fatal(err)
	}
	h.Mount(trusted)

	first := mcpStatus(trusted, "192.0.2.1:1234", "203.0.113.10")
	other := mcpStatus(trusted, "192.0.2.1:1234", "203.0.113.11")
	repeat := mcpStatus(trusted, "192.0.2.1:1234", "203.0.113.10")
	if first == http.StatusTooManyRequests || other == http.StatusTooManyRequests {
		t.Fatalf("two forwarded clients must get separate buckets, got %d and %d", first, other)
	}
	if repeat != http.StatusTooManyRequests {
		t.Fatalf("repeat of the same forwarded client must be limited, got %d", repeat)
	}

	untrusted := gin.New()
	if err := middleware.ConfigureTrustedProxies(untrusted, "192.0.2.1"); err != nil {
		t.Fatal(err)
	}
	h.Limiter = NewIPRateLimiter(1, time.Hour)
	h.Mount(untrusted)
	spoofed := mcpStatus(untrusted, "198.51.100.8:4321", "203.0.113.10")
	spoofedAgain := mcpStatus(untrusted, "198.51.100.8:4321", "198.51.100.77")
	if spoofed == http.StatusTooManyRequests {
		t.Fatalf("first untrusted request must not be limited, got %d", spoofed)
	}
	if spoofedAgain != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Forwarded-For must share the remote IP bucket, got %d then %d", spoofed, spoofedAgain)
	}
}

func mcpStatus(r http.Handler, remoteAddr, forwardedFor string) int {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("X-Forwarded-For", forwardedFor)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}
