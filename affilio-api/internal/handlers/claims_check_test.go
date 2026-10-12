package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/config"
)

func TestClaimsCheckStubKeepsUntrustedSubmitted(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsStub)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("stub submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, reviewReasonClaims, reviewClaimsNotLive)
	assertReviewOnReads(t, r, makerKey, offerID, reviewReasonClaims, reviewClaimsNotLive)
	if queueLen(t, r) != 1 {
		t.Fatal("a stubbed offer must stay in the review queue")
	}

	trustMaker(t, r, makerID)
	_, trustedOffer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 9,
	})
	listed, listedBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+trustedOffer["id"].(string)+"/submit", makerKey, nil)
	if listed.Code != http.StatusOK || listedBody["status"] != "listed" || listedBody["program_id"] == nil || listedBody["program_id"] == "" {
		t.Fatalf("trusted maker must still list under stub, got %d %v", listed.Code, listedBody)
	}
	requireReview(t, listedBody, nil, nil)

	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if body["status"] != "submitted" {
		t.Fatalf("the earlier untrusted offer must stay submitted, got %d %v", rec.Code, body["status"])
	}
	requireReview(t, body, reviewReasonClaims, reviewClaimsNotLive)
}

func TestClaimsCheckOffLocalListsWhenChecksPass(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsOffLocal)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" || body["program_id"] == nil || body["program_id"] == "" {
		t.Fatalf("off-local submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, nil, nil)
	_, got := doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	requireReview(t, got, nil, nil)
	_, offers := doJSON(t, r, http.MethodGet, "/v1/maker/offers", makerKey, nil)
	requireReview(t, itemByID(t, offers, "offers", offerID), nil, nil)
	_, listings := doJSON(t, r, http.MethodGet, "/v1/listings", makerKey, nil)
	requireReview(t, itemByID(t, listings, "listings", offerID), nil, nil)
	if queueLen(t, r) != 0 {
		t.Fatal("off-local must list when the four checks pass")
	}
}

func TestClaimsCheckRealDoesNotListYet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouterWithClaims(db, config.ClaimsReal)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("real submit: %d %v", rec.Code, body)
	}
	requireReview(t, body, reviewReasonClaims, reviewClaimsNotLive)
	assertReviewOnReads(t, r, makerKey, offerID, reviewReasonClaims, reviewClaimsNotLive)
	if queueLen(t, r) != 1 {
		t.Fatal("real must not auto-list until the page checker is installed")
	}
}

func trustMaker(t *testing.T, r interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, makerID string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/makers/"+makerID+"/trust", newJSONRequest(t, http.MethodPost, "", map[string]interface{}{"trusted": true}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trust: %d %s", rec.Code, rec.Body.String())
	}
}
