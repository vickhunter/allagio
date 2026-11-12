package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func newPayoutsRouter(db *database.DB) *gin.Engine {
	r := newAttributionRouter(db)
	payouts := &PayoutsHandler{DB: db}
	site := r.Group("/v1/payouts")
	site.Use(middleware.APIKeyAuth(db, services.NewAuditLogger(db)))
	{
		site.GET("/balance", payouts.Balance)
		site.GET("/history", payouts.History)
		site.POST("/:id/confirm-received", payouts.ConfirmReceived)
	}
	makerGroup := r.Group("/v1/maker/earnings")
	makerGroup.Use(middleware.MakerKeyAuth(db))
	makerGroup.GET("", payouts.MakerEarnings)
	admin := r.Group("/v1/payout-batches")
	admin.Use(middleware.AdminKeyAuth("admin-secret", services.NewAuditLogger(db)))
	{
		admin.POST("/run", payouts.RunBatch)
		admin.POST("/:id/mark-paid", payouts.MarkPaid)
	}
	return r
}

func adminJSON(t *testing.T, r *gin.Engine, method, path string, payload interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	if path == "/v1/payout-batches/run" {
		if values, ok := payload.(map[string]interface{}); ok {
			values["include_test_mode"] = true
		}
	}
	rec, body := doJSONWithHeader(t, r, method, path, "X-Admin-Key", "admin-secret", payload)
	return rec, body
}

func doJSONWithHeader(t *testing.T, r *gin.Engine, method, path, header, value string, payload interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if payload != nil {
		req = httptest.NewRequest(method, path, newJSONRequest(t, method, path, payload))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set(header, value)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return rec, body
}

func TestPayoutLifecycle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newPayoutsRouter(db)
	offerID := listedOffer(t, db, r, makerKey)

	siteKey := "site-key-payouts"
	createTestSite(t, db, siteKey, "blog3.example.com", "Blog 3")
	var siteID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM sites WHERE domain='blog3.example.com'`).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE agent_step_modes SET payout='auto' WHERE site_id=$1::uuid`, siteID); err != nil {
		t.Fatal(err)
	}
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	// Click, then a $200 sale at 20% -> $40 pending commission.
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)
	wrec, wbody := sendStripeEvent(t, db, r, stripeSalePayload("evt_payout_sale", "pi_payout", clickID, 20000))
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("sale: expected recorded, got %d %v", wrec.Code, wbody)
	}

	// Balance shows the commission pending inside the refund window.
	rec, body := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("balance: expected 200, got %d", rec.Code)
	}
	balances := body["balances"].([]interface{})
	if len(balances) != 1 {
		t.Fatalf("expected one currency balance, got %v", balances)
	}
	usd := balances[0].(map[string]interface{})
	if usd["pending"].(float64) != 40 || usd["cleared_unpaid"].(float64) != 0 {
		t.Fatalf("expected 40 pending, got %v", usd)
	}

	// A batch before the window matures pays nothing.
	rec, body = adminJSON(t, r, http.MethodPost, "/v1/payout-batches/run", map[string]interface{}{"min_payout": 10})
	if rec.Code != http.StatusOK || len(body["payouts"].([]interface{})) != 0 {
		t.Fatalf("expected empty batch inside refund window, got %d %v", rec.Code, body)
	}

	// Fast-forward the hold, then the batch pays the site.
	if _, err := db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at = NOW() - INTERVAL '1 day' WHERE status='pending'`); err != nil {
		t.Fatalf("mature ledger: %v", err)
	}
	rec, body = adminJSON(t, r, http.MethodPost, "/v1/payout-batches/run", map[string]interface{}{"min_payout": 10})
	payouts := body["payouts"].([]interface{})
	if rec.Code != http.StatusOK || len(payouts) != 1 {
		t.Fatalf("expected one payout, got %d %v", rec.Code, body)
	}
	payout := payouts[0].(map[string]interface{})
	if payout["amount"].(float64) != 40 || payout["status"] != "pending" {
		t.Fatalf("unexpected payout %v", payout)
	}

	// A second batch must not pay the same entries again.
	rec, body = adminJSON(t, r, http.MethodPost, "/v1/payout-batches/run", map[string]interface{}{"min_payout": 10})
	if rec.Code != http.StatusOK || len(body["payouts"].([]interface{})) != 0 {
		t.Fatalf("expected no double payout, got %v", body)
	}

	// Mark paid with a bank reference; balance moves to paid.
	payoutID := payout["id"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/payout-batches/"+payoutID+"/mark-paid", map[string]interface{}{"provider_reference": "SEPA-123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("mark-paid: expected 200, got %d", rec.Code)
	}
	rec, body = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usdReported := body["balances"].([]interface{})[0].(map[string]interface{})
	if usdReported["paid"] != float64(0) || usdReported["payment_reported"] != float64(40) {
		t.Fatalf("unconfirmed payout shown as paid: %v", usdReported)
	}
	rec, _ = doSiteJSON(t, r, http.MethodPost, "/v1/payouts/"+payoutID+"/confirm-received", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm receipt: %d", rec.Code)
	}
	_, body = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd = body["balances"].([]interface{})[0].(map[string]interface{})
	if usd["paid"].(float64) != 40 || usd["cleared_unpaid"].(float64) != 0 {
		t.Fatalf("expected 40 paid after payout, got %v", usd)
	}

	// History shows the payout and the ledger trail.
	rec, body = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/history", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d", rec.Code)
	}
	if len(body["payouts"].([]interface{})) != 1 || len(body["ledger"].([]interface{})) != 1 {
		t.Fatalf("expected payout + ledger history, got %v", body)
	}

	// Maker earnings reflect the demand side's activity.
	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/earnings", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("earnings: expected 200, got %d", rec.Code)
	}
	offerRows := body["offers"].([]interface{})
	if len(offerRows) != 1 {
		t.Fatalf("expected one offer in earnings, got %v", offerRows)
	}
	row := offerRows[0].(map[string]interface{})
	if row["clicks"].(float64) != 1 || row["sales"].(float64) != 1 || row["revenue"].(float64) != 200 || row["commission"].(float64) != 40 {
		t.Fatalf("unexpected earnings row %v", row)
	}
}
