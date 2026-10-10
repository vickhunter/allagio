package handlers

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestGoLinkHashTruncatesAndRotates(t *testing.T) {
	if TruncateReaderIP("203.0.113.60") != "203.0.113.0" || TruncateReaderIP("203.0.113.1") != TruncateReaderIP("203.0.113.255") {
		t.Fatalf("ipv4 /24: %s %s", TruncateReaderIP("203.0.113.60"), TruncateReaderIP("203.0.113.255"))
	}
	if TruncateReaderIP("203.0.114.1") == "203.0.113.0" {
		t.Fatal("a different ipv4 /24 collapsed")
	}
	if TruncateReaderIP("::ffff:203.0.113.9") != "203.0.113.0" {
		t.Fatalf("mapped ipv4: %s", TruncateReaderIP("::ffff:203.0.113.9"))
	}
	if TruncateReaderIP("2001:db8:abcd:1234::1") != "2001:db8:abcd::" || TruncateReaderIP("2001:db8:abcd:ffff::2") != "2001:db8:abcd::" {
		t.Fatalf("ipv6 /48: %s %s", TruncateReaderIP("2001:db8:abcd:1234::1"), TruncateReaderIP("2001:db8:abcd:ffff::2"))
	}
	if TruncateReaderIP("2001:db8:abce::1") == "2001:db8:abcd::" {
		t.Fatal("a different ipv6 /48 collapsed")
	}
	day := time.Date(2026, 10, 10, 23, 30, 0, 0, time.UTC)
	next := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	salt := GoLinkDailySalt("secret", day)
	rotated := GoLinkDailySalt("secret", next)
	if salt == rotated || len(salt) != 64 {
		t.Fatalf("daily salt did not rotate: %s %s", salt, rotated)
	}
	same := GoLinkClientHash(TruncateReaderIP("203.0.113.9"), salt)
	neighbor := GoLinkClientHash(TruncateReaderIP("203.0.113.200"), salt)
	tomorrow := GoLinkClientHash(TruncateReaderIP("203.0.113.9"), rotated)
	if same != neighbor || same == tomorrow || len(same) != 64 {
		t.Fatalf("client hash: same %s neighbor %s tomorrow %s", same, neighbor, tomorrow)
	}
	if clientHashHeader(strings.ToUpper(same)) != same || clientHashHeader("203.0.113.9") != "" || clientHashHeader(same+"aa") != "" {
		t.Fatal("plugin hash header was not limited to 64 hex characters")
	}
}

func TestGoLinkAccessLogScrubsIPs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/go/00000000-0000-4000-8000-000000000001?ref=1", nil)
	req.RemoteAddr = "203.0.113.77:1000"
	req.Header.Set("X-Forwarded-For", "198.51.100.44")
	line := middleware.AccessLogFormatter(gin.LogFormatterParams{
		TimeStamp:    time.Date(2026, 10, 10, 15, 4, 5, 0, time.UTC),
		StatusCode:   http.StatusFound,
		Method:       http.MethodGet,
		Request:      req,
		Latency:      time.Millisecond,
		ErrorMessage: "lookup failed from 203.0.113.77 and 2001:db8:abcd::1",
		ClientIP:     "203.0.113.77",
	})
	for _, secret := range []string{"203.0.113.77", "203.0.113.0", "198.51.100.44", "2001:db8:abcd::1"} {
		if strings.Contains(line, secret) {
			t.Fatalf("access log kept %s: %s", secret, line)
		}
	}
	if !strings.Contains(line, "302") || !strings.Contains(line, "/v1/go/") || strings.Contains(line, "ref=1") {
		t.Fatalf("access log shape: %s", line)
	}
}

