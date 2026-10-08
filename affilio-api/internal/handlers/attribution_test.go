package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const testStripeSecret = "whsec_attribution_test"

func newAttributionRouter(db *database.DB) *gin.Engine {
	r := newMakerRouter(db)
	marketplace := &MarketplaceHandler{DB: db}
	redirects := &RedirectsHandler{DB: db}
	webhooks := &WebhooksHandler{DB: db, Cipher: testConnectionCipher()}
	r.GET("/v1/go/:code", redirects.Go)
	r.HEAD("/v1/go/:code", redirects.Go)
	r.POST("/v1/webhooks/:provider", webhooks.Receive)
	r.POST("/v1/webhooks/:provider/:product_id", webhooks.Receive)
	site := r.Group("/v1")
	site.Use(middleware.APIKeyAuth(db, services.NewAuditLogger(db)))
	{
		site.GET("/marketplace/offers", marketplace.ListOffers)
		site.POST("/marketplace/offers/:id/join", marketplace.JoinOffer)
	}
	return r
}

// listedOffer drives the full supply path: maker + product + offer, readiness,
// submit, admin approve. Returns the listed offer id.
func listedOffer(t *testing.T, db *database.DB, r *gin.Engine, makerKey string) string {
	t.Helper()
	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             "InboxPilot",
		"website_url":      "https://inboxpilot.example.com",
		"product_type":     "saas",
		"payment_provider": "stripe",
	})
	productID := product["id"].(string)
	markProductReady(t, db, productID)
	ciphertext, err := testConnectionCipher().Encrypt(productID, testStripeSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProductConnection(context.Background(), database.ProductConnection{ProductID: productID, SecretCiphertext: ciphertext, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":         productID,
		"commission_type":    "percentage",
		"commission_value":   20,
		"refund_window_days": 30,
	})
	offerID := offer["id"].(string)
	rec, _ := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit: expected 200, got %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/"+offerID+"/approve", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	arec := httptest.NewRecorder()
	r.ServeHTTP(arec, req)
	if arec.Code != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d: %s", arec.Code, arec.Body.String())
	}
	return offerID
}

func doSiteJSON(t *testing.T, r *gin.Engine, method, path, siteKey string, payload interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if payload != nil {
		req = httptest.NewRequest(method, path, newJSONRequest(t, method, path, payload))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-API-Key", siteKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func sendStripeEvent(t *testing.T, db *database.DB, r *gin.Engine, payload map[string]interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id FROM product_connections LIMIT 1`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	if _, ok := object["metadata"]; !ok {
		object["metadata"] = map[string]interface{}{"dtr_product_id": productID}
	}
	return sendScopedStripeEvent(t, r, productID, payload, testStripeSecret)
}

func sendScopedStripeEvent(t *testing.T, r *gin.Engine, productID string, payload map[string]interface{}, secret string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe/"+productID, strings.NewReader(string(body)))
	req.Header.Set("Stripe-Signature", "t="+timestamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	out := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func stripeSalePayload(eventID, paymentIntent, ref string, amountCents int) map[string]interface{} {
	return map[string]interface{}{
		"id":   eventID,
		"type": "checkout.session.completed",
		// Two hours after the click: an organic purchase, clear of the
		// fast-click-to-purchase fraud window.
		"created": time.Now().Add(2 * time.Hour).Unix(),
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                  "cs_" + eventID,
				"amount_total":        amountCents,
				"payment_status":      "paid",
				"currency":            "usd",
				"client_reference_id": ref,
				"payment_intent":      paymentIntent,
			},
		},
	}
}

func TestAttributionEndToEnd(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)

	siteKey := "site-key-attribution"
	createTestSite(t, db, siteKey, "blog.example.com", "Blog")

	// Marketplace shows the listed offer.
	rec, body := doSiteJSON(t, r, http.MethodGet, "/v1/marketplace/offers", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("marketplace list: expected 200, got %d", rec.Code)
	}
	offers, _ := body["offers"].([]interface{})
	if len(offers) != 1 {
		t.Fatalf("expected 1 marketplace offer, got %d", len(offers))
	}
	if offers[0].(map[string]interface{})["joined"] != false {
		t.Fatalf("expected joined=false before join")
	}

	// Join creates the campaign with a stable tracking code.
	rec, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: expected 200, got %d: %v", rec.Code, join)
	}
	code := join["tracking_code"].(string)
	rec, join2 := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	if rec.Code != http.StatusOK || join2["tracking_code"].(string) != code {
		t.Fatalf("rejoin must keep tracking code: %v vs %v", join["tracking_code"], join2["tracking_code"])
	}

	// The tracked link logs a click and forwards with the click id in the ref param.
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	if rrec.Code != http.StatusFound {
		t.Fatalf("redirect: expected 302, got %d", rrec.Code)
	}
	location := rrec.Header().Get("Location")
	if !strings.Contains(location, "inboxpilot.example.com") || !strings.Contains(location, "via=") {
		t.Fatalf("unexpected redirect location %q", location)
	}
	clickID := location[strings.Index(location, "via=")+4:]

	var clickCount int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE id=$1`, clickID).Scan(&clickCount); err != nil || clickCount != 1 {
		t.Fatalf("expected click row for %s, err=%v count=%d", clickID, err, clickCount)
	}

	// A signed sale webhook books the conversion and a pending ledger entry.
	wrec, wbody := sendStripeEvent(t, db, r, stripeSalePayload("evt_sale_1", "pi_100", clickID, 10000))
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("webhook sale: expected recorded, got %d %v", wrec.Code, wbody)
	}
	var commission float64
	var ledgerStatus string
	err := db.Pool.QueryRow(context.Background(), `
