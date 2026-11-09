package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const (
	testLemonSecret = "lemon_test_secret"
	testPolarSecret = "polar_test_secret"
)

func newRefundTrustRouter(db *database.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := newMakerRouter(db)
	marketplace := &MarketplaceHandler{DB: db}
	redirects := &RedirectsHandler{DB: db}
	webhooks := &WebhooksHandler{
		DB:      db,
		Cipher:  testConnectionCipher(),
		Secrets: map[string]string{"lemonsqueezy": testLemonSecret, "polar": testPolarSecret},
	}
	payouts := &PayoutsHandler{DB: db}
	entry := &AgentEntry{DB: db}
	r.GET("/v1/go/:code", redirects.Go)
	r.POST("/v1/webhooks/:provider", webhooks.Receive)
	r.POST("/v1/webhooks/:provider/:product_id", webhooks.Receive)
	r.GET("/v1/maker/digest", entry.MakerDigest)
	site := r.Group("/v1")
	site.Use(middleware.APIKeyAuth(db, services.NewAuditLogger(db)))
	{
		site.GET("/marketplace/offers", marketplace.ListOffers)
		site.POST("/marketplace/offers/:id/join", marketplace.JoinOffer)
		site.GET("/payouts/history", payouts.History)
	}
	maker := r.Group("/v1/maker")
	maker.Use(middleware.MakerKeyAuth(db))
	maker.GET("/earnings", payouts.MakerEarnings)
	return r
}

type trustFixture struct {
	t         *testing.T
	db        *database.DB
	r         *gin.Engine
	makerKey  string
	siteKey   string
	clickID   string
	productID string
}

func newTrustFixture(t *testing.T) *trustFixture {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	_, makerKey := createTestMaker(t, db)
	r := newRefundTrustRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-refund-trust"
	createTestSite(t, db, siteKey, "refund-trust.example", "Refund Trust")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code, _ := join["tracking_code"].(string)
	if code == "" {
		t.Fatalf("join: %v", join)
	}
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id FROM product_connections LIMIT 1`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	return &trustFixture{
		t: t, db: db, r: r, makerKey: makerKey, siteKey: siteKey,
		clickID: insertCampaignClick(t, db, code), productID: productID,
	}
}

func moneyNear(got, want float64) bool {
	return math.Abs(got-want) < 0.001
}

func (fx *trustFixture) pending(objectID string) float64 {
	fx.t.Helper()
	var amount float64
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COALESCE(SUM(le.amount),0)
FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
WHERE cv.provider_object_id=$1 AND le.status='pending'`, objectID).Scan(&amount)
	if err != nil {
		fx.t.Fatal(err)
	}
	return amount
}

func (fx *trustFixture) saleRow(objectID string) (provider, trust, status, reason string, sales int) {
	fx.t.Helper()
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT cv.provider, cv.trust_level, COALESCE(cv.refund_status,''), COALESCE(cv.refund_reason,''), COUNT(*) OVER ()
FROM conversions cv
JOIN offers o ON o.id = cv.offer_id
JOIN products p ON p.id = o.product_id
JOIN makers m ON m.id = p.maker_id
WHERE cv.provider_object_id=$1 AND cv.kind='sale' AND m.api_key_hash=$2
ORDER BY cv.created_at DESC
LIMIT 1`, objectID, services.HashAPIKey(fx.makerKey)).Scan(&provider, &trust, &status, &reason, &sales)
	if err != nil {
		fx.t.Fatalf("sale %s: %v", objectID, err)
	}
	return provider, trust, status, reason, sales
}

func (fx *trustFixture) refundRow(objectID string) (provider, trust, status, reason string, n int) {
	fx.t.Helper()
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT cv.provider, cv.trust_level, COALESCE(cv.refund_status,''), COALESCE(cv.refund_reason,''), COUNT(*) OVER ()
FROM conversions cv
JOIN offers o ON o.id = cv.offer_id
JOIN products p ON p.id = o.product_id
JOIN makers m ON m.id = p.maker_id
WHERE cv.provider_object_id=$1 AND cv.kind='refund' AND m.api_key_hash=$2
ORDER BY cv.created_at DESC
LIMIT 1`, objectID, services.HashAPIKey(fx.makerKey)).Scan(&provider, &trust, &status, &reason, &n)
	if err != nil {
		fx.t.Fatalf("refund %s: %v", objectID, err)
	}
	return provider, trust, status, reason, n
}

func (fx *trustFixture) refuseGuarantee(rec *httptest.ResponseRecorder) {
	fx.t.Helper()
	if strings.Contains(strings.ToLower(rec.Body.String()), "guaranteed") {
		fx.t.Fatalf("response claims a guarantee: %s", rec.Body.String())
	}
}

func (fx *trustFixture) sell(level, objectID string) {
	fx.t.Helper()
	var rec *httptest.ResponseRecorder
	var body map[string]interface{}
	switch level {
	case "provider_verified":
		payload := stripeSalePayload("evt-sale-"+objectID, objectID, fx.clickID, 10000)
		object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
		object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID, "trust_level": "seller_reported"}
		payload["trust_level"] = "seller_reported"
		rec, body = sendScopedStripeEvent(fx.t, fx.r, fx.productID, payload, testStripeSecret)
	case "seller_signed_webhook":
		rec, body = fx.sendLemon(lemonOrder("order_created", objectID, fx.clickID, 10000, "provider_verified"))
	case "seller_reported":
		rec, body = fx.reportSale(objectID, "provider_verified")
	default:
		fx.t.Fatalf("unknown sale trust %s", level)
	}
	fx.refuseGuarantee(rec)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != level {
		fx.t.Fatalf("sale %s: %d %v", level, rec.Code, body)
	}
	provider, trust, _, _, sales := fx.saleRow(objectID)
	if trust != level || sales != 1 {
		fx.t.Fatalf("stored sale %s: provider %s trust %s rows %d", objectID, provider, trust, sales)
	}
	if level == "seller_reported" && provider != "seller_report" {
		fx.t.Fatalf("reported sale stored provider %s from the client", provider)
	}
	if !moneyNear(fx.pendingForSeller(objectID), 20) {
		fx.t.Fatalf("sale %s commission %v", objectID, fx.pendingForSeller(objectID))
	}
}

func (fx *trustFixture) refund(level, objectID, eventID string, amount int) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	switch level {
	case "provider_verified":
		payload := refundPayload(eventID, objectID, amount)
		payload["trust_level"] = "seller_reported"
		object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
		object["trust_level"] = "seller_reported"
		return sendScopedStripeEvent(fx.t, fx.r, fx.productID, payload, testStripeSecret)
	case "seller_signed_webhook":
		return fx.sendLemon(lemonOrder("order_refunded", objectID, "", 0, "provider_verified"))
	case "seller_reported":
		return fx.reportRefund(objectID, eventID, amount, "provider_verified")
	default:
		fx.t.Fatalf("unknown refund trust %s", level)
		return nil, nil
	}
}

func (fx *trustFixture) sendLemon(payload map[string]interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		fx.t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(testLemonSecret))
	mac.Write(raw)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/lemonsqueezy", strings.NewReader(string(raw)))
	req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	fx.r.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func lemonOrder(eventName, objectID, clickID string, total int, spoofTrust string) map[string]interface{} {
	custom := map[string]interface{}{}
	if clickID != "" {
		custom["dtr"] = clickID
	}
	if spoofTrust != "" {
		custom["trust_level"] = spoofTrust
	}
	return map[string]interface{}{
		"meta": map[string]interface{}{"event_name": eventName, "custom_data": custom},
		"data": map[string]interface{}{
			"id": objectID,
			"attributes": map[string]interface{}{
				"identifier": objectID,
				"total":      total,
				"currency":   "USD",
				"created_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}
}

func (fx *trustFixture) reportSale(objectID, spoofTrust string) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	return fx.reportSaleEvent("report-sale-"+objectID, objectID, 10000, spoofTrust)
}

func (fx *trustFixture) reportSaleEvent(eventID, objectID string, amount int, spoofTrust string) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	return doJSON(fx.t, fx.r, http.MethodPost, "/v1/maker/sales", fx.makerKey, map[string]interface{}{
		"click_id":      fx.clickID,
		"event_id":      eventID,
		"object_id":     objectID,
		"amount_minor":  amount,
		"currency":      "USD",
		"occurred_at":   time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		"trust_level":   spoofTrust,
		"provider":      "stripe",
		"refund_status": "applied",
	})
}

func (fx *trustFixture) reportRefund(objectID, eventID string, amount int, spoofTrust string) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	payload := map[string]interface{}{
		"event_id":      eventID,
		"object_id":     objectID,
		"currency":      "USD",
		"trust_level":   spoofTrust,
		"provider":      "stripe",
		"refund_status": "applied",
	}
	if amount >= 0 {
		payload["amount_refunded_minor"] = amount
	}
	return doJSON(fx.t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, payload)
}

