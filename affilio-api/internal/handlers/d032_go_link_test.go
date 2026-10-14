package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
)

func TestPlacementGoLink(t *testing.T) {
	if !strings.Contains(database.PlacementRedirectSQL, "placement_counts_as_live") {
		t.Fatal("placement go link does not use placement_counts_as_live")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	fixed := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	redirects := &RedirectsHandler{
		DB: db, HomeURL: "https://offerrails.example/", IPSaltSecret: "test-salt",
		now: func() time.Time { return fixed },
	}
	r.GET("/v1/go/:code", redirects.Go)
	r.HEAD("/v1/go/:code", redirects.Go)

	owner, _ := testOwnerSession(t, h, "go-owner", "go-owner@example.com")
	siteID := createTestSite(t, db, "go-site-key", "go.example", "Go")
	makerID, offerID := seedListedOffer(t, db, "go-maker-key", "go-maker@example.com", "Go Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://offer.example/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	otherSite := createTestSite(t, db, "go-other-key", "other.example", "Other")
	_ = otherSite
	siteKey := map[string]string{"X-API-Key": "go-site-key"}
	id := insertPlacement(t, db, siteID, offerID, "https://go.example/the-post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'go-place',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	plain := "https://offer.example/buy"
	postURL := "https://go.example/the-post"
	siteHomeURL := "https://go.example/"
	current := time.Now().UTC()

	makeLive := func() {
		t.Helper()
		if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed', landing_url='https://offer.example/buy' WHERE id=$1`, offerID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE campaigns SET status='active' WHERE id=$1`, campaignID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE approval_policies SET kill_switch=false WHERE site_id=$1 OR offer_id=$2`, siteID, offerID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='placed', plugin_wait_at=$2, plugin_failed_reason='', failed_action='', restore_unconfirmed=false,
campaign_id=$3, expires_at=$4, post_url=$5
WHERE id=$1`, id, current, campaignID, current.Add(48*time.Hour), postURL); err != nil {
			t.Fatal(err)
		}
	}
	clicks := func() int {
		t.Helper()
		var n int
		if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	goHit := func(label, path, remote, agent, referer string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		req.Header.Set("User-Agent", agent)
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: status %d body %s", label, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("%s: cache %q robots %q", label, rec.Header().Get("Cache-Control"), rec.Header().Get("X-Robots-Tag"))
		}
		return rec
	}
	expect := func(label, want string, hit bool) {
		t.Helper()
		before := clicks()
		rec := goHit(label, "/v1/go/"+id, "192.0.2.1:1000", testBrowserUA, "https://go.example/the-post", nil)
		if rec.Header().Get("Location") != want {
			t.Fatalf("%s: location %s", label, rec.Header().Get("Location"))
		}
		if parsed, err := url.Parse(rec.Header().Get("Location")); err != nil || (want == plain && parsed.RawQuery != "") {
			t.Fatalf("%s: tracking query on plain url %s", label, rec.Header().Get("Location"))
		}
		after := clicks()
		if hit && after != before+1 {
			t.Fatalf("%s: hits %d to %d", label, before, after)
		}
		if !hit && after != before {
			t.Fatalf("%s: recorded a hit %d to %d", label, before, after)
		}
	}

	polled, pollBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+id, nil, siteKey)
	if polled.Code != http.StatusOK || pollBody["fallback_url"] != plain {
		t.Fatalf("poll fallback: %d %s", polled.Code, polled.Body.String())
	}
	listed, listBody := postJSON(t, r, http.MethodGet, "/v1/placements", nil, siteKey)
	if listed.Code != http.StatusOK {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}
	foundFallback := false
	for _, raw := range listBody["placements"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == id {
			foundFallback = item["fallback_url"] == plain
		}
	}
	if !foundFallback {
		t.Fatalf("list fallback: %s", listed.Body.String())
	}

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='placed' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("revoked", plain, false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='remove', plugin_wait_at=$2 WHERE id=$1`, id, current); err != nil {
		t.Fatal(err)
	}
	expect("pending undo", plain, false)

	pauseThen := func(scope string, payload map[string]any) {
		t.Helper()
		makeLive()
		paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", payload, siteKey)
		if paused.Code != http.StatusOK {
			t.Fatalf("pause %s: %d %s", scope, paused.Code, paused.Body.String())
		}
		expect("paused "+scope, plain, false)
	}
	pauseThen("all", map[string]any{"scope": "all"})
	pauseThen("site", map[string]any{"scope": "site", "id": siteID})
	pauseThen("offer", map[string]any{"scope": "offer", "id": offerID})

	makeLive()
	killed, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", map[string]any{}, siteKey)
	if killed.Code != http.StatusOK {
		t.Fatalf("kill switch: %d %s", killed.Code, killed.Body.String())
	}
	expect("kill switch", plain, false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='remove', plugin_wait_at=$2 WHERE id=$1`, id, current.Add(-11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	expect("offline", plain, false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='restore', plugin_failed_reason='re-read failed', failed_action='restored', restore_unconfirmed=true
WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("failed", plain, false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='removed' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("removed", plain, false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	expect("offer not live", postURL, false)

	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET post_url='' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("offer not live missing post", siteHomeURL, false)

	makeLive()
	beforeLive := clicks()
	live := goHit("live", "/v1/go/"+id, "198.51.100.8:1000", testBrowserUA, "https://go.example/the-post", nil)
	liveURL, err := url.Parse(live.Header().Get("Location"))
	if err != nil || liveURL.Scheme+"://"+liveURL.Host+liveURL.Path != plain || liveURL.Query().Get("via") == "" {
		t.Fatalf("live location: %s", live.Header().Get("Location"))
	}
	if clicks() != beforeLive+1 {
		t.Fatalf("live hit was not recorded: %d", clicks())
	}
	readerHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	otherHash := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	forwarded := map[string]string{
		"X-OR-Client-IP":   "203.0.113.50",
		"X-OR-Client-Hash": readerHash,
		"X-OR-User-Agent":  testFirefoxUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	ignored := goHit("headers without key", "/v1/go/"+id+"?via=plugin", "192.0.2.20:1000", testSafariUA, "https://server.internal/", forwarded)
	if !strings.Contains(ignored.Header().Get("Location"), "via=") {
		t.Fatalf("unkeyed plugin call was not tracked: %s", ignored.Header().Get("Location"))
	}
	var ua, referrer, ipHash string
	if err := db.Pool.QueryRow(context.Background(), `SELECT user_agent, referrer, ip_hash FROM clicks WHERE placement_id=$1 ORDER BY clicked_at DESC LIMIT 1`, id).Scan(&ua, &referrer, &ipHash); err != nil {
		t.Fatal(err)
	}
	if ua != testSafariUA || referrer != "https://server.internal/" || ipHash != redirects.directReaderHash("192.0.2.20") || ipHash == readerHash {
		t.Fatalf("forwarded headers were used without a site key: %s %s %s", ua, referrer, ipHash)
	}

	wrong := map[string]string{
		"X-API-Key":        "go-other-key",
		"X-OR-Client-IP":   "203.0.113.51",
		"X-OR-Client-Hash": otherHash,
		"X-OR-User-Agent":  "OtherSiteReader/1",
		"X-OR-Referer":     "https://other.example/post",
	}
	goHit("headers with other site key", "/v1/go/"+id+"?via=plugin", "192.0.2.21:1000", testBrowserUA, "https://server.internal/other", wrong)
	if err := db.Pool.QueryRow(context.Background(), `SELECT user_agent, referrer, ip_hash FROM clicks WHERE placement_id=$1 ORDER BY clicked_at DESC LIMIT 1`, id).Scan(&ua, &referrer, &ipHash); err != nil {
		t.Fatal(err)
	}
	if ua != testBrowserUA || referrer != "https://server.internal/other" || ipHash != redirects.directReaderHash("192.0.2.21") || ipHash == otherHash {
		t.Fatalf("forwarded headers were used for another site's key: %s %s %s", ua, referrer, ipHash)
	}

	matched := map[string]string{
		"X-API-Key":        "go-site-key",
		"X-OR-Client-IP":   "203.0.113.60",
		"X-OR-Client-Hash": readerHash,
		"X-OR-User-Agent":  testFirefoxUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	tracked := goHit("plugin proxy", "/v1/go/"+id+"?via=plugin", "192.0.2.30:1000", "PublisherServer/1.0", "https://plugin.internal/", matched)
	if !strings.Contains(tracked.Header().Get("Location"), "via=") {
		t.Fatalf("plugin proxy was not tracked: %s", tracked.Header().Get("Location"))
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT user_agent, referrer, ip_hash FROM clicks WHERE placement_id=$1 ORDER BY clicked_at DESC LIMIT 1`, id).Scan(&ua, &referrer, &ipHash); err != nil {
		t.Fatal(err)
	}
	if ua != testFirefoxUA || referrer != "https://reader.example/article" || ipHash != readerHash || strings.Contains(ipHash, "203.0.113.60") {
		t.Fatalf("plugin proxy did not use reader context: %s %s %s", ua, referrer, ipHash)
	}
	beforeDup := clicks()
	goHit("plugin dedup", "/v1/go/"+id+"?via=plugin", "192.0.2.31:1000", "PublisherServer/1.0", "https://plugin.internal/", matched)
	if clicks() != beforeDup {
		t.Fatalf("same hash and user agent was not deduped: %d to %d", beforeDup, clicks())
	}
	matched["X-OR-User-Agent"] = testSafariUA
	goHit("plugin different agent", "/v1/go/"+id+"?via=plugin", "192.0.2.32:1000", "PublisherServer/1.0", "https://plugin.internal/", matched)
	if clicks() != beforeDup+1 {
		t.Fatalf("a different user agent should be a new hit: %d", clicks())
	}

	beforeBot := clicks()
	botHeaders := map[string]string{
		"X-API-Key":        "go-site-key",
		"X-OR-Client-IP":   "203.0.113.70",
		"X-OR-Client-Hash": readerHash,
		"X-OR-User-Agent":  "Twitterbot/1.0",
		"X-OR-Referer":     "https://reader.example/article",
	}
	bot := goHit("forwarded bot", "/v1/go/"+id+"?via=plugin", "192.0.2.40:1000", testBrowserUA, "https://go.example/the-post", botHeaders)
	if bot.Header().Get("Location") != plain || clicks() != beforeBot+1 {
		t.Fatalf("forwarded bot: %s hits %d", bot.Header().Get("Location"), clicks())
	}
	var botClass string
	if err := db.Pool.QueryRow(context.Background(), `SELECT click_class FROM clicks WHERE placement_id=$1 AND user_agent='Twitterbot/1.0'`, id).Scan(&botClass); err != nil || botClass != "unknown" {
		t.Fatalf("forwarded bot class %s %v", botClass, err)
	}

	unknownID := func(label, path string, headers map[string]string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "127.0.0.1:8443"
		req.RemoteAddr = "192.0.2.50:1000"
		req.Header.Set("User-Agent", testBrowserUA)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" || !strings.Contains(rec.Body.String(), "This link is not on Offer Rails.") || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s: %d location %q type %q body %s", label, rec.Code, rec.Header().Get("Location"), rec.Header().Get("Content-Type"), rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("%s dropped go-link headers", label)
		}
	}
	unknownID("unknown id", "/v1/go/00000000-0000-4000-8000-000000000099", nil)
	unknownID("unknown id with site key", "/v1/go/00000000-0000-4000-8000-000000000098?via=plugin", siteKey)
}
