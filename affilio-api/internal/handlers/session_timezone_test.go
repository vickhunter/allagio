package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWithSessionTimeZone(t *testing.T) {
	raw := "postgresql://affilio:password@localhost:5432/affilio_test?sslmode=disable"
	got, err := withSessionTimeZone(raw, "Europe/Rome")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("TimeZone") != "Europe/Rome" || u.Query().Get("sslmode") != "disable" {
		t.Fatalf("timezone url: %s", got)
	}
	if u.User.Username() != "affilio" {
		t.Fatalf("user: %s", got)
	}
	same, err := withSessionTimeZone(raw, "  ")
	if err != nil || same != raw {
		t.Fatalf("empty zone changed the url: %q %v", same, err)
	}
}

func TestHarnessSessionTimeZone(t *testing.T) {
	zone := strings.TrimSpace(os.Getenv(sessionTimeZoneEnv))
	if zone == "" {
		t.Skip("AFFILIO_TEST_TIMEZONE not set")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	var got string
	if err := db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != zone {
		t.Fatalf("session timezone %s, want %s", got, zone)
	}
}

// TestSaleAttributionAndPayoutWhenSessionTimeZoneIsNotUTC keeps a sale inside
// the cookie window and the refund hold when the session clock is not UTC.
// clicks.clicked_at and ledger_entries.clears_at are timestamp without time
// zone, written in the session wall clock. A Go instant has to meet that clock.
func TestSaleAttributionAndPayoutWhenSessionTimeZoneIsNotUTC(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	var zone string
	if err := db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone == "UTC" || zone == "Etc/UTC" {
		t.Fatalf("session timezone stayed %s", zone)
	}

	_, makerKey := createTestMaker(t, db)
	r := newPayoutsRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-session-zone"
	createTestSite(t, db, siteKey, "zone.example.com", "Zone Blog")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)

	immediate := stripeSalePayload("evt_zone_now", "pi_zone_now", clickID, 10000)
	immediate["created"] = time.Now().Unix()
	wrec, wbody := sendStripeEvent(t, db, r, immediate)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale at now: expected recorded, got %d %v", wrec.Code, wbody)
	}

	later := stripeSalePayload("evt_zone_later", "pi_zone_later", clickID, 20000)
	wrec, wbody = sendStripeEvent(t, db, r, later)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale two hours later: expected recorded, got %d %v", wrec.Code, wbody)
	}
	var flagged bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged FROM conversions WHERE provider_event_id='evt_zone_later'`).Scan(&flagged); err != nil {
		t.Fatal(err)
	}
	if flagged {
		t.Fatalf("organic purchase flagged in session %s", zone)
	}
	created, ok := later["created"].(int64)
	if !ok {
		t.Fatalf("sale created type %T", later["created"])
	}
	var skew float64
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXTRACT(EPOCH FROM ((le.clears_at AT TIME ZONE current_setting('TimeZone')) - $1::timestamptz))
FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id
WHERE cv.provider_event_id='evt_zone_later' AND le.kind='commission'`, time.Unix(created, 0).UTC().Add(30*24*time.Hour)).Scan(&skew); err != nil {
		t.Fatal(err)
	}
	if skew < -2 || skew > 2 {
		t.Fatalf("clears_at is %.0f seconds from the refund window in %s", skew, zone)
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at = NOW() - INTERVAL '1 day' WHERE conversion_id IN (SELECT id FROM conversions WHERE provider_event_id='evt_zone_later')`); err != nil {
		t.Fatal(err)
	}
	rec, body := adminJSON(t, r, http.MethodPost, "/v1/payout-batches/run", map[string]interface{}{"min_payout": 10})
	payouts, _ := body["payouts"].([]interface{})
	if rec.Code != http.StatusOK || len(payouts) != 1 {
		t.Fatalf("expected the matured commission to pay, got %d %v", rec.Code, body)
	}
}
