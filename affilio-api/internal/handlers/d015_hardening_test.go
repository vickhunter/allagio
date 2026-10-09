package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIsPreviewBotUsesExactTokens(t *testing.T) {
	visitors := []string{
		"Mozilla/5.0 MyWhatsAppBrowser/1.0",
		"NotATwitterbot/1.0",
		"facebookexternalhitextra/1.0",
		"Mozilla/5.0 (Linux) Chrome/120.0.0.0 Safari/537.36",
		"I use slackbot-like tooling",
	}
	for _, ua := range visitors {
		if isPreviewBot(ua) {
			t.Fatalf("substring match treated %q as a preview bot", ua)
		}
	}
	bots := []string{
		"Twitterbot/1.0",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"Quora Link Preview/1.0",
		"WhatsApp/2.23.20.0",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
	}
	for _, ua := range bots {
		if !isPreviewBot(ua) {
			t.Fatalf("exact token missed preview bot %q", ua)
		}
	}
}

func TestFailedClickInsertDoesNotRedirect(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-click-fail"
	createTestSite(t, db, siteKey, "fail.example.com", "Fail")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	if _, err := db.Pool.Exec(context.Background(), `
CREATE OR REPLACE FUNCTION d015_fail_click() RETURNS trigger AS $$
BEGIN
  IF NEW.user_agent = 'd015-fail-insert' THEN
    RAISE EXCEPTION 'forced click insert failure';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS d015_fail_click ON clicks;
CREATE TRIGGER d015_fail_click BEFORE INSERT ON clicks FOR EACH ROW EXECUTE FUNCTION d015_fail_click()`); err != nil {
		t.Fatal(err)
	}

	rec := followGo(r, http.MethodGet, code, "d015-fail-insert")
	if rec.Code == http.StatusFound || rec.Header().Get("Location") != "" {
		t.Fatalf("failed click insert redirected without attribution: %d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 attribution failure, got %d %s", rec.Code, rec.Body.String())
	}
	if n := clickCount(t, db); n != 0 {
		t.Fatalf("failed insert stored %d clicks", n)
	}
}

func TestSharedAdminKeyCannotApprovePlacementOrMarkPayoutPaid(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newRevenueRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-d015-admin"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Publisher")
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/post",
		"context": "InboxPilot email inbox productivity tutorial", "copy": "InboxPilot is a tool to consider for your email workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %v", rec.Code, prepared)
	}
	id := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = doJSONWithHeader(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", "X-Admin-Key", "admin-secret", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Shared admin key must not approve this placement.",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("shared admin key approved a placement: %d %s", rec.Code, rec.Body.String())
	}
	var payoutID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id, amount, currency, status) VALUES($1, 25, 'USD', 'pending') RETURNING id`, siteID).Scan(&payoutID); err != nil {
		t.Fatal(err)
	}
	rec, _ = doJSONWithHeader(t, r, http.MethodPost, "/v1/payout-batches/"+payoutID+"/mark-paid", "X-Admin-Key", "admin-secret", map[string]interface{}{"provider_reference": "BANK-1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("shared admin key marked a payout paid: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOperatorKeyApprovesPlacementAndMarksPayoutPaid(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newRevenueRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-d015-operator"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Publisher")
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/post",
		"context": "InboxPilot email inbox productivity tutorial", "copy": "InboxPilot is a tool to consider for your email workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %v", rec.Code, prepared)
	}
	id := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = doJSONWithHeader(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", "X-Operator-Key", testOperatorKey, map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Operator recorded the publisher approval.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("operator approval: %d %s", rec.Code, rec.Body.String())
	}
	var operatorID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT approved_by_operator::text FROM placement_proposals WHERE id=$1`, id).Scan(&operatorID); err != nil || operatorID == "" {
		t.Fatalf("approval was not attributed to an operator: %v %q", err, operatorID)
	}
	var payoutID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id, amount, currency, status) VALUES($1, 25, 'USD', 'pending') RETURNING id`, siteID).Scan(&payoutID); err != nil {
		t.Fatal(err)
	}
	rec, _ = doJSONWithHeader(t, r, http.MethodPost, "/v1/payout-batches/"+payoutID+"/mark-paid", "X-Operator-Key", testOperatorKey, map[string]interface{}{"provider_reference": "BANK-2"})
	if rec.Code != http.StatusOK {
		t.Fatalf("operator mark-paid: %d %s", rec.Code, rec.Body.String())
	}
	var markedBy string
	if err := db.Pool.QueryRow(context.Background(), `SELECT marked_paid_by::text FROM payouts WHERE id=$1`, payoutID).Scan(&markedBy); err != nil || markedBy != operatorID {
		t.Fatalf("payout marked by %q, approval operator %q, err=%v", markedBy, operatorID, err)
	}
}

func TestAdminStripeConnectRejectsMalformedSecretAndStoresCiphertext(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newRevenueRouter(db)
	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name": "Cipher Product", "website_url": "https://cipher.example.com", "product_type": "saas", "payment_provider": "stripe",
	})
	productID := product["id"].(string)
	bad := "whsec_bad secret!"
	rec, _ := adminJSON(t, r, http.MethodPost, "/v1/offer-review/products/"+productID+"/connection", map[string]interface{}{
		"signing_secret": bad, "currency": "USD", "live_mode": false,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed signing secret accepted: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), bad) {
		t.Fatal("response echoed the signing secret")
	}
	secret := "whsec_ValidSecretKey123"
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/products/"+productID+"/connection", map[string]interface{}{
		"signing_secret": secret, "currency": "USD", "live_mode": false,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid secret rejected: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("response echoed the signing secret")
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT encode(secret_ciphertext, 'escape') FROM product_connections WHERE product_id=$1`, productID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) {
		t.Fatal("signing secret stored in plaintext")
	}
}

