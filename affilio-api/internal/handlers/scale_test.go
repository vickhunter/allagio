package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMarketplaceRanksOffersBySiteProfile(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)

	// Two listed offers: one about email productivity, one about retro games.
	makeListed := func(name, tagline, category string, keywords []string) string {
		t.Helper()
		_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
			"name":             name,
			"tagline":          tagline,
			"website_url":      "https://" + strings.ToLower(name) + ".example.com",
			"primary_category": category,
			"keywords":         keywords,
		})
		markProductReady(t, db, product["id"].(string))
		_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
			"product_id":       product["id"],
			"commission_type":  "percentage",
			"commission_value": 20,
		})
		offerID := offer["id"].(string)
		doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
		req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/"+offerID+"/approve", nil)
		req.Header.Set("X-Admin-Key", "admin-secret")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve %s: %d %s", name, rec.Code, rec.Body.String())
		}
		return offerID
	}
	emailOffer := makeListed("InboxPilot", "Email productivity for founders", "software", []string{"email", "inbox", "productivity"})
	gameOffer := makeListed("PixelQuest", "A retro platformer adventure", "games", []string{"game", "retro", "platformer"})

	// An email-focused blog.
	siteKey := "site-key-profile"
	siteID := createTestSite(t, db, siteKey, "emailblog.example.com", "Email Blog")
	titles := []string{
		"How I organize my email inbox for productivity",
		"The best email tools for busy founders",
		"Inbox zero: a practical productivity guide",
	}
	for i, title := range titles {
		if _, err := db.UpsertArticle(context.Background(), siteID, int64(i+1), title, "https://emailblog.example.com/p"+fmt.Sprint(i), "hash"+fmt.Sprint(i)); err != nil {
			t.Fatalf("insert article: %v", err)
		}
	}

	rec, body := doSiteJSON(t, r, http.MethodGet, "/v1/marketplace/offers", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("marketplace: expected 200, got %d", rec.Code)
	}
	offers := body["offers"].([]interface{})
	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(offers))
	}
	first := offers[0].(map[string]interface{})
	second := offers[1].(map[string]interface{})
	if first["offer_id"] != emailOffer {
		t.Fatalf("expected email offer ranked first, got %v (game=%s)", first["offer_id"], gameOffer)
	}
	if first["relevance"].(float64) <= second["relevance"].(float64) {
		t.Fatalf("expected descending relevance, got %v then %v", first["relevance"], second["relevance"])
	}

	// Maker sees the email blog as the top match for the email offer.
	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+emailOffer+"/matches", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("matches: expected 200, got %d", rec.Code)
	}
	matches := body["matches"].([]interface{})
	if len(matches) != 1 {
		t.Fatalf("expected 1 matchable site, got %d", len(matches))
	}
	match := matches[0].(map[string]interface{})
	if match["site_domain"] != "emailblog.example.com" || match["relevance"].(float64) < 50 || match["joined"] != false {
		t.Fatalf("unexpected match %v", match)
	}
}

func TestTrustedMakerSubmitListsDirectly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)

	// Flip the trusted fast path on via the admin endpoint.
	req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/makers/"+makerID+"/trust",
		newJSONRequest(t, http.MethodPost, "", map[string]interface{}{"trusted": true}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trust: expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":        "TrustedTool",
		"website_url": "https://trustedtool.example.com",
	})
	markProductReady(t, db, product["id"].(string))
	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       product["id"],
		"commission_type":  "flat",
		"commission_value": 15,
	})
	offerID := offer["id"].(string)

	srec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if srec.Code != http.StatusOK || body["status"] != "listed" {
		t.Fatalf("expected trusted submit to list directly, got %d %v", srec.Code, body)
	}
	if body["program_id"] == nil || body["program_id"] == "" {
		t.Fatalf("expected program_id for trusted listing, got %v", body)
	}
}

