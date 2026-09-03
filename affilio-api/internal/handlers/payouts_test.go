package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func newPayoutsRouter(db *database.DB) *gin.Engine {
	r := newAttributionRouter(db)
	payouts := &PayoutsHandler{DB: db}
	site := r.Group("/v1/payouts")
	site.Use(middleware.APIKeyAuth(db))
	{
		site.GET("/balance", payouts.Balance)
		site.GET("/history", payouts.History)
	}
	makerGroup := r.Group("/v1/maker/earnings")
	makerGroup.Use(middleware.MakerKeyAuth(db))
	makerGroup.GET("", payouts.MakerEarnings)
	admin := r.Group("/v1/payout-batches")
	admin.Use(middleware.AdminKeyAuth("admin-secret"))
	{
		admin.POST("/run", payouts.RunBatch)
		admin.POST("/:id/mark-paid", payouts.MarkPaid)
	}
	return r
}

func adminJSON(t *testing.T, r *gin.Engine, method, path string, payload interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
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
	if _, err := db.CreateSite(context.Background(), siteKey, "blog3.example.com", "Blog 3"); err != nil {
		t.Fatalf("create site: %v", err)
	}
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	// Click, then a $200 sale at 20% -> $40 pending commission.
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	location := rrec.Header().Get("Location")
	clickID := location[strings.Index(location, "via=")+4:]
	wrec, wbody := sendStripeEvent(t, r, stripeSalePayload("evt_payout_sale", "pi_payout", clickID, 20000))
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