func TestRefundTrustAcceptsOnlyEqualOrHigherSource(t *testing.T) {
	fx := newTrustFixture(t)
	cases := []struct {
		sale      string
		refund    string
		accept    bool
		unmatched bool
	}{
		{"provider_verified", "provider_verified", true, false},
		{"provider_verified", "seller_signed_webhook", false, true},
		{"provider_verified", "seller_reported", false, false},
		{"seller_signed_webhook", "provider_verified", false, true},
		{"seller_signed_webhook", "seller_signed_webhook", true, false},
		{"seller_signed_webhook", "seller_reported", false, false},
		{"seller_reported", "provider_verified", false, true},
		{"seller_reported", "seller_signed_webhook", false, true},
		{"seller_reported", "seller_reported", true, false},
	}
	var appliedID, heldID string
	for _, tc := range cases {
		objectID := "obj-" + strings.ReplaceAll(tc.sale, "_", "")[:8] + "-" + strings.ReplaceAll(tc.refund, "_", "")[:8]
		fx.sell(tc.sale, objectID)
		rec, body := fx.refund(tc.refund, objectID, "evt-"+objectID, 10000)
		fx.refuseGuarantee(rec)
		if tc.unmatched {
			if tc.sale == "seller_reported" && tc.refund == "provider_verified" {
				if rec.Code != http.StatusAccepted || body["status"] != "pending" {
					t.Fatalf("upgradeable report closed the stripe refund: %d %v", rec.Code, body)
				}
				var processed bool
				if err := fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='stripe' AND provider_event_id=$1`, "evt-"+objectID).Scan(&processed); err != nil || processed {
					t.Fatalf("upgradeable report marked the refund processed: %v %v", processed, err)
				}
			} else if tc.refund == "provider_verified" {
				if rec.Code != http.StatusOK || body["status"] != "rejected" {
					t.Fatalf("stripe refund matched %s: %d %v", tc.sale, rec.Code, body)
				}
			} else if rec.Code != http.StatusServiceUnavailable || body["status"] != "unmatched" {
				t.Fatalf("report satisfied %s -> %s: %d %v", tc.sale, tc.refund, rec.Code, body)
			}
			if !moneyNear(fx.pending(objectID), 20) {
				t.Fatalf("unmatched %s -> %s changed commission to %v", tc.sale, tc.refund, fx.pending(objectID))
			}
			var refunds int
			if err := fx.db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM conversions WHERE provider_object_id=$1 AND kind='refund'`, objectID).Scan(&refunds); err != nil {
				t.Fatal(err)
			}
			if refunds != 0 {
				t.Fatalf("signed refund stored %d rows against a report", refunds)
			}
			continue
		}
		if tc.accept {
			if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || body["trust_level"] != tc.refund || body["reason"] != nil && body["reason"] != "" {
				t.Fatalf("accept %s -> %s: %d %v", tc.sale, tc.refund, rec.Code, body)
			}
			if !moneyNear(fx.pending(objectID), 0) {
				t.Fatalf("accept %s -> %s left commission %v", tc.sale, tc.refund, fx.pending(objectID))
			}
			appliedID = objectID
		} else {
			if rec.Code != http.StatusOK || body["status"] != "pending_review" || body["refund_status"] != "pending_review" || body["trust_level"] != tc.refund || body["reason"] != "refund_trust_below_sale" {
				t.Fatalf("hold %s -> %s: %d %v", tc.sale, tc.refund, rec.Code, body)
			}
			if !moneyNear(fx.pending(objectID), 20) {
				t.Fatalf("hold %s -> %s changed commission to %v", tc.sale, tc.refund, fx.pending(objectID))
			}
			heldID = objectID
		}
		provider, trust, status, reason, n := fx.refundRow(objectID)
		if trust != tc.refund || n != 1 {
			t.Fatalf("stored refund %s: provider %s trust %s status %s reason %s rows %d", objectID, provider, trust, status, reason, n)
		}
		if tc.refund == "seller_reported" && provider != "seller_report" {
			t.Fatalf("reported refund stored provider %s", provider)
		}
		if !tc.accept && (status != "pending_review" || reason != "refund_trust_below_sale") {
			t.Fatalf("held row %s status %s reason %s", objectID, status, reason)
		}
		if tc.accept && status != "applied" {
			t.Fatalf("applied row %s status %s", objectID, status)
		}
	}

	historyRec, history := doSiteJSON(t, fx.r, http.MethodGet, "/v1/payouts/history", fx.siteKey, nil)
	fx.refuseGuarantee(historyRec)
	applied := findReportedConversion(t, history, appliedID, "refund")
	held := findReportedConversion(t, history, heldID, "refund")
	if applied["trust_level"] == "" || applied["refund_status"] != "applied" {
		t.Fatalf("history applied: %v", applied)
	}
	if held["trust_level"] == "" || held["refund_status"] != "pending_review" || held["reason"] != "refund_trust_below_sale" {
		t.Fatalf("history held: %v", held)
	}
	if !ledgerHasTrust(history) {
		t.Fatalf("ledger entries omit trust: %v", history["ledger"])
	}

	earnRec, earnings := doJSON(t, fx.r, http.MethodGet, "/v1/maker/earnings", fx.makerKey, nil)
	fx.refuseGuarantee(earnRec)
	var sales, refunds, revenue, commission float64
	for _, raw := range earnings["offers"].([]interface{}) {
		row := raw.(map[string]interface{})
		sales += row["sales"].(float64)
		refunds += row["refunds"].(float64)
		revenue += row["revenue"].(float64)
		commission += row["commission"].(float64)
	}
	if sales != 9 || refunds != 3 || !moneyNear(revenue, 600) || !moneyNear(commission, 120) {
		t.Fatalf("earnings counted held refunds: sales %v refunds %v revenue %v commission %v", sales, refunds, revenue, commission)
	}
	if findReportedConversion(t, earnings, heldID, "refund")["refund_status"] != "pending_review" {
		t.Fatal("earnings omitted the held refund")
	}

	digestRec, digest := doJSON(t, fx.r, http.MethodGet, "/v1/maker/digest", fx.makerKey, nil)
	fx.refuseGuarantee(digestRec)
	if digestRec.Code != http.StatusOK {
		t.Fatalf("digest: %d %v", digestRec.Code, digest)
	}
	if findReportedConversion(t, digest, appliedID, "sale")["trust_level"] == "" {
		t.Fatal("digest sale missing trust")
	}
	if findReportedConversion(t, digest, heldID, "refund")["refund_status"] != "pending_review" {
		t.Fatal("digest refund missing status")
	}
}

func ledgerHasTrust(body map[string]interface{}) bool {
	entries, _ := body["ledger"].([]interface{})
	for _, raw := range entries {
		row := raw.(map[string]interface{})
		if row["trust_level"] != nil && row["trust_level"] != "" {
			return true
		}
	}
	return false
}

func findReportedConversion(t *testing.T, body map[string]interface{}, objectID, kind string) map[string]interface{} {
	t.Helper()
	raw, ok := body["conversions"].([]interface{})
	if !ok {
		t.Fatalf("missing conversions: %v", body)
	}
	for _, item := range raw {
		row := item.(map[string]interface{})
		if row["object_id"] == objectID && row["kind"] == kind {
			if _, exists := row["trust_level"]; !exists {
				t.Fatalf("conversion omits trust: %v", row)
			}
			if _, exists := row["refund_status"]; !exists {
				t.Fatalf("conversion omits refund status: %v", row)
			}
			return row
		}
	}
	t.Fatalf("missing %s %s in %v", kind, objectID, raw)
	return nil
}

