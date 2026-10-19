package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/entry"
)

func TestMerchantFetchUsesFixturesInTests(t *testing.T) {
	page := fetchMerchantPage(context.Background(), "http://127.0.0.1:1/merchant")
	if page.OK || page.Body != "" || page.Err != "fixture required" {
		t.Fatalf("tests must not dial the merchant page: %+v", page)
	}
}

func TestClaimsCheckRealListsWhenPageMatches(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsReal)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)
	insertListingCard(t, db, makerID, productID, northwindCard())
	useMerchantPage(t, "https://northwind.example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_match.html")})

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" || body["program_id"] == nil || body["program_id"] == "" {
		t.Fatalf("real submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, nil, nil)
	_, got := doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	requireReview(t, got, nil, nil)
	if queueLen(t, r) != 0 {
		t.Fatal("a matching merchant page must list")
	}
}

func TestClaimsCheckRealHoldsWhenPageDiffers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsReal)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)
	insertListingCard(t, db, makerID, productID, northwindCard())
	useMerchantPage(t, "https://northwind.example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_price.html")})

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("real submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, reviewReasonClaims, entry.PageClaimMessage(entry.PageReasonPrice))
	assertReviewOnReads(t, r, makerKey, offerID, reviewReasonClaims, entry.PageClaimMessage(entry.PageReasonPrice))
	_, again := doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if again["message"] != body["message"] || again["review_reason"] != body["review_reason"] {
		t.Fatalf("recompute changed the reason: %v then %v", body, again)
	}
	if queueLen(t, r) != 1 {
		t.Fatal("a price miss must stay in review")
	}

	trustMaker(t, r, makerID)
	_, trustedOffer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 9,
	})
	listed, listedBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+trustedOffer["id"].(string)+"/submit", makerKey, nil)
	if listed.Code != http.StatusOK || listedBody["status"] != "listed" {
		t.Fatalf("trusted maker must still list under real, got %d %v", listed.Code, listedBody)
	}
	requireReview(t, listedBody, nil, nil)
}

func TestClaimsCheckRealHeadMissUsesHeadReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsReal)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)
	insertListingCard(t, db, makerID, productID, northwindCard())
	useMerchantPage(t, "https://northwind.example.com", merchantPage{})

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("real submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, reviewReasonHead, reviewHeadMessage)
	assertReviewOnReads(t, r, makerKey, offerID, reviewReasonHead, reviewHeadMessage)
}

