package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
)

func ownerJSON(t *testing.T, h *HostedHandler, r http.Handler, cookie *http.Cookie, csrf, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, newJSONRequest(t, method, path, payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", h.BaseURL)
	req.Header.Set("X-CSRF-Token", csrf)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func linkedMaker(t *testing.T, h *HostedHandler, subject string) (*database.Owner, *http.Cookie) {
	t.Helper()
	o, cookie := testOwnerSession(t, h, subject, subject+"@example.com")
	if err := h.DB.CreateOwnerAccount(context.Background(), o, "maker", "", "", ""); err != nil {
		t.Fatal(err)
	}
	o, err := h.DB.OwnerSession(context.Background(), services.HashAPIKey(cookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	return o, cookie
}

func TestWorkspaceOwnerProductsAndCSRF(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := linkedMaker(t, h, "builder-a")
	other, otherCookie := linkedMaker(t, h, "builder-b")
	payload := map[string]any{"name": "Postgres Tool", "website_url": "https://tool.example", "tagline": "Debug postgres", "payment_provider": "stripe", "keywords": []string{"postgres"}}
	for _, bad := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{nil, o.CSRF}, {cookie, "wrong"}} {
		rec := ownerJSON(t, h, r, bad.cookie, bad.csrf, "POST", "/owner/api/products", payload)
		if rec.Code != 401 && rec.Code != 403 {
			t.Fatalf("unauthorized product: %d", rec.Code)
		}
	}
	created := ownerJSON(t, h, r, cookie, o.CSRF, "POST", "/owner/api/products", payload)
	if created.Code != 200 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var product map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &product); err != nil {
		t.Fatal(err)
	}
	id := product["id"].(string)
	if rec := ownerJSON(t, h, r, otherCookie, other.CSRF, "PATCH", "/owner/api/products/"+id, map[string]string{"name": "stolen"}); rec.Code != 404 {
		t.Fatalf("foreign update: %d", rec.Code)
	}
	if rec := ownerJSON(t, h, r, otherCookie, other.CSRF, "GET", "/owner/api/products", nil); strings.Contains(rec.Body.String(), id) {
		t.Fatal("foreign product listed")
	}
	offer := ownerJSON(t, h, r, cookie, o.CSRF, "POST", "/owner/api/offers", map[string]any{"product_id": id, "commission_type": "percentage", "commission_value": 20, "commission_currency": "USD"})
	if offer.Code != 200 {
		t.Fatalf("offer: %d %s", offer.Code, offer.Body.String())
	}
	if rec := ownerJSON(t, h, r, cookie, o.CSRF, "GET", "/owner/api/review/offers", nil); rec.Code != 403 {
		t.Fatal("ordinary owner became operator")
	}
	h.OperatorID = o.ID
	if rec := ownerJSON(t, h, r, cookie, o.CSRF, "GET", "/owner/api/review/offers", nil); rec.Code != 200 {
		t.Fatal("operator queue inaccessible")
	}
	h.AssetsPath = "../../../affilio-wp/admin/build"
	if err := h.LoadWorkspace(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/owner/products", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "workspace-app") || !strings.Contains(rec.Body.String(), "csrf-token") {
		t.Fatal("workspace not rendered")
	}
}

func TestOperatorReviewLinkFollowsOperatorOwner(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := linkedMaker(t, h, "operator-owner")
	h.AssetsPath = "../../../affilio-wp/admin/build"
	if err := h.LoadWorkspace(); err != nil {
		t.Fatal(err)
	}
	get := func() string {
		req := httptest.NewRequest("GET", "/owner/products", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("workspace: %d", rec.Code)
		}
		return rec.Body.String()
	}
	if strings.Contains(get(), `href="/owner/review"`) {
		t.Fatal("review link shown without OPERATOR_OWNER_ID")
	}
	h.OperatorID = o.ID
	body := get()
	if !strings.Contains(body, `href="/owner/review"`) || !strings.Contains(body, "Offer review") {
		t.Fatal("operator nav is missing Offer review")
	}
}

func TestWordPressOwnerConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	siteID := createTestSite(t, db, "installation-owner-key", "blog.example", "Blog")
	group := r.Group("/v1", middleware.APIKeyAuth(db, nil, h.resource()))
	group.POST("/sites/owner-connection", h.StartSiteConnection)
	group.GET("/sites/owner-connection", h.SiteConnectionStatus)
	start := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/sites/owner-connection", strings.NewReader(`{}`))
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := start("wrong-key"); rec.Code != 401 {
		t.Fatal("unowned installation connected")
	}
	agentID, err := db.CreateAgentAccess(context.Background(), siteID, services.HashAPIKey("publisher-agent-key"))
	if err != nil || agentID == "" {
		t.Fatal(err)
	}
	if rec := start("publisher-agent-key"); rec.Code != 403 {
		t.Fatal("agent gained owner authority")
	}
	started := start("installation-owner-key")
	if started.Code != 201 {
		t.Fatalf("start: %d", started.Code)
	}
	var data map[string]any
	json.Unmarshal(started.Body.Bytes(), &data)
	u, err := url.Parse(data["connect_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	token := u.Query().Get("token")
	o, cookie := testOwnerSession(t, h, "blog-owner", "blog-owner@example.com")
	req := httptest.NewRequest("GET", u.RequestURI(), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "blog.example") {
		t.Fatal("site review missing")
	}
	values := url.Values{"csrf": {o.CSRF}, "token": {token}}
	if rec := ownerForm(t, h, r, cookie, "/owner/connect-site", values); rec.Code != 200 {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	linked, err := db.OwnerSession(context.Background(), services.HashAPIKey(cookie.Value))
	if err != nil || linked.SiteID != siteID {
		t.Fatal("installed site was not linked")
	}
	if rec := ownerForm(t, h, r, cookie, "/owner/connect-site", values); rec.Code != 409 {
		t.Fatal("connection token replayed")
	}
	other, otherCookie := testOwnerSession(t, h, "other-owner", "other@example.com")
	json.Unmarshal(start("installation-owner-key").Body.Bytes(), &data)
	u, _ = url.Parse(data["connect_url"].(string))
	values = url.Values{"csrf": {other.CSRF}, "token": {u.Query().Get("token")}}
	if rec := ownerForm(t, h, r, otherCookie, "/owner/connect-site", values); rec.Code != 409 {
		t.Fatal("another identity claimed linked site")
	}
	if _, err = db.GetSiteByAnyKeyHash(context.Background(), services.HashAPIKey("installation-owner-key"), services.KeyPrefix("installation-owner-key")); err != nil {
		t.Fatal("WordPress installation credential was invalidated")
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE site_owner_connections SET expires_at=NOW()-INTERVAL '1 second'`); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest("GET", u.RequestURI(), nil)
	req.AddCookie(otherCookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 410 {
		t.Fatal("expired connection reviewed")
	}
}

func TestSiteConnectionConcurrentOwners(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, _ := hostedTestServer(t, db)
	siteID := createTestSite(t, db, "race-key", "race.example", "Race")
	a, _ := testOwnerSession(t, h, "claim-a", "a@example.com")
	b, _ := testOwnerSession(t, h, "claim-b", "b@example.com")
	hash := services.HashAPIKey("connection-token")
	if err := db.StartSiteOwnerConnection(context.Background(), siteID, hash); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, o := range []*database.Owner{a, b} {
		wg.Add(1)
		go func(o *database.Owner) {
			defer wg.Done()
			results <- db.CompleteSiteOwnerConnection(context.Background(), o.ID, hash, "")
		}(o)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("connections succeeded: %d", success)
	}
}

func TestMerchantPaymentAuthority(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	maker, cookie := linkedMaker(t, h, "payer")
	other, otherCookie := linkedMaker(t, h, "foreign-payer")
	siteID := createTestSite(t, db, "payee-key", "payee.example", "Payee")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE agent_step_modes SET payout='auto' WHERE site_id=$1::uuid`, siteID); err != nil {
		t.Fatal(err)
	}
	var liveID, testID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id,amount,currency,method,status,maker_id,payment_mode) VALUES($1,10,'USD','manual','pending',$2,'live') RETURNING id`, siteID, maker.MakerID).Scan(&liveID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id,amount,currency,method,status,maker_id,payment_mode) VALUES($1,10,'USD','manual','pending',$2,'test') RETURNING id`, siteID, maker.MakerID).Scan(&testID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"reference": "bank-transfer-reference", "sent": true}
	if rec := ownerJSON(t, h, r, otherCookie, other.CSRF, "POST", "/owner/api/payments/"+liveID+"/report", body); rec.Code != 409 {
		t.Fatal("foreign merchant reported payout")
	}
	if rec := ownerJSON(t, h, r, cookie, maker.CSRF, "POST", "/owner/api/payments/"+testID+"/report", body); rec.Code != 409 {
		t.Fatal("test payout reported as real payment")
	}
	if rec := ownerJSON(t, h, r, cookie, maker.CSRF, "POST", "/owner/api/payments/"+liveID+"/report", body); rec.Code != 200 {
		t.Fatal("maker could not report own live payout")
	}
	var received bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT received_at IS NOT NULL FROM payouts WHERE id=$1`, liveID).Scan(&received); err != nil || received {
		t.Fatal("report incorrectly confirmed receipt")
	}
	if rec := ownerJSON(t, h, r, otherCookie, other.CSRF, "GET", "/owner/api/payments", nil); strings.Contains(rec.Body.String(), liveID) {
		t.Fatal("foreign payout exposed")
	}
}