func TestRefundTrustIgnoresClientFieldAndKeepsAmountRules(t *testing.T) {
	fx := newTrustFixture(t)

	rec, body := fx.reportSale("pi_upgrade", "provider_verified")
	if rec.Code != http.StatusOK || body["trust_level"] != "seller_reported" || body["status"] != "recorded" {
		t.Fatalf("reported sale: %d %v", rec.Code, body)
	}
	provider, trust, _, _, sales := fx.saleRow("pi_upgrade")
	if provider != "seller_report" || trust != "seller_reported" || sales != 1 {
		t.Fatalf("spoofed sale stored as %s %s rows %d", provider, trust, sales)
	}

	payload := stripeSalePayload("evt-upgrade", "pi_upgrade", fx.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID, "trust_level": "seller_reported"}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "provider_verified" {
		t.Fatalf("verified arrival should raise trust without a second sale: %d %v", rec.Code, body)
	}
	_, trust, _, _, sales = fx.saleRow("pi_upgrade")
	if trust != "provider_verified" || sales != 1 || !moneyNear(fx.pending("pi_upgrade"), 20) {
		t.Fatalf("upgrade result trust %s rows %d pending %v", trust, sales, fx.pending("pi_upgrade"))
	}
	again, againBody := fx.reportSale("pi_upgrade", "provider_verified")
	if again.Code != http.StatusOK || againBody["status"] != "duplicate" || againBody["trust_level"] != "provider_verified" {
		t.Fatalf("later report must not lower trust: %d %v", again.Code, againBody)
	}

	rec, body = fx.reportRefund("pi_upgrade", "evt-held", 10000, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "pending_review" || body["trust_level"] != "seller_reported" || body["reason"] != "refund_trust_below_sale" {
		t.Fatalf("spoofed refund: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("pi_upgrade"), 20) {
		t.Fatalf("held refund changed commission to %v", fx.pending("pi_upgrade"))
	}
	replay, replayBody := fx.reportRefund("pi_upgrade", "evt-held", 10000, "provider_verified")
	if replay.Code != http.StatusOK || replayBody["status"] != "duplicate" || replayBody["refund_status"] != "pending_review" {
		t.Fatalf("replayed hold: %d %v", replay.Code, replayBody)
	}
	if _, _, status, _, n := fx.refundRow("pi_upgrade"); status != "pending_review" || n != 1 {
		t.Fatalf("replay inserted another refund status %s rows %d", status, n)
	}

	rec, body = fx.refund("provider_verified", "pi_upgrade", "evt-upgrade-refund", 10000)
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || body["trust_level"] != "provider_verified" {
		t.Fatalf("verified refund after hold: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("pi_upgrade"), 0) {
		t.Fatalf("verified refund pending %v", fx.pending("pi_upgrade"))
	}

	fx.sell("provider_verified", "pi_dup")
	rec, body = fx.refund("provider_verified", "pi_dup", "evt-dup", 10000)
	if rec.Code != http.StatusOK || body["status"] != "refunded" {
		t.Fatalf("first refund: %d %v", rec.Code, body)
	}
	rec, body = fx.refund("provider_verified", "pi_dup", "evt-dup", 10000)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["refund_status"] != "applied" || body["trust_level"] != "provider_verified" {
		t.Fatalf("duplicate refund: %d %v", rec.Code, body)
	}
	if _, _, _, _, n := fx.refundRow("pi_dup"); n != 1 || !moneyNear(fx.pending("pi_dup"), 0) {
		t.Fatalf("duplicate refund rows %d pending %v", n, fx.pending("pi_dup"))
	}

	fx.sell("provider_verified", "pi_over")
	rec, body = fx.refund("provider_verified", "pi_over", "evt-over", 10001)
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["refund_status"] != "rejected" || body["reason"] != "refund_exceeds_remaining" || body["trust_level"] != "provider_verified" {
		t.Fatalf("over-refund: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("pi_over"), 20) {
		t.Fatalf("over-refund changed commission to %v", fx.pending("pi_over"))
	}
	rec, body = fx.refund("provider_verified", "pi_over", "evt-over", 10001)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["refund_status"] != "rejected" {
		t.Fatalf("replayed over-refund: %d %v", rec.Code, body)
	}

	fx.sell("seller_reported", "pi_partial")
	rec, body = fx.reportRefund("pi_partial", "evt-partial-40", 4000, "seller_signed_webhook")
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || body["trust_level"] != "seller_reported" {
		t.Fatalf("partial: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("pi_partial"), 12) {
		t.Fatalf("40%% refund commission %v, want 12", fx.pending("pi_partial"))
	}
	rec, body = fx.reportRefund("pi_partial", "evt-partial-40", 4000, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || !moneyNear(fx.pending("pi_partial"), 12) {
		t.Fatalf("replayed partial: %d %v pending %v", rec.Code, body, fx.pending("pi_partial"))
	}
	rec, body = fx.reportRefund("pi_partial", "evt-partial-full", -1, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || !moneyNear(fx.pending("pi_partial"), 0) {
		t.Fatalf("full after partial: %d %v pending %v", rec.Code, body, fx.pending("pi_partial"))
	}

	fx.sell("provider_verified", "pi_odd")
	rec, body = fx.refund("provider_verified", "pi_odd", "evt-odd", 3333)
	if rec.Code != http.StatusOK || body["refund_status"] != "applied" || !moneyNear(fx.pending("pi_odd"), 13.33) {
		t.Fatalf("3333 refund: %d %v pending %v", rec.Code, body, fx.pending("pi_odd"))
	}
	fx.sell("provider_verified", "pi_then_full")
	if _, body = fx.refund("provider_verified", "pi_then_full", "evt-quarter", 2500); !moneyNear(fx.pending("pi_then_full"), 15) {
		t.Fatalf("quarter refund pending %v body %v", fx.pending("pi_then_full"), body)
	}
	if _, body = fx.refund("provider_verified", "pi_then_full", "evt-rest", 10000); body["status"] != "refunded" || !moneyNear(fx.pending("pi_then_full"), 0) {
		t.Fatalf("rest of refund: %v pending %v", body, fx.pending("pi_then_full"))
	}

	fx.sell("seller_signed_webhook", "order_signed_full")
	rec, body = fx.refund("seller_signed_webhook", "order_signed_full", "evt-signed-full", 0)
	if rec.Code != http.StatusOK || body["status"] != "refunded" || !moneyNear(fx.pending("order_signed_full"), 0) {
		t.Fatalf("signed full refund: %d %v pending %v", rec.Code, body, fx.pending("order_signed_full"))
	}

	fx.sell("provider_verified", "pi_remain")
	if _, body = fx.refund("provider_verified", "pi_remain", "evt-sixty", 6000); !moneyNear(fx.pending("pi_remain"), 8) {
		t.Fatalf("60%% pending %v", fx.pending("pi_remain"))
	}
	rec, body = fx.refund("provider_verified", "pi_remain", "evt-past", 10001)
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["reason"] != "refund_exceeds_remaining" || !moneyNear(fx.pending("pi_remain"), 8) {
		t.Fatalf("refund past remaining: %d %v pending %v", rec.Code, body, fx.pending("pi_remain"))
	}
}

func (fx *trustFixture) anotherSeller() *trustFixture {
	fx.t.Helper()
	makerKey := "maker-key-refund-b"
	if _, err := fx.db.CreateMaker(context.Background(), services.HashAPIKey(makerKey), services.KeyPrefix(makerKey), "maker-b-refund@example.com", "Maker B"); err != nil {
		fx.t.Fatal(err)
	}
	offerID := listedOffer(fx.t, fx.db, fx.r, makerKey)
	siteKey := "site-key-refund-trust-b"
	createTestSite(fx.t, fx.db, siteKey, "refund-trust-b.example", "Refund Trust B")
	_, join := doSiteJSON(fx.t, fx.r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code, _ := join["tracking_code"].(string)
	if code == "" {
		fx.t.Fatalf("join seller B: %v", join)
	}
	var productID string
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT p.id FROM products p JOIN makers m ON m.id=p.maker_id WHERE m.api_key_hash=$1 ORDER BY p.created_at DESC LIMIT 1`, services.HashAPIKey(makerKey)).Scan(&productID); err != nil {
		fx.t.Fatal(err)
	}
	return &trustFixture{
		t: fx.t, db: fx.db, r: fx.r, makerKey: makerKey, siteKey: siteKey,
		clickID: insertCampaignClick(fx.t, fx.db, code), productID: productID,
	}
}

func (fx *trustFixture) pendingForSeller(objectID string) float64 {
	fx.t.Helper()
	var amount float64
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COALESCE(SUM(le.amount),0)
FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
JOIN offers o ON o.id = cv.offer_id
JOIN products p ON p.id = o.product_id
JOIN makers m ON m.id = p.maker_id
WHERE cv.provider_object_id=$1 AND m.api_key_hash=$2 AND le.status='pending'`, objectID, services.HashAPIKey(fx.makerKey)).Scan(&amount)
	if err != nil {
		fx.t.Fatal(err)
	}
	return amount
}

func TestRefundTrustOrderStaysWithItsSeller(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()

	sellerB.sell("seller_reported", "order-1")
	sellerA.sell("seller_reported", "order-1")
	if !moneyNear(sellerB.pendingForSeller("order-1"), 20) || !moneyNear(sellerA.pendingForSeller("order-1"), 20) {
		t.Fatalf("both sellers should book order-1: A %v B %v", sellerA.pendingForSeller("order-1"), sellerB.pendingForSeller("order-1"))
	}

	partial, partialBody := sellerA.reportRefund("order-1", "evt-a-quarter", 2500, "provider_verified")
	if partial.Code != http.StatusOK || partialBody["status"] != "refunded" || partialBody["refund_status"] != "applied" {
		t.Fatalf("seller A partial: %d %v", partial.Code, partialBody)
	}
	if !moneyNear(sellerA.pendingForSeller("order-1"), 15) || !moneyNear(sellerB.pendingForSeller("order-1"), 20) {
		t.Fatalf("partial crossed sellers: A %v B %v", sellerA.pendingForSeller("order-1"), sellerB.pendingForSeller("order-1"))
	}

	quarter, quarterBody := sellerB.reportRefund("order-1", "evt-b-forty", 4000, "seller_signed_webhook")
	if quarter.Code != http.StatusOK || quarterBody["status"] != "refunded" || quarterBody["refund_status"] != "applied" || quarterBody["conversion_id"] == partialBody["conversion_id"] {
		t.Fatalf("seller B partial hit seller A: %d %v", quarter.Code, quarterBody)
	}
	if !moneyNear(sellerA.pendingForSeller("order-1"), 15) || !moneyNear(sellerB.pendingForSeller("order-1"), 12) {
		t.Fatalf("shared order totals: A %v B %v", sellerA.pendingForSeller("order-1"), sellerB.pendingForSeller("order-1"))
	}

	full, fullBody := sellerB.reportRefund("order-1", "evt-b-full", -1, "provider_verified")
	if full.Code != http.StatusOK || fullBody["status"] != "refunded" || fullBody["refund_status"] != "applied" {
		t.Fatalf("seller B full: %d %v", full.Code, fullBody)
	}
	if !moneyNear(sellerA.pendingForSeller("order-1"), 15) || !moneyNear(sellerB.pendingForSeller("order-1"), 0) {
		t.Fatalf("full refund crossed sellers: A %v B %v", sellerA.pendingForSeller("order-1"), sellerB.pendingForSeller("order-1"))
	}
}

func TestRefundTrustDuplicateEventIsPerSeller(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()
	sellerA.sell("seller_reported", "order-a")
	sellerB.sell("seller_reported", "order-b")

	first, firstBody := sellerA.reportRefund("order-a", "evt-shared", 10000, "provider_verified")
	if first.Code != http.StatusOK || firstBody["status"] != "refunded" || firstBody["conversion_id"] == "" {
		t.Fatalf("seller A refund: %d %v", first.Code, firstBody)
	}
	second, secondBody := sellerB.reportRefund("order-b", "evt-shared", 10000, "seller_signed_webhook")
	if second.Code != http.StatusOK || secondBody["status"] != "refunded" || secondBody["refund_status"] != "applied" || secondBody["trust_level"] != "seller_reported" {
		t.Fatalf("seller B refund dropped: %d %v", second.Code, secondBody)
	}
	sharedID, _ := firstBody["conversion_id"].(string)
	if secondBody["conversion_id"] == sharedID || strings.Contains(second.Body.String(), sharedID) {
		t.Fatalf("seller B response leaked seller A: %s", second.Body.String())
	}
	if secondBody["reason"] == "refund_trust_below_sale" {
		t.Fatalf("seller B inherited seller A's reason: %v", secondBody)
	}
	if !moneyNear(sellerA.pending("order-a"), 0) || !moneyNear(sellerB.pending("order-b"), 0) {
		t.Fatalf("commissions A %v B %v", sellerA.pending("order-a"), sellerB.pending("order-b"))
	}

	replay, replayBody := sellerB.reportRefund("order-b", "evt-shared", 10000, "provider_verified")
	if replay.Code != http.StatusOK || replayBody["status"] != "duplicate" || replayBody["conversion_id"] != secondBody["conversion_id"] || replayBody["refund_status"] != "applied" {
		t.Fatalf("seller B replay: %d %v", replay.Code, replayBody)
	}
	if replayBody["conversion_id"] == sharedID || strings.Contains(replay.Body.String(), sharedID) {
		t.Fatalf("replay leaked seller A: %s", replay.Body.String())
	}
}

func TestRefundTrustAppliesWhenSaleHasNoCommissionLedger(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("seller_reported", "order-no-ledger")
	if _, err := fx.db.Pool.Exec(context.Background(), `
DELETE FROM ledger_entries WHERE conversion_id IN (
  SELECT id FROM conversions WHERE provider_object_id='order-no-ledger' AND kind='sale'
)`); err != nil {
		t.Fatal(err)
	}
	rec, body := fx.reportRefund("order-no-ledger", "evt-no-ledger", 10000, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || body["trust_level"] != "seller_reported" {
		t.Fatalf("refund without ledger: %d %v", rec.Code, body)
	}
	var ledgerRows int
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
WHERE cv.provider_object_id='order-no-ledger'`).Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if ledgerRows != 0 {
		t.Fatalf("refund invented a commission reversal: %d rows", ledgerRows)
	}
	provider, trust, status, _, n := fx.refundRow("order-no-ledger")
	if provider != "seller_report" || trust != "seller_reported" || status != "applied" || n != 1 {
		t.Fatalf("stored refund provider %s trust %s status %s rows %d", provider, trust, status, n)
	}
}

func TestRefundTrustRejectsNonPositiveAmount(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("provider_verified", "order-zero")
	rec, body := doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":              "evt-zero",
		"object_id":             "order-zero",
		"currency":              "USD",
		"amount_refunded_minor": 0,
		"trust_level":           "provider_verified",
	})
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["refund_status"] != "rejected" || body["reason"] != "invalid_refund_amount" || body["trust_level"] != "seller_reported" {
		t.Fatalf("zero refund: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("order-zero"), 20) {
		t.Fatalf("zero refund changed commission to %v", fx.pending("order-zero"))
	}
	replay, replayBody := doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":              "evt-zero",
		"object_id":             "order-zero",
		"currency":              "USD",
		"amount_refunded_minor": 0,
	})
	if replay.Code != http.StatusOK || replayBody["status"] != "duplicate" || replayBody["refund_status"] != "rejected" || replayBody["reason"] != "invalid_refund_amount" {
		t.Fatalf("replayed zero refund: %d %v", replay.Code, replayBody)
	}

	fx.sell("seller_reported", "order-negative")
	rec, body = doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":              "evt-negative",
		"object_id":             "order-negative",
		"currency":              "USD",
		"amount_refunded_minor": -4,
		"trust_level":           "provider_verified",
	})
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["reason"] != "invalid_refund_amount" || body["refund_status"] != "rejected" {
		t.Fatalf("negative refund: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("order-negative"), 20) {
		t.Fatalf("negative refund changed commission to %v", fx.pending("order-negative"))
	}

	fx.sell("provider_verified", "order-zero-low")
	rec, body = fx.reportRefund("order-zero-low", "evt-zero-low", 0, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["reason"] != "invalid_refund_amount" || body["refund_status"] != "rejected" {
		t.Fatalf("low-trust zero refund: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("order-zero-low"), 20) {
		t.Fatalf("low-trust zero refund changed commission to %v", fx.pending("order-zero-low"))
	}
}

func TestRefundTrustRejectsClientSentinelAmount(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("seller_reported", "order-sentinel")
	rec, body := doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":              "evt-sentinel",
		"object_id":             "order-sentinel",
		"currency":              "USD",
		"amount_refunded_minor": -1,
		"full":                  true,
	})
	if rec.Code != http.StatusOK || body["status"] != "rejected" || body["refund_status"] != "rejected" || body["reason"] != "invalid_refund_amount" || body["trust_level"] != "seller_reported" {
		t.Fatalf("client -1: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pending("order-sentinel"), 20) {
		t.Fatalf("client -1 changed commission to %v", fx.pending("order-sentinel"))
	}

	fx.sell("seller_reported", "order-omit")
	rec, body = doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":  "evt-omit",
		"object_id": "order-omit",
		"currency":  "USD",
	})
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || !moneyNear(fx.pending("order-omit"), 0) {
		t.Fatalf("omitted amount: %d %v pending %v", rec.Code, body, fx.pending("order-omit"))
	}

	fx.sell("seller_reported", "order-full-flag")
	rec, body = doJSON(t, fx.r, http.MethodPost, "/v1/maker/refunds", fx.makerKey, map[string]interface{}{
		"event_id":  "evt-full-flag",
		"object_id": "order-full-flag",
		"currency":  "USD",
		"full":      true,
	})
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || !moneyNear(fx.pending("order-full-flag"), 0) {
		t.Fatalf("full flag: %d %v pending %v", rec.Code, body, fx.pending("order-full-flag"))
	}
}

func (fx *trustFixture) anotherClick() string {
	fx.t.Helper()
	var code string
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT cam.tracking_code FROM clicks cl JOIN campaigns cam ON cam.id=cl.campaign_id WHERE cl.id=$1`, fx.clickID).Scan(&code); err != nil {
		fx.t.Fatal(err)
	}
	return insertCampaignClick(fx.t, fx.db, code)
}

func (fx *trustFixture) saleFacts(objectID string) []map[string]interface{} {
	fx.t.Helper()
	rows, err := fx.db.Pool.Query(context.Background(), `
SELECT cv.provider, cv.trust_level, cv.currency, cv.amount, cv.click_id::text
FROM conversions cv
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
JOIN makers m ON m.id=p.maker_id
WHERE cv.provider_object_id=$1 AND cv.kind='sale' AND m.api_key_hash=$2
ORDER BY cv.created_at`, objectID, services.HashAPIKey(fx.makerKey))
	if err != nil {
		fx.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var provider, trust, currency, clickID string
		var amount float64
		if err = rows.Scan(&provider, &trust, &currency, &amount, &clickID); err != nil {
			fx.t.Fatal(err)
		}
		out = append(out, map[string]interface{}{"provider": provider, "trust": trust, "currency": currency, "amount": amount, "click_id": clickID})
	}
	if err = rows.Err(); err != nil {
		fx.t.Fatal(err)
	}
	return out
}

func TestSignedSaleUpgradesTrustOnlyWhenTheSaleMatches(t *testing.T) {
	fx := newTrustFixture(t)
	rec, body := fx.reportSale("order-match", "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_reported" {
		t.Fatalf("report: %d %v", rec.Code, body)
	}
	payload := stripeSalePayload("evt-match", "order-match", fx.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "provider_verified" {
		t.Fatalf("matching signed sale: %d %v", rec.Code, body)
	}
	facts := fx.saleFacts("order-match")
	if len(facts) != 1 || facts[0]["provider"] != "stripe" || facts[0]["trust"] != "provider_verified" || facts[0]["click_id"] != fx.clickID || facts[0]["currency"] != "USD" || !moneyNear(facts[0]["amount"].(float64), 100) {
		t.Fatalf("matching sale was copied instead of upgraded: %v", facts)
	}
	if !moneyNear(fx.pendingForSeller("order-match"), 20) {
		t.Fatalf("upgrade booked a second commission: %v", fx.pendingForSeller("order-match"))
	}
}

func TestSignedSaleWithDifferentClickCurrencyOrAmountStaysSeparate(t *testing.T) {
	fx := newTrustFixture(t)

	fx.reportSale("order-amount", "provider_verified")
	payload := stripeSalePayload("evt-amount", "order-amount", fx.clickID, 4000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body := sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "provider_verified" {
		t.Fatalf("different amount: %d %v", rec.Code, body)
	}
	var reportedTrust, signedTrust string
	var reportedAmount, signedAmount float64
	for _, row := range fx.saleFacts("order-amount") {
		switch row["provider"] {
		case "seller_report":
			reportedTrust = row["trust"].(string)
			reportedAmount = row["amount"].(float64)
		case "stripe":
			signedTrust = row["trust"].(string)
			signedAmount = row["amount"].(float64)
		}
	}
	if reportedTrust != "seller_reported" || !moneyNear(reportedAmount, 100) || signedTrust != "provider_verified" || !moneyNear(signedAmount, 40) {
		t.Fatalf("amount mismatch upgraded the report: %v", fx.saleFacts("order-amount"))
	}
	if !moneyNear(fx.pendingForSeller("order-amount"), 28) {
		t.Fatalf("commissions %v", fx.pendingForSeller("order-amount"))
	}

	fx.reportSale("order-click", "provider_verified")
	otherClick := fx.anotherClick()
	payload = stripeSalePayload("evt-click", "order-click", otherClick, 10000)
	object = payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "provider_verified" {
		t.Fatalf("different click: %d %v", rec.Code, body)
	}
	facts := fx.saleFacts("order-click")
	if len(facts) != 2 {
		t.Fatalf("different click collapsed the sales: %v", facts)
	}
	for _, row := range facts {
		if row["provider"] == "seller_report" && (row["trust"] != "seller_reported" || row["click_id"] != fx.clickID) {
			t.Fatalf("reported row changed: %v", facts)
		}
		if row["provider"] == "stripe" && (row["trust"] != "provider_verified" || row["click_id"] != otherClick) {
			t.Fatalf("signed row: %v", facts)
		}
	}

	fx.reportSale("order-currency", "provider_verified")
	lemon := lemonOrder("order_created", "order-currency", fx.clickID, 10000, "provider_verified")
	lemon["data"].(map[string]interface{})["attributes"].(map[string]interface{})["currency"] = "EUR"
	rec, body = fx.sendLemon(lemon)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("different currency: %d %v", rec.Code, body)
	}
	facts = fx.saleFacts("order-currency")
	if len(facts) != 2 {
		t.Fatalf("different currency collapsed the sales: %v", facts)
	}
	for _, row := range facts {
		if row["provider"] == "seller_report" && (row["trust"] != "seller_reported" || row["currency"] != "USD") {
			t.Fatalf("reported currency row changed: %v", facts)
		}
		if row["provider"] == "lemonsqueezy" && (row["trust"] != "seller_signed_webhook" || row["currency"] != "EUR") {
			t.Fatalf("signed currency row: %v", facts)
		}
	}
}

func TestSignedRefundDoesNotUseAnotherSellersReport(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()
	sellerB.sell("seller_reported", "order-other-only")

	rec, body := sellerA.sendLemon(lemonOrder("order_refunded", "order-other-only", "", 0, "provider_verified"))
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "unmatched" {
		t.Fatalf("report satisfied a signed refund: %d %v", rec.Code, body)
	}
	if !moneyNear(sellerB.pendingForSeller("order-other-only"), 20) {
		t.Fatalf("other seller commission %v", sellerB.pendingForSeller("order-other-only"))
	}
	var refunds int
	if err := sellerA.db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM conversions WHERE provider_object_id='order-other-only' AND kind='refund'`).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	if refunds != 0 {
		t.Fatalf("signed refund stored %d rows against a report", refunds)
	}
}

func TestSignedRefundAppliesWhenAnotherSellerReportedTheSameOrder(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()
	sellerA.sell("seller_signed_webhook", "order-both")
	sellerB.sell("seller_reported", "order-both")

	rec, body := sellerA.sendLemon(lemonOrder("order_refunded", "order-both", "", 0, "seller_reported"))
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("signed refund blocked by the other report: %d %v", rec.Code, body)
	}
	if !moneyNear(sellerA.pendingForSeller("order-both"), 0) || !moneyNear(sellerB.pendingForSeller("order-both"), 20) {
		t.Fatalf("refund crossed sellers: signed %v reported %v", sellerA.pendingForSeller("order-both"), sellerB.pendingForSeller("order-both"))
	}
	reported := sellerB.saleFacts("order-both")
	if len(reported) != 1 || reported[0]["trust"] != "seller_reported" || reported[0]["provider"] != "seller_report" {
		t.Fatalf("reported sale changed: %v", reported)
	}
}

func TestFlaggedOrderDoesNotHoldAnotherSellersCommission(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()
	sellerA.sell("seller_reported", "order-1")
	sellerB.sell("seller_reported", "order-1")
	tag, err := sellerA.db.Pool.Exec(context.Background(), `
UPDATE conversions cv SET flagged=true, flag_reason='held'
FROM offers o, products p, makers m
WHERE cv.offer_id=o.id AND o.product_id=p.id AND p.maker_id=m.id
AND cv.provider_object_id='order-1' AND cv.kind='sale' AND m.api_key_hash=$1`, services.HashAPIKey(sellerA.makerKey))
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("flag sale: %v rows %d", err, tag.RowsAffected())
	}
	if _, err = sellerA.db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at=NOW() - INTERVAL '1 day'`); err != nil {
		t.Fatal(err)
	}
	if _, err = sellerA.db.ClearMaturedLedgerEntries(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := sellerA.commissionStatus("order-1"); status != "pending" {
		t.Fatalf("flagged commission cleared: %s", status)
	}
	if status := sellerB.commissionStatus("order-1"); status != "cleared" {
		t.Fatalf("other seller stayed %s", status)
	}
}

func (fx *trustFixture) commissionStatus(objectID string) string {
	fx.t.Helper()
	var status string
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT le.status FROM ledger_entries le
JOIN conversions cv ON cv.id=le.conversion_id
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
JOIN makers m ON m.id=p.maker_id
WHERE cv.provider_object_id=$1 AND cv.kind='sale' AND le.kind='commission' AND m.api_key_hash=$2`, objectID, services.HashAPIKey(fx.makerKey)).Scan(&status)
	if err != nil {
		fx.t.Fatal(err)
	}
	return status
}

func TestRefundFollowsTheSaleItMatched(t *testing.T) {
	fx := newTrustFixture(t)
	fx.pairSales("order-report-refund")
	rec, body := fx.reportRefund("order-report-refund", "evt-report-full", -1, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" {
		t.Fatalf("reported full refund: %d %v", rec.Code, body)
	}
	reportStatus, reportAmount := fx.commissionLedger("order-report-refund", "seller_report")
	signedStatus, signedAmount := fx.commissionLedger("order-report-refund", "stripe")
	if reportStatus != "reversed" || !moneyNear(reportAmount, 20) {
		t.Fatalf("reported commission: %s %v", reportStatus, reportAmount)
	}
	if signedStatus != "pending" || !moneyNear(signedAmount, 20) {
		t.Fatalf("reported refund changed the signed sale: %s %v", signedStatus, signedAmount)
	}
	if clawbacks := fx.clawbacks("order-report-refund"); clawbacks != 0 {
		t.Fatalf("reported refund inserted %d clawbacks", clawbacks)
	}
	if got, want := fx.refundSaleID("order-report-refund", "seller_report"), fx.saleID("order-report-refund", "seller_report"); got != want {
		t.Fatalf("reported refund linked %s, sale %s", got, want)
	}

	fx.pairSales("order-signed-refund")
	rec, body = fx.refund("provider_verified", "order-signed-refund", "evt-signed-full", 10000)
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" {
		t.Fatalf("signed full refund: %d %v", rec.Code, body)
	}
	reportStatus, reportAmount = fx.commissionLedger("order-signed-refund", "seller_report")
	signedStatus, signedAmount = fx.commissionLedger("order-signed-refund", "stripe")
	if signedStatus != "reversed" || !moneyNear(signedAmount, 20) {
		t.Fatalf("signed commission: %s %v", signedStatus, signedAmount)
	}
	if reportStatus != "pending" || !moneyNear(reportAmount, 20) {
		t.Fatalf("signed refund counted against the reported sale: %s %v", reportStatus, reportAmount)
	}
	if got, want := fx.refundSaleID("order-signed-refund", "stripe"), fx.saleID("order-signed-refund", "stripe"); got != want {
		t.Fatalf("signed refund linked %s, sale %s", got, want)
	}
	rec, body = fx.reportRefund("order-signed-refund", "evt-report-after-signed", -1, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" {
		t.Fatalf("reported refund after signed refund: %d %v", rec.Code, body)
	}
	reportStatus, _ = fx.commissionLedger("order-signed-refund", "seller_report")
	signedStatus, _ = fx.commissionLedger("order-signed-refund", "stripe")
	if reportStatus != "reversed" || signedStatus != "reversed" {
		t.Fatalf("after both refunds report %s signed %s", reportStatus, signedStatus)
	}
	var reportedRefund float64
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT amount FROM conversions WHERE provider_object_id='order-signed-refund' AND kind='refund' AND provider='seller_report'`).Scan(&reportedRefund); err != nil {
		t.Fatal(err)
	}
	if !moneyNear(reportedRefund, 100) {
		t.Fatalf("reported refund amount %v", reportedRefund)
	}
	if clawbacks := fx.clawbacks("order-signed-refund"); clawbacks != 0 {
		t.Fatalf("signed refund inserted %d clawbacks", clawbacks)
	}
}

func (fx *trustFixture) pairSales(objectID string) {
	fx.t.Helper()
	fx.reportSale(objectID, "provider_verified")
	other := fx.anotherClick()
	payload := stripeSalePayload("evt-sale-"+objectID, objectID, other, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body := sendScopedStripeEvent(fx.t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "provider_verified" {
		fx.t.Fatalf("signed twin %s: %d %v", objectID, rec.Code, body)
	}
	var offers int
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COUNT(DISTINCT offer_id) FROM conversions WHERE provider_object_id=$1 AND kind='sale'`, objectID).Scan(&offers); err != nil {
		fx.t.Fatal(err)
	}
	if offers != 1 || len(fx.saleFacts(objectID)) != 2 {
		fx.t.Fatalf("pair %s offers %d sales %v", objectID, offers, fx.saleFacts(objectID))
	}
}

func (fx *trustFixture) commissionLedger(objectID, provider string) (status string, amount float64) {
	fx.t.Helper()
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT le.status, le.amount
FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
WHERE cv.provider_object_id=$1 AND cv.kind='sale' AND cv.provider=$2 AND le.kind='commission'`, objectID, provider).Scan(&status, &amount)
	if err != nil {
		fx.t.Fatalf("ledger %s %s: %v", objectID, provider, err)
	}
	return status, amount
}

func (fx *trustFixture) saleID(objectID, provider string) string {
	fx.t.Helper()
	var id string
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT id::text FROM conversions WHERE provider_object_id=$1 AND kind='sale' AND provider=$2`, objectID, provider).Scan(&id)
	if err != nil {
		fx.t.Fatalf("sale id %s %s: %v", objectID, provider, err)
	}
	return id
}

func (fx *trustFixture) refundSaleID(objectID, provider string) string {
	fx.t.Helper()
	var id string
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT refunded_conversion_id::text FROM conversions WHERE provider_object_id=$1 AND kind='refund' AND provider=$2`, objectID, provider).Scan(&id)
	if err != nil {
		fx.t.Fatalf("refund link %s %s: %v", objectID, provider, err)
	}
	return id
}

func (fx *trustFixture) clawbacks(objectID string) int {
	fx.t.Helper()
	var n int
	err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM ledger_entries le
JOIN conversions cv ON cv.id = le.conversion_id
WHERE cv.provider_object_id=$1 AND le.kind='clawback'`, objectID).Scan(&n)
	if err != nil {
		fx.t.Fatal(err)
	}
	return n
}

func TestStripeRefundMatchesOnlyItsOwnSale(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("seller_signed_webhook", "order-stripe-of-lemon")
	rec, body := fx.refund("provider_verified", "order-stripe-of-lemon", "evt-stripe-of-lemon", 10000)
	if rec.Code != http.StatusOK || body["status"] != "rejected" {
		t.Fatalf("stripe refund matched the lemon sale: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pendingForSeller("order-stripe-of-lemon"), 20) {
		t.Fatalf("stripe refund reversed the lemon sale: %v", fx.pendingForSeller("order-stripe-of-lemon"))
	}
	makerID, err := fx.db.MakerIDForProduct(context.Background(), fx.productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fx.db.RecordPartialRefund(context.Background(), "stripe", "evt-stripe-of-lemon-direct", "order-stripe-of-lemon", 0, time.Now().Add(2*time.Hour), database.TrustProviderVerified, makerID, "", "USD", true)
	if !errors.Is(err, database.ErrConversionNotFound) {
		t.Fatalf("sale match accepted another provider: %v", err)
	}
	if !moneyNear(fx.pendingForSeller("order-stripe-of-lemon"), 20) {
		t.Fatalf("direct stripe refund reversed the lemon sale: %v", fx.pendingForSeller("order-stripe-of-lemon"))
	}

	fx.sell("seller_reported", "order-stripe-of-report")
	rec, body = fx.refund("provider_verified", "order-stripe-of-report", "evt-stripe-of-report", 10000)
	if rec.Code != http.StatusAccepted || body["status"] != "pending" {
		t.Fatalf("seller report closed the stripe refund: %d %v", rec.Code, body)
	}
	var reportProcessed bool
	if err = fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='stripe' AND provider_event_id='evt-stripe-of-report'`).Scan(&reportProcessed); err != nil || reportProcessed {
		t.Fatalf("seller report marked the refund processed: %v %v", reportProcessed, err)
	}
	if !moneyNear(fx.pendingForSeller("order-stripe-of-report"), 20) {
		t.Fatalf("stripe refund reversed the seller report: %v", fx.pendingForSeller("order-stripe-of-report"))
	}
	_, err = fx.db.RecordPartialRefund(context.Background(), "stripe", "evt-stripe-of-report-direct", "order-stripe-of-report", 0, time.Now().Add(2*time.Hour), database.TrustProviderVerified, makerID, "", "USD", true)
	if !errors.Is(err, database.ErrConversionNotFound) {
		t.Fatalf("sale match accepted a seller report: %v", err)
	}
	if !moneyNear(fx.pendingForSeller("order-stripe-of-report"), 20) {
		t.Fatalf("direct stripe refund reversed the seller report: %v", fx.pendingForSeller("order-stripe-of-report"))
	}
}

func TestStripeRefundWaitsForAnUpgradeableReport(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("seller_reported", "order-upgrade-later")
	rec, body := fx.refund("provider_verified", "order-upgrade-later", "evt-upgrade-later", 10000)
	if rec.Code != http.StatusAccepted || body["status"] != "pending" {
		t.Fatalf("seller report closed the stripe refund: %d %v", rec.Code, body)
	}
	var processed bool
	if err := fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='stripe' AND provider_event_id='evt-upgrade-later'`).Scan(&processed); err != nil || processed {
		t.Fatalf("refund marked processed: processed %v err %v", processed, err)
	}
	if !moneyNear(fx.pendingForSeller("order-upgrade-later"), 20) {
		t.Fatalf("pending refund reversed the report: %v", fx.pendingForSeller("order-upgrade-later"))
	}
	payload := stripeSalePayload("evt-upgrade-later-sale", "order-upgrade-later", fx.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "provider_verified" {
		t.Fatalf("stripe sale did not upgrade the report: %d %v", rec.Code, body)
	}
	if _, err := fx.db.Pool.Exec(context.Background(), `UPDATE webhook_events SET next_attempt_at=NOW() WHERE provider_event_id='evt-upgrade-later'`); err != nil {
		t.Fatal(err)
	}
	if err := (&WebhooksHandler{DB: fx.db}).ReconcilePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !moneyNear(fx.pendingForSeller("order-upgrade-later"), 0) {
		t.Fatalf("refund after upgrade left commission %v", fx.pendingForSeller("order-upgrade-later"))
	}
	provider, _, _, _, sales := fx.saleRow("order-upgrade-later")
	if provider != "stripe" || sales != 1 {
		t.Fatalf("upgraded sale provider %s rows %d", provider, sales)
	}
	if got, want := fx.refundSaleID("order-upgrade-later", "stripe"), fx.saleID("order-upgrade-later", "stripe"); got != want {
		t.Fatalf("refund linked %s want %s", got, want)
	}
}

func TestStripeRefundWaitsWhenAnotherSellerHasTheOrder(t *testing.T) {
	sellerA := newTrustFixture(t)
	sellerB := sellerA.anotherSeller()
	sellerB.sell("seller_signed_webhook", "order-shared-seller")
	rec, body := sellerA.refund("provider_verified", "order-shared-seller", "evt-shared-seller", 10000)
	if rec.Code != http.StatusAccepted || body["status"] != "pending" {
		t.Fatalf("other seller closed the stripe refund: %d %v", rec.Code, body)
	}
	var processed bool
	if err := sellerA.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='stripe' AND provider_event_id='evt-shared-seller'`).Scan(&processed); err != nil || processed {
		t.Fatalf("refund marked processed: processed %v err %v", processed, err)
	}
	if !moneyNear(sellerB.pendingForSeller("order-shared-seller"), 20) {
		t.Fatalf("other seller commission %v", sellerB.pendingForSeller("order-shared-seller"))
	}
	payload := stripeSalePayload("evt-shared-seller-sale", "order-shared-seller", sellerA.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": sellerA.productID}
	rec, body = sendScopedStripeEvent(t, sellerA.r, sellerA.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "provider_verified" {
		t.Fatalf("stripe sale: %d %v", rec.Code, body)
	}
	if !moneyNear(sellerA.pendingForSeller("order-shared-seller"), 20) {
		t.Fatalf("seller A commission before refund replay %v", sellerA.pendingForSeller("order-shared-seller"))
	}
	if _, err := sellerA.db.Pool.Exec(context.Background(), `UPDATE webhook_events SET next_attempt_at=NOW() WHERE provider_event_id='evt-shared-seller'`); err != nil {
		t.Fatal(err)
	}
	if err := (&WebhooksHandler{DB: sellerA.db}).ReconcilePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !moneyNear(sellerA.pendingForSeller("order-shared-seller"), 0) {
		t.Fatalf("seller A commission after refund %v", sellerA.pendingForSeller("order-shared-seller"))
	}
	if !moneyNear(sellerB.pendingForSeller("order-shared-seller"), 20) {
		t.Fatalf("other seller commission changed: %v", sellerB.pendingForSeller("order-shared-seller"))
	}
	if got, want := sellerA.refundSaleID("order-shared-seller", "stripe"), sellerA.saleID("order-shared-seller", "stripe"); got != want {
		t.Fatalf("refund linked %s want %s", got, want)
	}
}

func TestSignedRefundCurrencyMismatchIsRejected(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sell("seller_signed_webhook", "order-cur-mismatch")
	payload := lemonOrder("order_refunded", "order-cur-mismatch", "", 0, "provider_verified")
	payload["data"].(map[string]interface{})["attributes"].(map[string]interface{})["currency"] = "EUR"
	rec, body := fx.sendLemon(payload)
	if rec.Code != http.StatusOK || body["status"] != "rejected" {
		t.Fatalf("lemon currency mismatch stayed retryable: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pendingForSeller("order-cur-mismatch"), 20) {
		t.Fatalf("lemon currency mismatch moved commission: %v", fx.pendingForSeller("order-cur-mismatch"))
	}
	var processed bool
	if err := fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='lemonsqueezy' AND provider_event_id='order_refunded:order-cur-mismatch'`).Scan(&processed); err != nil || !processed {
		t.Fatalf("lemon currency mismatch was not marked processed: %v %v", processed, err)
	}
	replay, replayBody := fx.sendLemon(payload)
	if replay.Code != http.StatusOK || replayBody["status"] == "unmatched" {
		t.Fatalf("lemon currency mismatch was retried: %d %v", replay.Code, replayBody)
	}
	if !moneyNear(fx.pendingForSeller("order-cur-mismatch"), 20) {
		t.Fatalf("lemon replay moved commission: %v", fx.pendingForSeller("order-cur-mismatch"))
	}

	fx.sendPolarSale("order-polar-cur", fx.clickID, "USD", 10000)
	rec, body = fx.sendPolar("order.refunded", "order-polar-cur", "", "EUR", 0)
	if rec.Code != http.StatusOK || body["status"] != "rejected" {
		t.Fatalf("polar currency mismatch stayed retryable: %d %v", rec.Code, body)
	}
	status, amount := fx.commissionLedger("order-polar-cur", "polar")
	if status != "pending" || !moneyNear(amount, 20) {
		t.Fatalf("polar currency mismatch moved commission: %s %v", status, amount)
	}
	if err := fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='polar' AND provider_event_id='order.refunded:order-polar-cur'`).Scan(&processed); err != nil || !processed {
		t.Fatalf("polar currency mismatch was not marked processed: %v %v", processed, err)
	}

	early := lemonOrder("order_refunded", "order-not-yet", "", 0, "")
	rec, body = fx.sendLemon(early)
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "unmatched" {
		t.Fatalf("refund before sale: %d %v", rec.Code, body)
	}
	var earlyProcessed bool
	if err := fx.db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider='lemonsqueezy' AND provider_event_id='order_refunded:order-not-yet'`).Scan(&earlyProcessed); err != nil || earlyProcessed {
		t.Fatalf("early refund was closed: %v %v", earlyProcessed, err)
	}
}

func TestSignedUpgradeLeavesAnotherProvidersSale(t *testing.T) {
	fx := newTrustFixture(t)
	rec, body := fx.sendLemon(lemonOrder("order_created", "order-lemon-stays", fx.clickID, 10000, "provider_verified"))
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("lemon sale: %d %v", rec.Code, body)
	}
	payload := stripeSalePayload("evt-lemon-stays", "order-lemon-stays", fx.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("stripe rewrote the lemon sale: %d %v", rec.Code, body)
	}
	facts := fx.saleFacts("order-lemon-stays")
	if len(facts) != 1 || facts[0]["provider"] != "lemonsqueezy" || facts[0]["trust"] != "seller_signed_webhook" {
		t.Fatalf("lemon row changed: %v", facts)
	}
	var eventID string
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT provider_event_id FROM conversions WHERE provider_object_id='order-lemon-stays' AND kind='sale'`).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if eventID != "order_created:order-lemon-stays" {
		t.Fatalf("lemon event id became %s", eventID)
	}
	if !moneyNear(fx.pendingForSeller("order-lemon-stays"), 20) {
		t.Fatalf("rewrite booked commission %v", fx.pendingForSeller("order-lemon-stays"))
	}
	rec, body = fx.refund("provider_verified", "order-lemon-stays", "evt-lemon-stays-refund", 10000)
	if rec.Code != http.StatusOK || body["status"] != "rejected" {
		t.Fatalf("stripe refund of the untouched lemon sale: %d %v", rec.Code, body)
	}
	if !moneyNear(fx.pendingForSeller("order-lemon-stays"), 20) {
		t.Fatalf("stripe refund reversed the lemon sale: %v", fx.pendingForSeller("order-lemon-stays"))
	}
	rec, body = fx.sendLemon(lemonOrder("order_refunded", "order-lemon-stays", "", 0, "provider_verified"))
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || !moneyNear(fx.pendingForSeller("order-lemon-stays"), 0) {
		t.Fatalf("lemon refund after a stripe arrival: %d %v pending %v", rec.Code, body, fx.pendingForSeller("order-lemon-stays"))
	}
}

func TestUpgradedReportReservesItsEventID(t *testing.T) {
	fx := newTrustFixture(t)
	rec, body := fx.reportSaleEvent("report-evt-keep", "order-keep", 10000, "provider_verified")
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_reported" {
		t.Fatalf("report: %d %v", rec.Code, body)
	}
	payload := stripeSalePayload("evt-keep-upgrade", "order-keep", fx.clickID, 10000)
	object := payload["data"].(map[string]interface{})["object"].(map[string]interface{})
	object["metadata"] = map[string]interface{}{"dtr_product_id": fx.productID}
	rec, body = sendScopedStripeEvent(t, fx.r, fx.productID, payload, testStripeSecret)
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "provider_verified" {
		t.Fatalf("upgrade: %d %v", rec.Code, body)
	}
	replay, replayBody := fx.reportSaleEvent("report-evt-keep", "order-keep-other", 10000, "provider_verified")
	if replay.Code != http.StatusOK || replayBody["status"] != "duplicate" || replayBody["trust_level"] != "provider_verified" {
		t.Fatalf("reserved report inserted again: %d %v", replay.Code, replayBody)
	}
	if len(fx.saleFacts("order-keep-other")) != 0 {
		t.Fatalf("second object booked a sale: %v", fx.saleFacts("order-keep-other"))
	}
	if !moneyNear(fx.pendingForSeller("order-keep"), 20) {
		t.Fatalf("reserved report changed commission to %v", fx.pendingForSeller("order-keep"))
	}
	var reserved int
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM reserved_sale_events WHERE provider_event_id='report-evt-keep'`).Scan(&reserved); err != nil || reserved != 1 {
		t.Fatalf("reserved events %d err %v", reserved, err)
	}
}

func TestSignedRefundDoesNotClawAnotherProvidersSale(t *testing.T) {
	fx := newTrustFixture(t)
	fx.sendPolarSale("order-polar-only", fx.clickID, "USD", 10000)
	rec, body := fx.sendLemon(lemonOrder("order_refunded", "order-polar-only", "", 0, "provider_verified"))
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "unmatched" {
		t.Fatalf("lemon refund clawed the polar sale: %d %v", rec.Code, body)
	}
	status, amount := fx.commissionLedger("order-polar-only", "polar")
	if status != "pending" || !moneyNear(amount, 20) {
		t.Fatalf("polar commission after lemon refund: %s %v", status, amount)
	}

	fx.sendLemon(lemonOrder("order_created", "order-lemon-only", fx.clickID, 10000, "provider_verified"))
	rec, body = fx.sendPolarRefund("order-lemon-only")
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "unmatched" {
		t.Fatalf("polar refund clawed the lemon sale: %d %v", rec.Code, body)
	}
	status, amount = fx.commissionLedger("order-lemon-only", "lemonsqueezy")
	if status != "pending" || !moneyNear(amount, 20) {
		t.Fatalf("lemon commission after polar refund: %s %v", status, amount)
	}
}

func TestSellerReportRefundUsesTheMatchedSaleCurrency(t *testing.T) {
	fx := newTrustFixture(t)

	t.Run("newer usd sale", func(t *testing.T) {
		fx.reportSale("order-eur-then-usd", "provider_verified")
		if _, err := fx.db.Pool.Exec(context.Background(), `
UPDATE conversions SET currency='EUR'
WHERE provider_object_id='order-eur-then-usd' AND provider='seller_report' AND kind='sale'`); err != nil {
			t.Fatal(err)
		}
		rec, body := fx.sendLemon(lemonOrder("order_created", "order-eur-then-usd", fx.clickID, 10000, "provider_verified"))
		if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
			t.Fatalf("newer usd sale: %d %v", rec.Code, body)
		}
		rec, body = fx.reportRefund("order-eur-then-usd", "evt-usd-after-eur", -1, "provider_verified")
		eurStatus, eurAmount := fx.commissionLedger("order-eur-then-usd", "seller_report")
		usdStatus, usdAmount := fx.commissionLedger("order-eur-then-usd", "lemonsqueezy")
		if eurStatus != "pending" || !moneyNear(eurAmount, 20) {
			t.Fatalf("usd refund reversed the eur report: %s %v body %d %v", eurStatus, eurAmount, rec.Code, body)
		}
		if usdStatus != "pending" || !moneyNear(usdAmount, 20) {
			t.Fatalf("usd refund reversed the usd sale: %s %v", usdStatus, usdAmount)
		}
		if rec.Code != http.StatusOK || body["status"] != "pending_review" || body["reason"] != "refund_trust_below_sale" {
			t.Fatalf("usd refund of the usd sale: %d %v", rec.Code, body)
		}
		if got, want := fx.refundSaleID("order-eur-then-usd", "seller_report"), fx.saleID("order-eur-then-usd", "lemonsqueezy"); got != want {
			t.Fatalf("usd refund linked %s, usd sale %s", got, want)
		}
	})

	t.Run("newer other currency", func(t *testing.T) {
		fx.reportSale("order-usd-then-eur", "provider_verified")
		lemon := lemonOrder("order_created", "order-usd-then-eur", fx.clickID, 10000, "provider_verified")
		lemon["data"].(map[string]interface{})["attributes"].(map[string]interface{})["currency"] = "EUR"
		rec, body := fx.sendLemon(lemon)
		if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
			t.Fatalf("newer eur sale: %d %v", rec.Code, body)
		}
		rec, body = fx.reportRefund("order-usd-then-eur", "evt-usd-before-eur", -1, "provider_verified")
		if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" {
			t.Fatalf("usd refund of the usd report: %d %v", rec.Code, body)
		}
		usdStatus, usdAmount := fx.commissionLedger("order-usd-then-eur", "seller_report")
		eurStatus, eurAmount := fx.commissionLedger("order-usd-then-eur", "lemonsqueezy")
		if usdStatus != "reversed" || !moneyNear(usdAmount, 20) {
			t.Fatalf("usd report after its refund: %s %v", usdStatus, usdAmount)
		}
		if eurStatus != "pending" || !moneyNear(eurAmount, 20) {
			t.Fatalf("usd refund reversed the eur sale: %s %v", eurStatus, eurAmount)
		}
		if got, want := fx.refundSaleID("order-usd-then-eur", "seller_report"), fx.saleID("order-usd-then-eur", "seller_report"); got != want {
			t.Fatalf("usd refund linked %s, usd report %s", got, want)
		}
	})
}

func TestFullRefundStaysOnItsProviderAndCurrency(t *testing.T) {
	fx := newTrustFixture(t)
	fx.reportSale("order-xcur", "provider_verified")
	lemon := lemonOrder("order_created", "order-xcur", fx.clickID, 10000, "provider_verified")
	lemon["data"].(map[string]interface{})["attributes"].(map[string]interface{})["currency"] = "EUR"
	rec, body := fx.sendLemon(lemon)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("eur sale: %d %v", rec.Code, body)
	}
	var offers int
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT COUNT(DISTINCT offer_id) FROM conversions WHERE provider_object_id='order-xcur' AND kind='sale'`).Scan(&offers); err != nil {
		t.Fatal(err)
	}
	if offers != 1 {
		t.Fatalf("cross-currency sales split across %d offers", offers)
	}
	refund := lemonOrder("order_refunded", "order-xcur", "", 0, "provider_verified")
	refund["data"].(map[string]interface{})["attributes"].(map[string]interface{})["currency"] = "EUR"
	rec, body = fx.sendLemon(refund)
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" {
		t.Fatalf("eur refund: %d %v", rec.Code, body)
	}
	eurStatus, eurAmount := fx.commissionLedger("order-xcur", "lemonsqueezy")
	usdStatus, usdAmount := fx.commissionLedger("order-xcur", "seller_report")
	if eurStatus != "reversed" || !moneyNear(eurAmount, 20) {
		t.Fatalf("eur commission: %s %v", eurStatus, eurAmount)
	}
	if usdStatus != "pending" || !moneyNear(usdAmount, 20) {
		t.Fatalf("eur refund reversed the usd sale: %s %v", usdStatus, usdAmount)
	}
	if got, want := fx.refundSaleID("order-xcur", "lemonsqueezy"), fx.saleID("order-xcur", "lemonsqueezy"); got != want {
		t.Fatalf("eur refund linked %s, eur sale %s", got, want)
	}
}

func TestUpgradedSaleRefundsFromItsNewProvider(t *testing.T) {
	fx := newTrustFixture(t)
	fx.reportSale("order-upgraded", "provider_verified")
	rec, body := fx.sendLemon(lemonOrder("order_created", "order-upgraded", fx.clickID, 10000, "seller_reported"))
	if rec.Code != http.StatusOK || body["status"] != "duplicate" || body["trust_level"] != "seller_signed_webhook" {
		t.Fatalf("upgrade: %d %v", rec.Code, body)
	}
	var provider, eventID, trust string
	if err := fx.db.Pool.QueryRow(context.Background(), `
SELECT provider, provider_event_id, trust_level FROM conversions
WHERE provider_object_id='order-upgraded' AND kind='sale'`).Scan(&provider, &eventID, &trust); err != nil {
		t.Fatal(err)
	}
	if provider != "lemonsqueezy" || eventID != "order_created:order-upgraded" || trust != "seller_signed_webhook" {
		t.Fatalf("upgraded row provider %s event %s trust %s", provider, eventID, trust)
	}
	rec, body = fx.sendLemon(lemonOrder("order_refunded", "order-upgraded", "", 0, "provider_verified"))
	if rec.Code != http.StatusOK || body["status"] != "refunded" || body["refund_status"] != "applied" || !moneyNear(fx.pending("order-upgraded"), 0) {
		t.Fatalf("refund of upgraded sale: %d %v pending %v", rec.Code, body, fx.pending("order-upgraded"))
	}
}

func (fx *trustFixture) sendPolarSale(objectID, clickID, currency string, amount int) {
	fx.t.Helper()
	rec, body := fx.sendPolar("order.created", objectID, clickID, currency, amount)
	if rec.Code != http.StatusOK || body["status"] != "recorded" || body["trust_level"] != "seller_signed_webhook" {
		fx.t.Fatalf("polar sale %s: %d %v", objectID, rec.Code, body)
	}
}

func (fx *trustFixture) sendPolarRefund(objectID string) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	return fx.sendPolar("order.refunded", objectID, "", "USD", 0)
}

func (fx *trustFixture) sendPolar(eventType, objectID, clickID, currency string, amount int) (*httptest.ResponseRecorder, map[string]interface{}) {
	fx.t.Helper()
	data := map[string]interface{}{
		"id":         objectID,
		"amount":     amount,
		"currency":   currency,
		"created_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		"metadata":   map[string]string{},
	}
	if clickID != "" {
		data["metadata"] = map[string]string{"dtr": clickID}
	}
	raw, err := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	if err != nil {
		fx.t.Fatal(err)
	}
	msgID := "msg-" + eventType + "-" + objectID
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(testPolarSecret))
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(raw)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/polar", strings.NewReader(string(raw)))
	req.Header.Set("webhook-id", msgID)
	req.Header.Set("webhook-timestamp", timestamp)
	req.Header.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	fx.r.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestRefundTrustMigrationIsIdempotent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join("..", "migrations", "034_refund_trust.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	scope, err := os.ReadFile(filepath.Join("..", "migrations", "035_refund_seller_scope.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(scope)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(scope)); err != nil {
		t.Fatalf("second seller-scope apply: %v", err)
	}
	link, err := os.ReadFile(filepath.Join("..", "migrations", "036_refund_sale_link.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(link)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(link)); err != nil {
		t.Fatalf("second sale-link apply: %v", err)
	}
	reserved, err := os.ReadFile(filepath.Join("..", "migrations", "037_reserved_sale_events.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(reserved)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(reserved)); err != nil {
		t.Fatalf("second reserved-event apply: %v", err)
	}
	_, err = db.Pool.Exec(context.Background(), `
INSERT INTO conversions (provider, provider_event_id, kind, amount, currency, occurred_at, trust_level)
VALUES ('seller_report', 'evt-bad-trust', 'sale', 1, 'USD', NOW(), 'spoofed')`)
	if err == nil || !strings.Contains(err.Error(), "conversions_trust_level_check") {
		t.Fatalf("bad trust level accepted: %v", err)
	}
}