func TestGoLinkStoresNoRawIP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "privacy-site-key", "privacy.example", "Privacy")
	_, offerID := seedListedOffer(t, db, "privacy-maker-key", "privacy-maker@example.com", "Privacy Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://offer.example/buy', status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	id := insertPlacement(t, db, siteID, offerID, "https://privacy.example/the-post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'privacy-place',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='placed', plugin_wait_at=$2, plugin_failed_reason='', failed_action='', restore_unconfirmed=false,
campaign_id=$3, expires_at=$4, post_url='https://privacy.example/the-post'
WHERE id=$1`, id, now, campaignID, now.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	prevLog := log.Writer()
	prevGin := gin.DefaultWriter
	log.SetOutput(&logs)
	gin.DefaultWriter = &logs
	t.Cleanup(func() {
		log.SetOutput(prevLog)
		gin.DefaultWriter = prevGin
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	r.Use(gin.LoggerWithFormatter(middleware.AccessLogFormatter), gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		log.Print(middleware.ScrubLoggedIPs("request panic recovered"))
		c.AbortWithStatus(500)
	}))
	fixed := time.Date(2026, 10, 10, 15, 0, 0, 0, time.UTC)
	redirects := &RedirectsHandler{DB: db, IPSaltSecret: "privacy-salt", now: func() time.Time { return fixed }}
	r.GET("/v1/go/:code", redirects.Go)
	r.HEAD("/v1/go/:code", redirects.Go)

	const (
		rawIP      = "203.0.113.77"
		samePrefix = "203.0.113.10"
		forwarded  = "198.51.100.44"
		truncated  = "203.0.113.0"
	)
	hit := func(remote, agent string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/go/"+id, nil)
		req.RemoteAddr = remote + ":1000"
		req.Header.Set("User-Agent", agent)
		req.Header.Set("X-Forwarded-For", forwarded)
		req.Header.Set("X-OR-Client-IP", rawIP)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	first := hit(rawIP, "Mozilla/5.0 reader")
	if first.Code != http.StatusFound || first.Header().Get("Cache-Control") != "no-store" || first.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("first hit: %d cache %q robots %q", first.Code, first.Header().Get("Cache-Control"), first.Header().Get("X-Robots-Tag"))
	}
	second := hit(samePrefix, "Mozilla/5.0 reader")
	if second.Code != http.StatusFound {
		t.Fatalf("same /24: %d", second.Code)
	}
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("same /24 and user agent recorded %d clicks", n)
	}
	var ipHash, stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(ip_hash,''), COALESCE(ip_hash,'') || ' ' || COALESCE(user_agent,'') || ' ' || COALESCE(referrer,'') || ' ' || id::text || ' ' || site_id::text || ' ' || COALESCE(campaign_id::text,'') || ' ' || COALESCE(placement_id::text,'') FROM clicks WHERE placement_id=$1`, id).Scan(&ipHash, &stored); err != nil {
		t.Fatal(err)
	}
	want := redirects.directReaderHash(rawIP)
	if ipHash != want || ipHash != redirects.directReaderHash(samePrefix) || strings.Contains(ipHash, ".") {
		t.Fatalf("stored hash %s want %s", ipHash, want)
	}
	for _, secret := range []string{rawIP, samePrefix, forwarded, truncated} {
		if strings.Contains(stored, secret) || strings.Contains(logs.String(), secret) {
			t.Fatalf("raw address %s leaked stored=%q logs=%q", secret, stored, logs.String())
		}
	}

	missing := httptest.NewRequest(http.MethodGet, "/v1/go/not-a-placement", nil)
	missing.RemoteAddr = rawIP + ":1000"
	missing.Header.Set("X-Forwarded-For", forwarded)
	missed := httptest.NewRecorder()
	r.ServeHTTP(missed, missing)
	if missed.Code != http.StatusNotFound || missed.Header().Get("Cache-Control") != "no-store" || missed.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("unknown code: %d cache %q robots %q", missed.Code, missed.Header().Get("Cache-Control"), missed.Header().Get("X-Robots-Tag"))
	}
	if strings.Contains(logs.String(), rawIP) || strings.Contains(logs.String(), forwarded) {
		t.Fatalf("unknown code logged an address: %s", logs.String())
	}
}