func TestLemonAndPolarWebhookSecretsStaySeparate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &WebhooksHandler{DB: db, Secrets: map[string]string{"lemonsqueezy": "shared-webhook-secret", "polar": "shared-webhook-secret"}}
	r.POST("/v1/webhooks/:provider", h.Receive)
	body := `{"meta":{"event_name":"order_created","custom_data":{"dtr":"11111111-2222-3333-4444-555555555555"}},"data":{"id":"1","attributes":{"identifier":"ord-1","total":1000,"currency":"usd","created_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}}}`
	mac := hmac.New(sha256.New, []byte("shared-webhook-secret"))
	mac.Write([]byte(body))
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("identical Lemon Squeezy and Polar secrets were accepted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSelfReferralSameOwnerOrPaymentIdentity(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newRevenueRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-d015-self"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Publisher")
	prepare := func() string {
		t.Helper()
		rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
			"offer_id": offerID, "surface_url": "https://publisher.example.com/post",
			"context": "InboxPilot email inbox productivity tutorial", "copy": "InboxPilot is a tool to consider for your email workflow.",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("prepare: %d %v", rec.Code, prepared)
		}
		return prepared["placement"].(map[string]interface{})["id"].(string)
	}
	approve := func(id string) *httptest.ResponseRecorder {
		t.Helper()
		rec, _ := doJSONWithHeader(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", "X-Operator-Key", testOperatorKey, map[string]interface{}{
			"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Different email, same party on both sides.",
		})
		return rec
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO owners(issuer, subject, email, name, site_id, maker_id) VALUES('https://identity.example','same-person','publisher@example.com','Same',$1,$2)`, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	ownerPlacement := prepare()
	if rec := approve(ownerPlacement); rec.Code != http.StatusConflict {
		t.Fatalf("same owner on both sides was approved: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM owners WHERE subject='same-person'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE sites SET stripe_customer_id='cus_same', payment_fingerprint='fp_same' WHERE id=$1`, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE makers SET stripe_customer_id='cus_same', payment_fingerprint='fp_same' WHERE id=$1`, makerID); err != nil {
		t.Fatal(err)
	}
	paymentPlacement := prepare()
	if rec := approve(paymentPlacement); rec.Code != http.StatusConflict {
		t.Fatalf("same Stripe customer and payment fingerprint was approved: %d %s", rec.Code, rec.Body.String())
	}
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	click := followGo(r, http.MethodGet, join["tracking_code"].(string), "Mozilla/5.0")
	location := click.Header().Get("Location")
	via := location[strings.Index(location, "via=")+4:]
	sale, saleBody := sendStripeEvent(t, db, r, stripeSalePayload("evt_self", "pi_self", via, 10000))
	if sale.Code != http.StatusOK || saleBody["status"] != "recorded" {
		t.Fatalf("self-referral sale: %d %v", sale.Code, saleBody)
	}
	var flagged bool
	var reason string
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged, COALESCE(flag_reason,'') FROM conversions WHERE provider_event_id='evt_self'`).Scan(&flagged, &reason); err != nil {
		t.Fatal(err)
	}
	if !flagged || reason != "self referral" {
		t.Fatalf("self-referral sale was payable: flagged=%v reason=%q", flagged, reason)
	}
}

func TestSaleStoresIntegerMinorUnits(t *testing.T) {
	db, r, _, _, clickID := testPaidCampaign(t)
	rec, body := sendStripeEvent(t, db, r, stripeSalePayload("evt_minor", "pi_minor", clickID, 10001))
	if rec.Code != http.StatusOK || body["status"] != "recorded" {
		t.Fatalf("sale: %d %v", rec.Code, body)
	}
	var amountMinor, commissionMinor, ledgerMinor int64
	err := db.Pool.QueryRow(context.Background(), `
SELECT cv.amount_minor, cv.commission_minor, le.amount_minor
FROM conversions cv JOIN ledger_entries le ON le.conversion_id=cv.id
WHERE cv.provider_event_id='evt_minor'`).Scan(&amountMinor, &commissionMinor, &ledgerMinor)
	if err != nil {
		t.Fatal(err)
	}
	if amountMinor != 10001 || commissionMinor != 2000 || ledgerMinor != 2000 {
		t.Fatalf("minor units amount=%d commission=%d ledger=%d", amountMinor, commissionMinor, ledgerMinor)
	}
	var offerMinor int64
	if err := db.Pool.QueryRow(context.Background(), `SELECT commission_value_minor FROM offers LIMIT 1`).Scan(&offerMinor); err != nil {
		t.Fatal(err)
	}
	if offerMinor != 2000 {
		t.Fatalf("offer commission stored as %d minor units", offerMinor)
	}
}
