package handlers

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
)

func TestDigestOfferIncludesReviewFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.ClaimsCheck = config.ClaimsStub
	owner, cookie := testOwnerSession(t, h, "digest-offer-owner", "digest-offer@example.com")
	makerID, listedID := seedListedOffer(t, db, "digest-offer-maker", "digest-offer-maker@example.com", "Listed Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	var submittedID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'submitted',commission_type,commission_value,landing_url FROM offers WHERE id=$1 RETURNING id`, listedID).Scan(&submittedID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	seen := map[string]bool{}
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		if item["kind"] != "offer" {
			continue
		}
		if _, ok := item["status"]; !ok {
			t.Fatalf("status missing: %v", item)
		}
		if _, ok := item["review_reason"]; !ok {
			t.Fatalf("review_reason missing: %v", item)
		}
		if _, ok := item["message"]; !ok {
			t.Fatalf("message missing: %v", item)
		}
		if _, ok := item["plugin_failed_reason"]; ok {
			t.Fatalf("offer included plugin_failed_reason: %v", item)
		}
		id := item["id"].(string)
		seen[id] = true
		switch id {
		case listedID:
			if item["status"] != "listed" || item["review_reason"] != nil || item["message"] != nil || item["live_now"] != true || item["claims_check"] != "not_checked" {
				t.Fatalf("listed offer: %v", item)
			}
		case submittedID:
			if item["status"] != "submitted" || item["review_reason"] != "claims" || item["message"] != "Claims are not checked yet." || item["live_now"] != false || item["claims_check"] != "not_checked" {
				t.Fatalf("submitted offer: %v", item)
			}
		}
	}
	if !seen[listedID] || !seen[submittedID] {
		t.Fatalf("offers missing: %v", seen)
	}
	if body["claims_check"] != "not_checked" {
		t.Fatalf("stub digest claims_check: %s", rec.Body.String())
	}
}

func TestDigestListsOfferWhenSessionTimeZoneIsNotUTC(t *testing.T) {
	raw := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("TimeZone", "Europe/Rome")
	u.RawQuery = q.Encode()
	db, err := database.Connect(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	applyMigrations(t, db)
	defer func() {
		_ = truncateAll(db)
		db.Close()
	}()

	var zone string
	if err = db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone == "UTC" || zone == "Etc/UTC" {
		t.Fatalf("session timezone stayed %s", zone)
	}

	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "digest-tz", "digest-tz@example.com")
	makerID, offerID := seedListedOffer(t, db, "digest-tz-maker", "digest-tz-maker@example.com", "Digest Product")
	if _, err = db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `INSERT INTO owner_events(kind, user_id, created_at) VALUES('digest_opened',$1,NOW()-INTERVAL '1 hour')`, owner.ID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	for _, rawItem := range body["items"].([]any) {
		item := rawItem.(map[string]any)
		if item["id"] == offerID && item["kind"] == "offer" && item["live_now"] == true {
			return
		}
	}
	t.Fatalf("listed offer missing in %s: %s", zone, rec.Body.String())
}

// TestDigestItemsIncludeUIFieldsUnderEuropeRome re-runs the digest UI test in
// a process whose TZ is Europe/Rome. pgx scans the digest floor with
// time.Local, so a UTC database still drops the listed offer when that floor
// is sent as a timestamp.
func TestDigestItemsIncludeUIFieldsUnderEuropeRome(t *testing.T) {
	if strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL")) == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	if os.Getenv("AFFILIO_TZ_CHILD") == "1" {
		TestDigestItemsIncludeUIFields(t)
		return
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "TZ=") || strings.HasPrefix(item, "AFFILIO_TZ_CHILD=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "TZ=Europe/Rome", "AFFILIO_TZ_CHILD=1")
	cmd := exec.Command(os.Args[0], "-test.run", "^TestDigestItemsIncludeUIFieldsUnderEuropeRome$", "-test.count=1", "-test.v")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Europe/Rome: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--- PASS: TestDigestItemsIncludeUIFieldsUnderEuropeRome") {
		t.Fatalf("Europe/Rome child did not pass the digest test:\n%s", out)
	}
}
