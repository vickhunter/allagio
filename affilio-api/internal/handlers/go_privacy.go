package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/outbound"
	"github.com/gin-gonic/gin"
)

const (
	headerORClientHash = "X-OR-Client-Hash"
	headerORUserAgent  = "X-OR-User-Agent"
	headerORReferer    = "X-OR-Referer"
	goLinkSaltLabel    = "|go-link|"
)

// goRequest is the reader context copied before the raw address is scrubbed
// off the request. pluginHash is the hex SHA-256 the plugin already computed.
type goRequest struct {
	clientIP   string
	userAgent  string
	referer    string
	via        string
	apiKey     string
	pluginHash string
	pluginUA   string
	pluginRef  string
}

func captureGoRequest(c *gin.Context) goRequest {
	return goRequest{
		clientIP:   strings.TrimSpace(c.ClientIP()),
		userAgent:  c.Request.UserAgent(),
		referer:    c.Request.Referer(),
		via:        c.Query("via"),
		apiKey:     strings.TrimSpace(c.GetHeader("X-API-Key")),
		pluginHash: c.GetHeader(headerORClientHash),
		pluginUA:   c.GetHeader(headerORUserAgent),
		pluginRef:  c.GetHeader(headerORReferer),
	}
}

func stampGoResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex, nofollow")
}

// scrubGoRequest drops the raw address and any header that might carry it
// so later logging and error reports cannot print it. The handler already
// copied the values it needs.
func scrubGoRequest(r *http.Request) {
	if r == nil {
		return
	}
	r.RemoteAddr = ""
	for _, header := range []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"Forwarded",
		"True-Client-IP",
		"CF-Connecting-IP",
		"X-OR-Client-IP",
	} {
		r.Header.Del(header)
	}
}

func logGo(format string, args ...any) {
	log.Print(middleware.ScrubLoggedIPs(fmt.Sprintf(format, args...)))
}

func (h *RedirectsHandler) clock() time.Time {
	if h != nil && h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *RedirectsHandler) dailySalt() string {
	secret := ""
	if h != nil {
		secret = strings.TrimSpace(h.IPSaltSecret)
	}
	return GoLinkDailySalt(secret, h.clock())
}

// directReaderHash truncates the connection address and hashes it with the
// server salt for the current UTC day. An unparseable address returns empty.
func (h *RedirectsHandler) directReaderHash(ip string) string {
	truncated := TruncateReaderIP(ip)
	if truncated == "" {
		return ""
	}
	return GoLinkClientHash(truncated, h.dailySalt())
}

// PurgeClickSignals clears ip_hash, user_agent, and referrer on clicks older
// than 30 days. It runs once immediately and then every hour until ctx ends.
func PurgeClickSignals(ctx context.Context, db *database.DB) {
	purge := func() {
		cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
		if err := db.PurgeClickSignals(ctx, cutoff); err != nil && ctx.Err() == nil {
			logGo("click signal purge failed err=%v", err)
		}
		if err := db.PruneClientAddressHashes(ctx); err != nil && ctx.Err() == nil {
			logGo("client address hash purge failed err=%v", err)
		}
	}
	purge()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}

func (h *RedirectsHandler) rateLimitKey(ip string) string {
	if hashed := h.directReaderHash(ip); hashed != "" {
		return hashed
	}
	return "unknown"
}

// GoLinkDailySalt is the lowercase hex SHA-256 of secret|go-link|YYYY-MM-DD
// in UTC. It changes at 00:00 UTC.
func GoLinkDailySalt(secret string, now time.Time) string {
	day := now.UTC().Format("2006-01-02")
	sum := sha256.Sum256([]byte(secret + goLinkSaltLabel + day))
	return hex.EncodeToString(sum[:])
}

// GoLinkClientHash is the lowercase hex HMAC-SHA256 of the truncated address,
// keyed by the daily salt.
func GoLinkClientHash(truncated, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	_, _ = mac.Write([]byte(truncated))
	return hex.EncodeToString(mac.Sum(nil))
}

type landingProbe struct {
	ok bool
	at time.Time
}

var (
	landingCache     sync.Map
	landingReachable = probeLanding
	// landingResolver is the public-address dialer for offer landing probes.
	// Tests that serve a page on 127.0.0.1 set AllowLoopback.
	landingResolver outbound.Resolver
)

func init() {
	if testing.Testing() {
		landingReachable = func(string) bool { return true }
	}
}

func probeLanding(raw string) bool {
	if cached, ok := landingCache.Load(raw); ok {
		entry := cached.(landingProbe)
		if time.Since(entry.at) < 10*time.Minute {
			return entry.ok
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	resp, err := landingResolver.Do(ctx, http.MethodHead, raw, nil, outbound.Options{
		Timeout:    1500 * time.Millisecond,
		NoRedirect: true,
	})
	ok := false
	if err == nil {
		ok = landingUp(resp.StatusCode, nil)
		resp.Body.Close()
	}
	landingCache.Store(raw, landingProbe{ok: ok, at: time.Now()})
	return ok
}

// landingUp treats 403 and 405 as up. A network error, 404, 410, or 5xx is down.
func landingUp(status int, err error) bool {
	if err != nil {
		return false
	}
	if status == http.StatusNotFound || status == http.StatusGone || status >= 500 {
		return false
	}
	return true
}

// TruncateReaderIP keeps the IPv4 /24 or the IPv6 /48. IPv4 is dotted decimal
// with the last octet zero. IPv6 is lowercase RFC 5952 form. IPv4-mapped IPv6
// is truncated as IPv4.
func TruncateReaderIP(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String()
	}
	v6 := parsed.To16()
	if v6 == nil {
		return ""
	}
	return v6.Mask(net.CIDRMask(48, 128)).String()
}

// clientHashHeader accepts the plugin's finished hash. Anything that is not
// 64 hex characters is ignored so a raw address cannot be stored in its place.
func clientHashHeader(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if len(raw) != 64 {
		return ""
	}
	for _, r := range raw {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return raw
}