func TestFraudFlagsHoldCommissionsUntilCleared(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newPayoutsRouter(db)
	offerID := listedOffer(t, db, r, makerKey)

	siteKey := "site-key-fraud"
	createTestSite(t, db, siteKey, "fraudblog.example.com", "Blog")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)

	// Purchase seconds after the click: booked, but flagged. The click's own
	// truncated second is not this flag, so the sale is five seconds later.
	var clicked time.Time
	if err := db.Pool.QueryRow(context.Background(), `SELECT clicked_at FROM clicks WHERE id=$1`, clickID).Scan(&clicked); err != nil {
		t.Fatal(err)
	}
	payload := stripeSalePayload("evt_fraud_1", "pi_fraud", clickID, 10000)
	payload["created"] = clicked.Add(5 * time.Second).Unix()
	wrec, wbody := sendStripeEvent(t, db, r, payload)
	if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
		t.Fatalf("expected recorded, got %d %v", wrec.Code, wbody)
	}
	var flagged bool
	var reason string
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged, COALESCE(flag_reason,'') FROM conversions WHERE provider_event_id='evt_fraud_1'`).Scan(&flagged, &reason); err != nil {
		t.Fatalf("load conversion: %v", err)
	}
	if !flagged || !strings.Contains(reason, "fast click-to-purchase") {
		t.Fatalf("expected fast-purchase flag, got flagged=%v reason=%q", flagged, reason)
	}

	// Even matured, the flagged commission never clears.
	if _, err := db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at = NOW() - INTERVAL '1 day'`); err != nil {
		t.Fatalf("mature ledger: %v", err)
	}
	rec, body := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("balance: expected 200, got %d", rec.Code)
	}
	usd := body["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"].(float64) != 20 || usd["cleared_unpaid"].(float64) != 0 {
		t.Fatalf("expected flagged commission held pending, got %v", usd)
	}

	// Admin reviews the flag queue and releases it; the commission clears.
	req = httptest.NewRequest(http.MethodGet, "/v1/offer-review/flagged-conversions", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	frec := httptest.NewRecorder()
	r.ServeHTTP(frec, req)
	if frec.Code != http.StatusOK || !strings.Contains(frec.Body.String(), "fast click-to-purchase") {
		t.Fatalf("flagged list: expected 200 with flag reason, got %d %s", frec.Code, frec.Body.String())
	}
	var conversionID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM conversions WHERE provider_event_id='evt_fraud_1'`).Scan(&conversionID); err != nil {
		t.Fatalf("lookup conversion: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/offer-review/conversions/"+conversionID+"/clear-flag", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	crec := httptest.NewRecorder()
	r.ServeHTTP(crec, req)
	if crec.Code != http.StatusOK {
		t.Fatalf("clear-flag: expected 200, got %d", crec.Code)
	}
	rec, body = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd = body["balances"].([]interface{})[0].(map[string]interface{})
	if usd["cleared_unpaid"].(float64) != 20 {
		t.Fatalf("expected commission cleared after flag release, got %v", usd)
	}
}

func TestVelocityCapPausesCampaign(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)

	siteKey := "site-key-velocity"
	createTestSite(t, db, siteKey, "velocityblog.example.com", "Blog")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)
	campaignID := join["campaign_id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+code, nil)
	rrec := httptest.NewRecorder()
	r.ServeHTTP(rrec, req)
	requirePlainRedirect(t, rrec, "https://inboxpilot.example.com")
	clickID := insertCampaignClick(t, db, code)

	// 21 sales inside an hour: the 21st trips the velocity cap.
	for i := 1; i <= 21; i++ {
		wrec, wbody := sendStripeEvent(t, db, r, stripeSalePayload(fmt.Sprintf("evt_vel_%d", i), fmt.Sprintf("pi_vel_%d", i), clickID, 1000))
		if wrec.Code != http.StatusOK || wbody["status"] != "recorded" {
			t.Fatalf("sale %d: expected recorded, got %d %v", i, wrec.Code, wbody)
		}
	}
	var flagged bool
	var reason string
	if err := db.Pool.QueryRow(context.Background(), `SELECT flagged, COALESCE(flag_reason,'') FROM conversions WHERE provider_event_id='evt_vel_21'`).Scan(&flagged, &reason); err != nil {
		t.Fatalf("load conversion: %v", err)
	}
	if !flagged || !strings.Contains(reason, "velocity") {
		t.Fatalf("expected velocity flag on 21st sale, got flagged=%v reason=%q", flagged, reason)
	}
	var campaignStatus string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM campaigns WHERE id=$1`, campaignID).Scan(&campaignStatus); err != nil {
		t.Fatalf("load campaign: %v", err)
	}
	if campaignStatus != "paused" {
		t.Fatalf("expected campaign paused by velocity cap, got %s", campaignStatus)
	}
}