func TestRestoreAndRetryUsePageClaims(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	current := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }
	owner, cookie := testOwnerSession(t, h, "page-owner", "page-owner@example.com")
	siteID := createTestSite(t, db, "page-site-key", "page.example", "Page")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	makerID, offerID := seedListedOffer(t, db, "page-maker", "page-maker@example.com", "Claims Product")
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id::text FROM offers WHERE id=$1`, offerID).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	insertListingCard(t, db, makerID, productID, entry.ListingCard{
		Name: "Claims Product", Summary: "A solid claims product.", Category: "devtools",
		Features: []string{"exports"}, PricingSummary: "Plans start at $29 a month",
	})
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://page.example/post", "Claims Product is a solid fit.")
	approved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	if approved.Code != http.StatusOK {
		t.Fatalf("decide: %d %s", approved.Code, approved.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, map[string]string{"X-API-Key": "page-site-key"})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}

	api.Claims = entry.StubClaimChecker{}
	api.ClaimsCheck = config.ClaimsReal
	useMerchantPage(t, "https://example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_restore_miss.html")})
	current = current.Add(time.Second)
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "claims_from_offer" || blockedBody["check_text"] != "Use only claims from the offer." {
		t.Fatalf("restore claims: %d %s", blocked.Code, blocked.Body.String())
	}
	var status, wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&status, &wait); err != nil || status != "revoked" || wait == "restore" {
		t.Fatalf("restore wrote through a page miss: %s %s %v", status, wait, err)
	}

	useMerchantPage(t, "https://example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_restore_match.html")})
	current = current.Add(time.Second)
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	pluginKey := issuePluginKey(t, db, siteID, "page-plugin-key")
	failed, failedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-read failed"}, pluginKey)
	if failed.Code != http.StatusOK || failedBody["failed_action"] != "restored" {
		t.Fatalf("fail ack: %d %s", failed.Code, failed.Body.String())
	}

	useMerchantPage(t, "https://example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_restore_miss.html")})
	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, map[string]string{"X-API-Key": "page-site-key"})
	if retried.Code != http.StatusUnprocessableEntity || retriedBody["check_id"] != "claims_from_offer" {
		t.Fatalf("retry claims: %d %s", retried.Code, retried.Body.String())
	}
	var action, reason string
	if err := db.Pool.QueryRow(context.Background(), `SELECT failed_action, plugin_failed_reason FROM placement_proposals WHERE id=$1`, placementID).Scan(&action, &reason); err != nil || action != "restored" || reason != "re-read failed" {
		t.Fatalf("retry wrote through a page miss: %s %s %v", action, reason, err)
	}

	useMerchantPage(t, "https://example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_restore_match.html")})
	retried, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, map[string]string{"X-API-Key": "page-site-key"})
	if retried.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", retried.Code, retried.Body.String())
	}
}

func TestRealClaimsInReleaseListsMatchAndHoldsMismatch(t *testing.T) {
	mode, err := config.ResolveClaimsCheck("release", "real")
	if err != nil || mode != config.ClaimsReal {
		t.Fatalf("hosted real: %q %v", mode, err)
	}
	if checker := claimsChecker(mode); checker.Real() == false {
		t.Fatal("release real must wire the live page checker")
	}
	if _, ok := claimsChecker(mode).(entry.PageClaimChecker); !ok {
		t.Fatal("release real must use PageClaimChecker")
	}
	if _, ok := claimsChecker(config.ClaimsStub).(entry.StubClaimChecker); !ok || claimsChecker("").Real() {
		t.Fatal("the hosted default must stay the stub")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, mode)
	matchID, matchOffer := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://match.example.com", passingTerms("Northwind Ledger", 20), 20)
	missID, missOffer := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://miss.example.com", passingTerms("Northwind Ledger", 20), 20)
	brokenID, brokenOffer := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://broken.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, matchID)
	markProductReady(t, db, missID)
	markProductReady(t, db, brokenID)
	insertListingCard(t, db, makerID, matchID, northwindCard())
	insertListingCard(t, db, makerID, missID, northwindCard())
	insertListingCard(t, db, makerID, brokenID, northwindCard())
	prev := fetchMerchantPage
	fetchMerchantPage = func(_ context.Context, got string) merchantPage {
		switch got {
		case "https://match.example.com":
			return merchantPage{OK: true, Body: clientRenderedOffer("Plans start at $29 a month")}
		case "https://miss.example.com":
			return merchantPage{OK: true, Body: clientRenderedOffer("Plans start at $49 a month")}
		case "https://broken.example.com":
			return merchantPage{OK: true, Body: "<html><head><script type=\"application/ld+json\">{</script></head><body><div id=\"root\"></div></body></html>"}
		default:
			return merchantPage{Err: "fixture required"}
		}
	}
	t.Cleanup(func() { fetchMerchantPage = prev })

	listed, listedBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+matchOffer+"/submit", makerKey, nil)
	if listed.Code != http.StatusOK || listedBody["status"] != "listed" {
		t.Fatalf("matching client-rendered page: %d %v", listed.Code, listedBody)
	}
	requireReview(t, listedBody, nil, nil)

	held, heldBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+missOffer+"/submit", makerKey, nil)
	if held.Code != http.StatusOK || heldBody["status"] != "submitted" {
		t.Fatalf("mismatched claim: %d %v", held.Code, heldBody)
	}
	requireReview(t, heldBody, reviewReasonClaims, entry.PageClaimMessage(entry.PageReasonPrice))

	broken, brokenBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+brokenOffer+"/submit", makerKey, nil)
	if broken.Code != http.StatusOK || brokenBody["status"] != "submitted" || brokenBody["review_reason"] != reviewReasonClaims {
		t.Fatalf("broken page must be held with a reason, not 500: %d %v", broken.Code, brokenBody)
	}
}

func clientRenderedOffer(price string) string {
	return `<!DOCTYPE html><html><head><title>App</title>` +
		`<meta content="Northwind Ledger" property="og:title">` +
		`<meta property="og:description" content="Ledger software for small publishers.">` +
		`<meta name="description" content="Category: software">` +
		`<script type='application/ld+json'>{"featureList":["invoices"],"offers":{"description":"` + price + `","price":20}}</script>` +
		`</head><body><div id="root"></div></body></html>`
}

func TestPageClaimCheckerRestoreHoldsAMiss(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "page-live-owner", "page-live-owner@example.com")
	siteID := createTestSite(t, db, "page-live-key", "page-live.example", "Page")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	makerID, offerID := seedListedOffer(t, db, "page-live-maker", "page-live-maker@example.com", "Claims Product")
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT product_id::text FROM offers WHERE id=$1`, offerID).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	insertListingCard(t, db, makerID, productID, entry.ListingCard{
		Name: "Claims Product", Summary: "A solid claims product.", Category: "devtools",
		Features: []string{"exports"}, PricingSummary: "Plans start at $29 a month",
	})
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://page-live.example/post", "Claims Product is a solid fit.")
	approved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	if approved.Code != http.StatusOK {
		t.Fatalf("decide: %d %s", approved.Code, approved.Body.String())
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, map[string]string{"X-API-Key": "page-live-key"})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}

	api.Claims = entry.PageClaimChecker{}
	api.ClaimsCheck = config.ClaimsReal
	useMerchantPage(t, "https://example.com", merchantPage{OK: true, Body: merchantFixture(t, "merchant_restore_miss.html")})
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "claims_from_offer" {
		t.Fatalf("restore claims: %d %s", blocked.Code, blocked.Body.String())
	}
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, placementID).Scan(&status); err != nil || status != "revoked" {
		t.Fatalf("restore wrote through a page miss: %s %v", status, err)
	}
}

func useMerchantPage(t *testing.T, rawURL string, page merchantPage) {
	t.Helper()
	prev := fetchMerchantPage
	fetchMerchantPage = func(_ context.Context, got string) merchantPage {
		if got == rawURL {
			return page
		}
		return merchantPage{Err: "fixture required"}
	}
	t.Cleanup(func() { fetchMerchantPage = prev })
}

func merchantFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "entry", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func northwindCard() entry.ListingCard {
	return entry.ListingCard{
		Name:           "Northwind Ledger",
		Summary:        "Ledger software for small publishers.",
		Category:       "software",
		Features:       []string{"invoices"},
		PricingSummary: "Plans start at $29 a month",
	}
}

func insertListingCard(t *testing.T, db *database.DB, makerID, productID string, card entry.ListingCard) {
	t.Helper()
	raw, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.InsertListingCard(context.Background(), database.ListingCardRow{
		MakerID: makerID, ProductID: productID, Card: raw,
		Signals: []byte(`[{"name":"payments","pass":true}]`), Score: 80,
		PaymentsCheck: "pass", OfferLinkHealth: "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
}
