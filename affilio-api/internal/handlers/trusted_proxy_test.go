package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Real visitor IPs forwarded by a trusted proxy must land in separate rate-limit
// buckets and produce different click IP hashes. The same headers from a remote
// that is not a trusted proxy must be ignored.
func TestTrustedProxyClientIPRateLimitAndHash(t *testing.T) {
	gin.SetMode(gin.TestMode)

	trusted := gin.New()
	if err := middleware.ConfigureTrustedProxies(trusted, "192.0.2.1"); err != nil {
		t.Fatalf("configure trusted proxies: %v", err)
	}
	limiter := NewIPRateLimiter(1, time.Hour)
	trusted.GET("/ip", func(c *gin.Context) {
		ip := c.ClientIP()
		c.JSON(http.StatusOK, gin.H{
			"ip":      ip,
			"hash":    hashIP(ip),
			"allowed": limiter.Allow(ip),
		})
	})

	first := proxyIPRequest(trusted, "192.0.2.1:1234", "203.0.113.10", "")
	second := proxyIPRequest(trusted, "192.0.2.1:1234", "203.0.113.11", "")
	if first["allowed"] != true || second["allowed"] != true {
		t.Fatalf("forwarded clients must get separate rate-limit buckets, got %#v and %#v", first, second)
	}
	if first["ip"] != "203.0.113.10" || second["ip"] != "203.0.113.11" {
		t.Fatalf("expected forwarded client IPs, got %#v and %#v", first["ip"], second["ip"])
	}
	if first["hash"] == second["hash"] || first["hash"] != hashIP("203.0.113.10") || second["hash"] != hashIP("203.0.113.11") {
		t.Fatalf("expected distinct visitor IP hashes, got %#v and %#v", first["hash"], second["hash"])
	}
	repeat := proxyIPRequest(trusted, "192.0.2.1:1234", "203.0.113.10", "")
	if repeat["allowed"] != false {
		t.Fatalf("repeat of the same forwarded client must share its bucket, got %#v", repeat)
	}

	untrusted := gin.New()
	if err := middleware.ConfigureTrustedProxies(untrusted, "192.0.2.1"); err != nil {
		t.Fatalf("configure trusted proxies: %v", err)
	}
	spoofLimiter := NewIPRateLimiter(1, time.Hour)
	untrusted.GET("/ip", func(c *gin.Context) {
		ip := c.ClientIP()
		c.JSON(http.StatusOK, gin.H{
			"ip":      ip,
			"hash":    hashIP(ip),
			"allowed": spoofLimiter.Allow(ip),
		})
	})
	spoofed := proxyIPRequest(untrusted, "198.51.100.8:4321", "203.0.113.10", "203.0.113.99")
	spoofedAgain := proxyIPRequest(untrusted, "198.51.100.8:4321", "198.51.100.77", "203.0.113.50")
	if spoofed["ip"] != "198.51.100.8" || spoofedAgain["ip"] != "198.51.100.8" {
		t.Fatalf("untrusted remote must ignore forwarded headers, got %#v and %#v", spoofed["ip"], spoofedAgain["ip"])
	}
	if spoofed["hash"] != hashIP("198.51.100.8") || spoofed["hash"] != spoofedAgain["hash"] {
		t.Fatalf("spoofed headers must not change the IP hash, got %#v and %#v", spoofed["hash"], spoofedAgain["hash"])
	}
	if spoofed["allowed"] != true || spoofedAgain["allowed"] != false {
		t.Fatalf("spoofed clients must share the remote IP bucket, got %#v and %#v", spoofed, spoofedAgain)
	}

	closed := gin.New()
	if err := middleware.ConfigureTrustedProxies(closed, ""); err != nil {
		t.Fatalf("empty trusted proxies: %v", err)
	}
	closed.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": c.ClientIP()})
	})
	direct := proxyIPRequest(closed, "198.51.100.8:4321", "203.0.113.10", "203.0.113.99")
	if direct["ip"] != "198.51.100.8" {
		t.Fatalf("empty TRUSTED_PROXIES must ignore forwarded headers, got %#v", direct["ip"])
	}
}

func proxyIPRequest(r *gin.Engine, remoteAddr, forwardedFor, realIP string) map[string]interface{} {
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	out := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}