SELECT cv.commission_amount, le.status
FROM conversions cv
JOIN ledger_entries le ON le.conversion_id = cv.id
WHERE cv.provider_event_id='evt_sale_1'`).Scan(&commission, &ledgerStatus)
	if err != nil {
		t.Fatalf("load conversion: %v", err)
	}
	if commission != 20 || ledgerStatus != "pending" {
		t.Fatalf("expected 20%% of $100 pending, got commission=%v status=%s", commission, ledgerStatus)
	}

	// Redelivery of the same event is a duplicate, not a double booking.
	wrec, wbody = sendStripeEvent(t, db, r, stripeSalePayload("evt_sale_1", "pi_100", clickID, 10000))
	if wrec.Code != http.StatusOK || wbody["status"] != "duplicate" {
		t.Fatalf("expected duplicate, got %d %v", wrec.Code, wbody)
	}

	// Refund inside the window reverses the pending commission.
	wrec, wbody = sendStripeEvent(t, db, r, map[string]interface{}{
		"id":      "evt_refund_1",
		"type":    "charge.refunded",
		"created": time.Now().Unix(),
		"data":    map[string]interface{}{"object": map[string]interface{}{"id": "ch_1", "payment_intent": "pi_100", "amount_refunded": 10000, "currency": "usd"}},
	})
	if wrec.Code != http.StatusOK || wbody["status"] != "refunded" {
		t.Fatalf("expected refunded, got %d %v", wrec.Code, wbody)
	}
	err = db.Pool.QueryRow(context.Background(), `
SELECT le.status FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
WHERE cv.provider_event_id='evt_sale_1' AND le.kind='commission'`).Scan(&ledgerStatus)
	if err != nil || ledgerStatus != "reversed" {
		t.Fatalf("expected reversed ledger entry, got %s err=%v", ledgerStatus, err)
	}
}

func TestWebhookRejectsBadSignatureAndUnattributed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := newAttributionRouter(db)
	_, makerKey := createTestMaker(t, db)
	listedOffer(t, db, r, makerKey)
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id FROM product_connections LIMIT 1`).Scan(&productID); err != nil {
		t.Fatal(err)
	}

	// Wrong signature never touches storage.
	body := []byte(`{"id":"evt_x","type":"checkout.session.completed","data":{"object":{}}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe/"+productID, strings.NewReader(string(body)))
	req.Header.Set("Stripe-Signature", "t=1,v1=bad")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad signature, got %d", rec.Code)
	}
	var stored int
	_ = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM webhook_events`).Scan(&stored)
	if stored != 0 {
		t.Fatalf("expected no stored events for bad signature, got %d", stored)
	}

	// A sale without a matching click is stored but unattributed.
	wrec, wbody := sendStripeEvent(t, db, r, stripeSalePayload("evt_lost", "pi_lost", "99999999-9999-9999-9999-999999999999", 5000))
	if wrec.Code != http.StatusOK || wbody["status"] != "unattributed" {
		t.Fatalf("expected unattributed, got %d %v", wrec.Code, wbody)
	}
	var errText string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(error,'') FROM webhook_events WHERE provider_event_id='evt_lost'`).Scan(&errText); err != nil {
		t.Fatalf("load event: %v", err)
	}
	if !strings.Contains(errText, "unattributed") {
		t.Fatalf("expected unattributed error note, got %q", errText)
	}

	// Unconfigured provider degrades off.
	req = httptest.NewRequest(http.MethodPost, "/v1/webhooks/polar", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unconfigured provider, got %d", rec.Code)
	}
}

func TestRedirectStopsAttributingWhenPaused(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)

	siteKey := "site-key-paused"
	createTestSite(t, db, siteKey, "blog2.example.com", "Blog 2")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	rec, _ := doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{"status": "paused"})
	if rec.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	if rrec.Code != http.StatusGone || rrec.Header().Get("Location") != "" || !strings.Contains(rrec.Body.String(), "paused") {
		t.Fatalf("expected 410 for paused offer with no redirect, got %d %q %s", rrec.Code, rrec.Header().Get("Location"), rrec.Body.String())
	}
	var clicks int
	_ = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks`).Scan(&clicks)
	if clicks != 0 {
		t.Fatalf("expected no clicks logged while paused, got %d", clicks)
	}

	// Unknown code 404s.
	req = httptest.NewRequest(http.MethodGet, "/v1/go/doesnotexist", nil)
	rrec = httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	if rrec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown code, got %d", rrec.Code)
	}
}

