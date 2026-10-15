package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
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

// TestTimestamptzSurvivesRomeDSTFold writes the two Europe/Rome instants that
// share the wall clock 02:30 on 25 Oct 2026 and reads each one back. The fall
// back from CEST to CET makes that local time ambiguous for timestamp without
// time zone.
func TestTimestamptzSurvivesRomeDSTFold(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	before := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC) // 02:30 CEST
	after := time.Date(2026, 10, 25, 1, 30, 0, 0, time.UTC)  // 02:30 CET
	assertFoldInstant(t, db, "clicks", "clicked_at", `INSERT INTO clicks(clicked_at) VALUES ($1::timestamptz) RETURNING id`, before, after)
	assertFoldInstant(t, db, "conversions", "occurred_at", `INSERT INTO conversions(provider, provider_event_id, kind, amount, occurred_at) VALUES ('stripe', $1, 'sale', 10, $2::timestamptz) RETURNING id`, before, after)
	assertFoldInstant(t, db, "ledger_entries", "clears_at", `INSERT INTO ledger_entries(kind, amount, clears_at) VALUES ('commission', 1, $1::timestamptz) RETURNING id`, before, after)

	up, err := os.ReadFile(filepath.Join("..", "migrations", "038_timestamptz_instants.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(up)); err != nil {
		t.Fatalf("re-applying the migration: %v", err)
	}
	assertFoldInstant(t, db, "clicks", "clicked_at", `INSERT INTO clicks(clicked_at) VALUES ($1::timestamptz) RETURNING id`, before, after)

	down, err := os.ReadFile(filepath.Join("..", "migrations", "down", "038_timestamptz_instants.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp without time zone")
	assertColumnType(t, db, "conversions", "occurred_at", "timestamp without time zone")
	assertColumnType(t, db, "ledger_entries", "clears_at", "timestamp without time zone")
	if _, err = db.Pool.Exec(context.Background(), string(up)); err != nil {
		t.Fatalf("up migration after down: %v", err)
	}
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp with time zone")
	assertFoldInstant(t, db, "clicks", "clicked_at", `INSERT INTO clicks(clicked_at) VALUES ($1::timestamptz) RETURNING id`, before, after)
}

func assertFoldInstant(t *testing.T, db *database.DB, table, column, insert string, before, after time.Time) {
	t.Helper()
	gotBefore := insertAndReadInstant(t, db, table, column, insert, before, "fold-before-"+table)
	gotAfter := insertAndReadInstant(t, db, table, column, insert, after, "fold-after-"+table)
	if !gotBefore.Equal(before) {
		t.Fatalf("%s.%s before fold: got %s want %s", table, column, gotBefore.Format(time.RFC3339Nano), before.Format(time.RFC3339Nano))
	}
	if !gotAfter.Equal(after) {
		t.Fatalf("%s.%s after fold: got %s want %s", table, column, gotAfter.Format(time.RFC3339Nano), after.Format(time.RFC3339Nano))
	}
	if gotBefore.Equal(gotAfter) || gotAfter.Sub(gotBefore) != time.Hour {
		t.Fatalf("%s.%s collapsed the fold: %s and %s", table, column, gotBefore.Format(time.RFC3339Nano), gotAfter.Format(time.RFC3339Nano))
	}
}

func insertAndReadInstant(t *testing.T, db *database.DB, table, column, insert string, instant time.Time, label string) time.Time {
	t.Helper()
	var id string
	var err error
	if strings.Contains(insert, "$2") {
		err = db.Pool.QueryRow(context.Background(), insert, label, instant).Scan(&id)
	} else {
		err = db.Pool.QueryRow(context.Background(), insert, instant).Scan(&id)
	}
	if err != nil {
		t.Fatalf("insert %s: %v", table, err)
	}
	var got time.Time
	query := "SELECT " + column + " FROM " + table + " WHERE id=$1"
	if err = db.Pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertColumnType(t *testing.T, db *database.DB, table, column, want string) {
	t.Helper()
	var got string
	if err := db.Pool.QueryRow(context.Background(), `SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`, table, column).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s.%s type %s, want %s", table, column, got, want)
	}
}

func TestPartialRefundWhileSessionIsEuropeRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newPayoutsRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-partial-rome"
	createTestSite(t, db, siteKey, "partial-rome.example.com", "Partial Rome")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)

	saleAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	refundAt := saleAt.Add(time.Hour)
	sale := stripeSalePayload("evt_rome_partial_sale", "pi_rome_partial", clickID, 10000)
	sale["created"] = saleAt.Unix()
	wrec, wbody := sendStripeEvent(t, db, r, sale)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale: %d %v", wrec.Code, wbody)
	}
	refund := map[string]interface{}{
		"id": "evt_rome_partial_refund", "type": "charge.refunded", "created": refundAt.Unix(),
		"data": map[string]interface{}{"object": map[string]interface{}{
			"id": "ch_rome_partial", "payment_intent": "pi_rome_partial", "amount_refunded": 4000, "currency": "usd",
		}},
	}
	wrec, wbody = sendStripeEvent(t, db, r, refund)
	if wrec.Code != http.StatusOK || wbody["status"] != "refunded" {
		t.Fatalf("partial refund: %d %v", wrec.Code, wbody)
	}
	_, balance := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd := balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"].(float64) != 12 {
		t.Fatalf("pending after 40%% refund: %v", usd)
	}
	wantClear := saleAt.Add(30 * 24 * time.Hour)
	var commissionClear, clawbackClear, occurred time.Time
	if err := db.Pool.QueryRow(context.Background(), `SELECT le.clears_at FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id WHERE cv.provider_event_id='evt_rome_partial_sale' AND le.kind='commission'`).Scan(&commissionClear); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT le.clears_at FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id WHERE cv.provider_event_id='evt_rome_partial_refund' AND le.kind='clawback'`).Scan(&clawbackClear); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT occurred_at FROM conversions WHERE provider_event_id='evt_rome_partial_refund'`).Scan(&occurred); err != nil {
		t.Fatal(err)
	}
	if !commissionClear.Equal(wantClear) || !clawbackClear.Equal(wantClear) {
		t.Fatalf("clears_at commission %s clawback %s, want %s", commissionClear.Format(time.RFC3339Nano), clawbackClear.Format(time.RFC3339Nano), wantClear.Format(time.RFC3339Nano))
	}
	if !occurred.Equal(refundAt) {
		t.Fatalf("refund occurred_at %s, want %s", occurred.Format(time.RFC3339Nano), refundAt.Format(time.RFC3339Nano))
	}
}

