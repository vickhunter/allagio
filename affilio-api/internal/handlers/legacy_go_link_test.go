package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestLegacyCampaignGoLink locks the permanent /v1/go/<code> route. Saved
// posts keep that URL, so the bare code and the same code with ?placement=
// both follow the go-link rules: 302, no-store, noindex, a hit only while the
// placement counts as live, and a rate limit that is an untracked 302.
func TestLegacyCampaignGoLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	redirects := &RedirectsHandler{
		DB: db, HomeURL: "https://offerrails.example/", IPSaltSecret: "test-salt",
		RateLimiter: NewIPRateLimiter(1000, time.Hour),
		now:         func() time.Time { return fixed },
	}
	r.GET("/v1/go/:code", redirects.Go)
	r.HEAD("/v1/go/:code", redirects.Go)

	owner, _ := testOwnerSession(t, h, "legacy-owner", "legacy-owner@example.com")
	siteID := createTestSite(t, db, "legacy-site-key", "legacy.example", "Legacy")
	makerID, offerID := seedListedOffer(t, db, "legacy-maker-key", "legacy-maker@example.com", "Legacy Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://offer.example/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "legacy-site-key"}
	postURL := "https://legacy.example/the-post"
	id := insertPlacement(t, db, siteID, offerID, postURL, "Honest note about the product.")
	const code = "legacy-go"
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,$3,true,'active') RETURNING id`, offerID, siteID, code).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	plain := "https://offer.example/buy"
	siteHomeURL := "https://legacy.example/"
	railsHome := "https://offerrails.example/"
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
	hit := func(label, path, remote string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		req.Header.Set("User-Agent", "Mozilla/5.0 reader")
		req.Header.Set("Referer", postURL)
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
	check := func(label, path, remote, want string, tracked bool) {
		t.Helper()
		before := clicks()
		rec := hit(label, path, remote)
		loc := rec.Header().Get("Location")
		if tracked {
			parsed, err := url.Parse(loc)
			if err != nil || parsed.Scheme+"://"+parsed.Host+parsed.Path != want || parsed.Query().Get("via") == "" {
				t.Fatalf("%s: location %s", label, loc)
			}
			if clicks() != before+1 {
				t.Fatalf("%s: hits %d to %d", label, before, clicks())
			}
			return
		}
		if loc != want {
			t.Fatalf("%s: location %s, want %s", label, loc, want)
		}
		if clicks() != before {
			t.Fatalf("%s: recorded a hit %d to %d", label, before, clicks())
		}
	}

	type legacyCase struct {
		name      string
		setup     func()
		bareWant  string
		queryWant string
		tracked   bool
		limited   bool
		barePath  string
		queryPath string
	}
	cases := []legacyCase{
		{name: "live", setup: makeLive, bareWant: plain, queryWant: plain, tracked: true},
		{name: "pending", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='', plugin_wait_at=$2 WHERE id=$1`, id, current); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "removed", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='removed' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "offline", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='remove', plugin_wait_at=$2 WHERE id=$1`, id, current.Add(-11*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "failed removal", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='remove', plugin_failed_reason='re-read failed', failed_action='removed', plugin_wait_at=$2
WHERE id=$1`, id, current); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "failed restore", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='restore', plugin_failed_reason='re-read failed', failed_action='restored', restore_unconfirmed=true
WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "revoked", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='placed' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "undo", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='remove', plugin_wait_at=$2 WHERE id=$1`, id, current); err != nil {
				t.Fatal(err)
			}
		}, bareWant: plain, queryWant: plain},
		{name: "pause all", setup: func() {
			makeLive()
			paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "all"}, siteKey)
			if paused.Code != http.StatusOK {
				t.Fatalf("pause all: %d %s", paused.Code, paused.Body.String())
			}
		}, bareWant: plain, queryWant: plain},
		{name: "pause site", setup: func() {
			makeLive()
			paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID}, siteKey)
			if paused.Code != http.StatusOK {
				t.Fatalf("pause site: %d %s", paused.Code, paused.Body.String())
			}
		}, bareWant: plain, queryWant: plain},
		{name: "pause offer", setup: func() {
			makeLive()
			paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "offer", "id": offerID}, siteKey)
			if paused.Code != http.StatusOK {
				t.Fatalf("pause offer: %d %s", paused.Code, paused.Body.String())
			}
		}, bareWant: plain, queryWant: plain},
		{name: "kill switch", setup: func() {
			makeLive()
			killed, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", map[string]any{}, siteKey)
			if killed.Code != http.StatusOK {
				t.Fatalf("kill switch: %d %s", killed.Code, killed.Body.String())
			}
		}, bareWant: plain, queryWant: plain},
		{name: "offer not live", setup: func() {
			makeLive()
			if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
				t.Fatal(err)
			}
		}, bareWant: siteHomeURL, queryWant: postURL},
		{
			name: "unknown code", setup: makeLive,
			bareWant: railsHome, queryWant: railsHome,
			barePath:  "/v1/go/missing-legacy",
			queryPath: "/v1/go/missing-legacy",
		},
		{name: "rate-limited", setup: func() {
			makeLive()
			redirects.RateLimiter = NewIPRateLimiter(1, time.Hour)
		}, bareWant: plain, queryWant: plain, limited: true},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			bare := tc.barePath
			if bare == "" {
				bare = "/v1/go/" + code
			}
			query := tc.queryPath
			if query == "" {
				query = "/v1/go/" + code + "?placement=" + id
			}
			forms := []struct {
				name string
				path string
				ip   string
				want string
			}{
				{"bare", bare, fmt.Sprintf("198.51.%d.10:443", i+1), tc.bareWant},
				{"placement", query, fmt.Sprintf("203.0.%d.10:443", i+1), tc.queryWant},
			}
			for i, form := range forms {
				if tc.limited {
					// The cap is the placement. The first reader is tracked. A
					// second hit, including one from another address, is not.
					if i == 0 {
						check(tc.name+" "+form.name+" allowed", form.path, form.ip, plain, true)
					}
					check(tc.name+" "+form.name, form.path, form.ip, form.want, false)
					continue
				}
				check(tc.name+" "+form.name, form.path, form.ip, form.want, tc.tracked)
			}
		})
	}
}