func TestRedirectRefusesPausedUnlistedAndDeleted(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-unavailable"
	createTestSite(t, db, siteKey, "gone.example.com", "Gone Blog")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	rec, _ := doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{"status": "paused"})
	if rec.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d", rec.Code)
	}
	t.Run("paused", func(t *testing.T) {
		assertUnavailable(t, r, db, "/v1/go/"+code, http.StatusGone, "paused")
		bot := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
		bot.Header.Set("User-Agent", "Slackbot-LinkExpanding 1.0")
		botRec := httptest.NewRecorder()
		r.ServeHTTP(botRec, bot)
		if botRec.Code != http.StatusGone || botRec.Header().Get("Location") != "" {
			t.Fatalf("preview bot must not be redirected to a paused offer, got %d %q", botRec.Code, botRec.Header().Get("Location"))
		}
	})

	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='draft' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	t.Run("unlisted", func(t *testing.T) {
		assertUnavailable(t, r, db, "/v1/go/"+code, http.StatusGone, "unlisted")
	})

	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM campaigns WHERE tracking_code=$1`, code); err != nil {
		t.Fatal(err)
	}
	t.Run("deleted", func(t *testing.T) {
		assertUnavailable(t, r, db, "/v1/go/"+code, http.StatusNotFound, "")
	})
}

func assertUnavailable(t *testing.T, r *gin.Engine, db *database.DB, path string, want int, reason string) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != want {
		t.Fatalf("%s: expected %d, got %d %s", path, want, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("%s: must not redirect, got %q", path, loc)
	}
	if reason != "" && !strings.Contains(rec.Body.String(), reason) {
		t.Fatalf("%s: body %s missing reason %q", path, rec.Body.String(), reason)
	}
	var clicks int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks`).Scan(&clicks); err != nil {
		t.Fatal(err)
	}
	if clicks != 0 {
		t.Fatalf("%s: expected no clicks, got %d", path, clicks)
	}
}

func TestDestinationURLsRejectHTTP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	t.Run("create website", func(t *testing.T) {
		rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
			"name":             "Insecure",
			"website_url":      "http://insecure.example.com",
			"product_type":     "saas",
			"payment_provider": "stripe",
		})
		if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(body["error"]), "https") {
			t.Fatalf("http website_url must be rejected with an https error, got %d %v", rec.Code, body)
		}
	})

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             "InboxPilot",
		"website_url":      "https://inboxpilot.example.com",
		"docs_url":         "http://docs.inboxpilot.example.com",
		"product_type":     "saas",
		"payment_provider": "stripe",
	})
	if product["id"] == nil {
		t.Fatal("https product with an http docs_url should still be accepted")
	}
	productID := product["id"].(string)
	t.Run("update website", func(t *testing.T) {
		rec, body := doJSON(t, r, http.MethodPatch, "/v1/maker/products/"+productID, makerKey, map[string]interface{}{
			"website_url": "http://insecure.example.com/path",
		})
		if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(body["error"]), "https") {
			t.Fatalf("http website_url update must be rejected, got %d %v", rec.Code, body)
		}
	})
	t.Run("create landing", func(t *testing.T) {
		rec, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
			"product_id":       productID,
			"commission_type":  "percentage",
			"commission_value": 20,
			"landing_url":      "http://insecure.example.com/offer",
		})
		if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(offer["error"]), "https") {
			t.Fatalf("http landing_url must be rejected with an https error, got %d %v", rec.Code, offer)
		}
	})

	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-http-dest"
	createTestSite(t, db, siteKey, "httpdest.example.com", "HTTP Dest")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='http://insecure.example.com/stored' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	t.Run("stored destination", func(t *testing.T) {
		assertUnavailable(t, r, db, "/v1/go/"+code, http.StatusBadRequest, "https")
		bot := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
		bot.Header.Set("User-Agent", "Twitterbot/1.0")
		botRec := httptest.NewRecorder()
		r.ServeHTTP(botRec, bot)
		if botRec.Code == http.StatusFound || botRec.Header().Get("Location") != "" {
			t.Fatalf("preview bot must not be sent to a non-https destination, got %d %q", botRec.Code, botRec.Header().Get("Location"))
		}
	})
}

func testConnectionCipher() *services.ConnectionCipher {
	cipher, err := services.NewConnectionCipher("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		panic(err)
	}
	return cipher
}