func TestSaleAtClickInstantStaysUnflagged(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	instant := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	db.Now = func() time.Time { return instant }

	_, makerKey := createTestMaker(t, db)
	r := newPayoutsRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-exact-instant"
	createTestSite(t, db, siteKey, "exact.example.com", "Exact Blog")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)

	var clicked time.Time
	if err := db.Pool.QueryRow(context.Background(), `SELECT clicked_at FROM clicks WHERE id=$1`, clickID).Scan(&clicked); err != nil {
		t.Fatal(err)
	}
	if !clicked.Equal(instant) {
		t.Fatalf("click instant %s, want %s", clicked.Format(time.RFC3339Nano), instant.Format(time.RFC3339Nano))
	}
	sale := stripeSalePayload("evt_exact_instant", "pi_exact_instant", clickID, 10000)
	sale["created"] = instant.Unix()
	wrec, wbody := sendStripeEvent(t, db, r, sale)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale at the click: %d %v", wrec.Code, wbody)
	}
	var flagged bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged FROM conversions WHERE provider_event_id='evt_exact_instant'`).Scan(&flagged); err != nil {
		t.Fatal(err)
	}
	if flagged {
		t.Fatal("sale at exactly the click instant was flagged")
	}

	later := instant.Add(5 * time.Second)
	fast := stripeSalePayload("evt_five_seconds", "pi_five_seconds", clickID, 10000)
	fast["created"] = later.Unix()
	wrec, wbody = sendStripeEvent(t, db, r, fast)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale five seconds later: %d %v", wrec.Code, wbody)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged FROM conversions WHERE provider_event_id='evt_five_seconds'`).Scan(&flagged); err != nil {
		t.Fatal(err)
	}
	if !flagged {
		t.Fatal("sale five seconds after the click stayed unflagged")
	}
}

func TestKeywordStyleDSNSessionTimeZone(t *testing.T) {
	raw := "host=localhost port=5432 dbname=affilio user=affilio password=secret sslmode=disable"
	got, err := withSessionTimeZone(raw, "Europe/Rome")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "%") || strings.Contains(got, "://") {
		t.Fatalf("keyword DSN was rewritten as a URL: %s", got)
	}
	for _, part := range []string{"host=localhost", "port=5432", "dbname=affilio", "user=affilio", "password=secret", "sslmode=disable", "TimeZone=Europe/Rome"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in %s", part, got)
		}
	}
	replaced, err := withSessionTimeZone(got, "UTC")
	if err != nil || strings.Count(replaced, "TimeZone=") != 1 || !strings.Contains(replaced, "TimeZone=UTC") || strings.Contains(replaced, "Europe/Rome") {
		t.Fatalf("timezone not replaced: %s %v", replaced, err)
	}
	same, err := withSessionTimeZone(raw, "  ")
	if err != nil || same != raw {
		t.Fatalf("empty zone changed the keyword DSN: %q %v", same, err)
	}
}

func TestKeywordDSNConnectsInSessionTimeZone(t *testing.T) {
	raw := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set")
	}
	keyword := keywordDSN(t, raw)
	got, err := withSessionTimeZone(keyword, "Europe/Rome")
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(context.Background(), got)
	if err != nil {
		t.Fatalf("connect keyword DSN %s: %v", got, err)
	}
	defer db.Close()
	var zone string
	if err = db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone != "Europe/Rome" {
		t.Fatalf("keyword DSN session timezone %s", zone)
	}
}

func keywordDSN(t *testing.T, raw string) string {
	t.Helper()
	if !strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	name := strings.TrimPrefix(u.Path, "/")
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	parts := []string{"host=" + host, "port=" + port, "dbname=" + name, "user=" + user, "password=" + pass}
	q := u.Query()
	q.Del("TimeZone")
	for key, values := range q {
		if len(values) > 0 {
			parts = append(parts, key+"="+values[0])
		}
	}
	return strings.Join(parts, " ")
}
