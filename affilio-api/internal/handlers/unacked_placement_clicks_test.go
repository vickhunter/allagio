package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/entry"
)

// TestUnackedTrackedLinkCountsEverySurface is GAP-01. One committed tracked
// link, with no plugin-ack, counts a browser click from the first redirect
// on an email open (no referrer), an X referrer, an Instagram referrer, and
// a blog referrer. plugin-ack stays a verification signal. A revoked link
// and a removed link do not count. The same reader inside the dedupe window
// does not count twice.
func TestUnackedTrackedLinkCountsEverySurface(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	redirects := &RedirectsHandler{DB: db, HomeURL: "https://offerrails.example/", IPSaltSecret: "gap01-salt"}
	r.GET("/v1/go/:code", redirects.Go)

	siteID := createTestSite(t, db, "gap01-site-key", "gap01.example", "Gap")
	_, offerID := seedListedOffer(t, db, "gap01-maker-key", "gap01-maker@example.com", "Gap Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://gap01.example/the-post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'gap01-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='', plugin_failed_reason='', failed_action='', restore_unconfirmed=false,
campaign_id=$2, expires_at=NOW()+INTERVAL '2 days', post_url='https://gap01.example/the-post'
WHERE id=$1`, placementID, campaignID); err != nil {
		t.Fatal(err)
	}
	var live bool
	var wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1), plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&live, &wait); err != nil || live || wait != "" {
		t.Fatalf("unacked placement was live or already acked: live %v wait %q err %v", live, wait, err)
	}

	type open struct {
		name     string
		remote   string
		referrer string
		source   string
	}
	opens := []open{
		{name: "email", remote: "203.0.113.10:443", referrer: "", source: "direct"},
		{name: "x", remote: "203.0.114.10:443", referrer: "https://x.com/offerrails/status/1", source: "x.com"},
		{name: "instagram", remote: "203.0.115.10:443", referrer: "https://instagram.com/p/abc", source: "instagram.com"},
		{name: "blog", remote: "203.0.116.10:443", referrer: "https://blog.example/posts/note", source: "blog.example"},
	}
	hit := func(remote, referrer string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/go/"+placementID, nil)
		req.RemoteAddr = remote
		req.Header.Set("User-Agent", testBrowserUA)
		if referrer != "" {
			req.Header.Set("Referer", referrer)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	for _, op := range opens {
		rec := hit(op.remote, op.referrer)
		parsed, err := url.Parse(rec.Header().Get("Location"))
		if rec.Code != http.StatusFound || err != nil || parsed.Query().Get("via") == "" {
			t.Fatalf("%s: %d %s", op.name, rec.Code, rec.Header().Get("Location"))
		}
		if got := entry.ClickSource(op.referrer); got != op.source {
			t.Fatalf("%s source %s", op.name, got)
		}
	}
	rows, err := db.Pool.Query(context.Background(), `SELECT COALESCE(referrer,''), click_class, COALESCE(ip_hash,'') FROM clicks WHERE placement_id=$1`, placementID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	sources := map[string]int{}
	n := 0
	for rows.Next() {
		var referrer, class, hash string
		if err := rows.Scan(&referrer, &class, &hash); err != nil {
			t.Fatal(err)
		}
		if class != ClickClassBrowser {
			t.Fatalf("class %s for %q", class, referrer)
		}
		if hash == "" || len(hash) != 64 || strings.Contains(hash, ".") {
			t.Fatalf("reader hash %q", hash)
		}
		sources[entry.ClickSource(referrer)]++
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("clicks %d sources %v", n, sources)
	}
	for _, op := range opens {
		if sources[op.source] != 1 {
			t.Fatalf("source %s count %d in %v", op.source, sources[op.source], sources)
		}
	}

	beforeDup := n
	dup := hit("203.0.114.20:443", "https://x.com/offerrails/status/1")
	if dup.Code != http.StatusFound || !containsAll(dup.Header().Get("Location"), "via=") || placementClickCount(t, db, placementID) != beforeDup {
		t.Fatalf("dedupe: %d %s clicks %d", dup.Code, dup.Header().Get("Location"), placementClickCount(t, db, placementID))
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&wait); err != nil || wait != "" {
		t.Fatalf("plugin-ack was required: wait %q %v", wait, err)
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='remove', plugin_wait_at=NOW() WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	revoked := hit("203.0.117.10:443", "https://blog.example/posts/note")
	requirePlainRedirect(t, revoked, "https://example.com")
	if placementClickCount(t, db, placementID) != 4 {
		t.Fatal("revoked link credited a click")
	}

	removedID := insertPlacement(t, db, siteID, offerID, "https://gap01.example/other-post", "Another honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='removed', campaign_id=$2, expires_at=NOW()+INTERVAL '2 days'
WHERE id=$1`, removedID, campaignID); err != nil {
		t.Fatal(err)
	}
	removedReq := httptest.NewRequest(http.MethodGet, "/v1/go/"+removedID, nil)
	removedReq.RemoteAddr = "198.51.100.10:443"
	removedReq.Header.Set("User-Agent", testBrowserUA)
	removedReq.Header.Set("Referer", "https://blog.example/posts/note")
	removedRec := httptest.NewRecorder()
	r.ServeHTTP(removedRec, removedReq)
	requirePlainRedirect(t, removedRec, "https://example.com")
	if placementClickCount(t, db, removedID) != 0 {
		t.Fatal("removed link credited a click")
	}
}
