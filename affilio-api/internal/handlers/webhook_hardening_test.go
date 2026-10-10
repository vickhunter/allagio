package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"affilio-api/internal/database"
)

func TestStripeWebhookRejectsBadOrMissingSignature(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := newAttributionRouter(db)
	_, makerKey := createTestMaker(t, db)
	listedOffer(t, db, r, makerKey)
	productID := productConnectionID(t, db)

	payload := stripeSalePayload("evt_sig_missing", "pi_sig_missing", "99999999-9999-9999-9999-999999999999", 5000)
	payload["data"].(map[string]interface{})["object"].(map[string]interface{})["metadata"] = map[string]interface{}{"dtr_product_id": productID}

	missing := postUnsignedStripe(r, productID, payload, "")
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing signature: expected 400, got %d %s", missing.Code, missing.Body.String())
	}
	bad := postUnsignedStripe(r, productID, payload, "t=1,v1=bad")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad signature: expected 400, got %d %s", bad.Code, bad.Body.String())
	}
	if n := countQuery(t, db, `SELECT COUNT(*) FROM webhook_events`); n != 0 {
		t.Fatalf("unsigned events must not be stored, got %d webhook rows", n)
	}
	if n := countQuery(t, db, `SELECT COUNT(*) FROM conversions`); n != 0 {
		t.Fatalf("unsigned events must not create conversions, got %d", n)
	}
	if n := countQuery(t, db, `SELECT COUNT(*) FROM ledger_entries`); n != 0 {
		t.Fatalf("unsigned events must not write the ledger, got %d", n)
	}
}

func TestStripeWebhookIgnoresEventsThatAreNotOurs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := newAttributionRouter(db)
	_, makerKey := createTestMaker(t, db)
	listedOffer(t, db, r, makerKey)
	productID := productConnectionID(t, db)
	otherProduct := "11111111-1111-1111-1111-111111111111"

	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	cases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{name: "missing dtr_product_id", payload: saleWithMeta("evt_no_product", "usd", map[string]interface{}{})},
		{name: "unknown product", payload: saleWithMeta("evt_other_product", "usd", map[string]interface{}{"dtr_product_id": otherProduct})},
		{name: "other currency", payload: saleWithMeta("evt_eur", "eur", map[string]interface{}{"dtr_product_id": productID})},
		{name: "unsupported currency", payload: saleWithMeta("evt_jpy", "jpy", map[string]interface{}{"dtr_product_id": productID})},
		{name: "subscription", payload: map[string]interface{}{
			"id":      "evt_sub",
			"type":    "customer.subscription.created",
			"created": 1,
			"data": map[string]interface{}{"object": map[string]interface{}{
				"id":       "sub_1",
				"currency": "usd",
				"metadata": map[string]interface{}{"dtr_product_id": productID},
			}},
		}},
		{name: "invoice", payload: map[string]interface{}{
			"id":      "evt_invoice",
			"type":    "invoice.paid",
			"created": 1,
			"data": map[string]interface{}{"object": map[string]interface{}{
				"id":       "in_1",
				"currency": "usd",
				"metadata": map[string]interface{}{"dtr_product_id": productID},
			}},
		}},
	}
	for _, tc := range cases {
		logs.Reset()
		rec, body := sendScopedStripeEvent(t, r, productID, tc.payload, testStripeSecret)
		if rec.Code < 200 || rec.Code >= 300 || body["status"] != "ignored" {
			t.Fatalf("%s: expected 2xx ignored, got %d %v", tc.name, rec.Code, body)
		}
		if !strings.Contains(strings.ToLower(logs.String()), "ignored") {
			t.Fatalf("%s: expected an ignore log, got %q", tc.name, logs.String())
		}
		if n := countQuery(t, db, `SELECT COUNT(*) FROM conversions`); n != 0 {
			t.Fatalf("%s: expected no conversions, got %d", tc.name, n)
		}
		if n := countQuery(t, db, `SELECT COUNT(*) FROM ledger_entries`); n != 0 {
			t.Fatalf("%s: expected no ledger writes, got %d", tc.name, n)
		}
	}
}

func TestStripeWebhookKeepsOurSalesAndRefunds(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-webhook-ours"
	createTestSite(t, db, siteKey, "ours.example.com", "Ours")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "via=") {
		t.Fatalf("expected attributed redirect, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	requireGoLinkHeaders(t, rec)
	clickID := rec.Header().Get("Location")
	clickID = clickID[strings.Index(clickID, "via=")+4:]

	wrec, wbody := sendStripeEvent(t, db, r, stripeSalePayload("evt_ours_sale", "pi_ours", clickID, 10000))
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("our sale: expected recorded, got %d %v", wrec.Code, wbody)
	}
	if n := countQuery(t, db, `SELECT COUNT(*) FROM ledger_entries WHERE kind='commission' AND status='pending'`); n != 1 {
		t.Fatalf("expected one pending commission, got %d", n)
	}

	wrec, wbody = sendStripeEvent(t, db, r, map[string]interface{}{
		"id":      "evt_ours_refund",
		"type":    "charge.refunded",
		"created": 1,
		"data": map[string]interface{}{"object": map[string]interface{}{
			"id": "ch_ours", "payment_intent": "pi_ours", "amount_refunded": 10000, "currency": "usd",
		}},
	})
	if wrec.Code != http.StatusOK || wbody["status"] != "refunded" {
		t.Fatalf("our refund: expected refunded, got %d %v", wrec.Code, wbody)
	}
}

func productConnectionID(t *testing.T, db *database.DB) string {
	t.Helper()
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id FROM product_connections LIMIT 1`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	return productID
}

func countQuery(t *testing.T, db *database.DB, query string) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func saleWithMeta(eventID, currency string, metadata map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":      eventID,
		"type":    "checkout.session.completed",
		"created": 1,
		"data": map[string]interface{}{"object": map[string]interface{}{
			"id":                  "cs_" + eventID,
			"amount_total":        5000,
			"payment_status":      "paid",
			"currency":            currency,
			"client_reference_id": "99999999-9999-9999-9999-999999999999",
			"payment_intent":      "pi_" + eventID,
			"metadata":            metadata,
		}},
	}
}

func postUnsignedStripe(r http.Handler, productID string, payload map[string]interface{}, signature string) *httptest.ResponseRecorder {
	body := mustJSON(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe/"+productID, strings.NewReader(body))
	if signature != "" {
		req.Header.Set("Stripe-Signature", signature)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func mustJSON(payload map[string]interface{}) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
