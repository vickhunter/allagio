package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
)

func TestUntrustedMakerListsWhenChecksPass(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	claims := entry.StubClaimChecker{}
	if claims.Real() {
		t.Fatal("the production claim checker must stay a stub so yolo remains preselected")
	}

	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict || body["error"] != readinessFailMessage || body["reason"] != reviewReasonHead || body["message"] == "" {
		t.Fatalf("unassessed submit: %d %v", rec.Code, body)
	}
	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "draft" {
		t.Fatalf("unassessed offer must stay draft, got %d %v", rec.Code, body["status"])
	}

	markProductReady(t, db, productID)
	rec, body = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" || body["program_id"] == nil || body["program_id"] == "" {
		t.Fatalf("passing checks must list, got %d %v", rec.Code, body)
	}
	requireReview(t, body, nil, nil)

	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if body["status"] != "listed" {
		t.Fatalf("get: %d %v", rec.Code, body["status"])
	}
	requireReview(t, body, nil, nil)

	_, offers := doJSON(t, r, http.MethodGet, "/v1/maker/offers", makerKey, nil)
	requireReview(t, itemByID(t, offers, "offers", offerID), nil, nil)
	_, listings := doJSON(t, r, http.MethodGet, "/v1/listings", makerKey, nil)
	listed := itemByID(t, listings, "listings", offerID)
	requireReview(t, listed, nil, nil)
	card, _ := listed["card"].(map[string]interface{})
	if card["name"] != "Northwind Ledger" {
		t.Fatalf("listing card: %v", listed["card"])
	}
	if queueLen(t, r) != 0 {
		t.Fatal("a listed offer must not sit in the review queue")
	}
}

func TestUntrustedMakerClaimsFlagListsAnyway(t *testing.T) {
	assertListedWithAdvisory(t, "Northwind Ledger saves 99 percent.", reviewReasonClaims, reviewClaimsMessage)
}

func TestUntrustedMakerHeadFlagListsAnyway(t *testing.T) {
	prev := landingReachable
	landingReachable = func(string) bool { return false }
	t.Cleanup(func() { landingReachable = prev })
	assertListedWithAdvisory(t, passingTerms("Northwind Ledger", 20), reviewReasonHead, reviewHeadMessage)
}

func TestUntrustedMakerRelevanceFlagListsAnyway(t *testing.T) {
	assertListedWithAdvisory(t, "A handy tool for teams.", reviewReasonRelevance, reviewRelevanceMessage)
}

func TestUntrustedMakerCapsFailureStaysInReview(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	for i := 0; i < listingCap; i++ {
		_, offerID := createDraftOffer(t, r, makerKey, fmt.Sprintf("Cap Filler %d", i), fmt.Sprintf("https://cap-%d.example.com", i), "", 10)
		if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed' WHERE id=$1`, offerID); err != nil {
			t.Fatal(err)
		}
	}
	var listed int
	if err := db.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM offers o JOIN products p ON p.id = o.product_id
WHERE p.maker_id=$1 AND o.status='listed'`, makerID).Scan(&listed); err != nil || listed != listingCap {
		t.Fatalf("listed count %d: %v", listed, err)
	}
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", passingTerms("Northwind Ledger", 20), 20)
	markProductReady(t, db, productID)
	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" {
		t.Fatalf("cap: %d %v", rec.Code, body)
	}
	requireReview(t, body, reviewReasonCaps, reviewCapsMessage)
	assertListedOnReads(t, r, makerKey, offerID)
	if queueLen(t, r) != 0 {
		t.Fatal("a listed offer must not sit in the review queue")
	}
}

func TestReadinessFailureMapsReviewReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	productID, offerID := createDraftOffer(t, r, makerKey, "Harbor Atlas", "https://harbor.example.com", passingTerms("Harbor Atlas", 15), 15)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict || body["error"] != readinessFailMessage || body["reason"] != reviewReasonHead || body["message"] != readinessFailMessage {
		t.Fatalf("unassessed: %d %v", rec.Code, body)
	}

	setReadiness(t, db, productID, "failing", `[{"key":"checkout_present","gate":true,"status":"fail","remediation":"Add a buy, subscribe, checkout, or sign-up link."}]`)
	rec, body = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict || body["reason"] != reviewReasonClaims || body["message"] != "Add a buy, subscribe, checkout, or sign-up link." || body["error"] != readinessFailMessage {
		t.Fatalf("checkout: %d %v", rec.Code, body)
	}

	setReadiness(t, db, productID, "failing", `[{"key":"site_reachable","gate":true,"status":"fail","remediation":"The site did not answer."}]`)
	rec, body = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict || body["reason"] != reviewReasonHead || body["message"] != "The site did not answer." {
		t.Fatalf("site: %d %v", rec.Code, body)
	}

	setReadiness(t, db, productID, "failing", `[{"key":"pricing_page","gate":true,"status":"fail","remediation":"Add a pricing page."}]`)
	rec, body = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict || body["reason"] != reviewReasonRelevance || body["message"] != "Add a pricing page." {
		t.Fatalf("pricing: %d %v", rec.Code, body)
	}

	rec, body = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if body["status"] != "draft" {
		t.Fatalf("readiness failure must leave the offer draft, got %v", body["status"])
	}

	markProductReady(t, db, productID)
	rec, body = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" {
		t.Fatalf("a later ready submit must list, got %d %v", rec.Code, body)
	}
	requireReview(t, body, nil, nil)
}

func assertListedWithAdvisory(t *testing.T, terms, reason, message string) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	productID, offerID := createDraftOffer(t, r, makerKey, "Northwind Ledger", "https://northwind.example.com", terms, 20)
	markProductReady(t, db, productID)
	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK || body["status"] != "listed" {
		t.Fatalf("%s: %d %v", reason, rec.Code, body)
	}
	requireReview(t, body, reason, message)
	assertListedOnReads(t, r, makerKey, offerID)
	if queueLen(t, r) != 0 {
		t.Fatalf("%s offer must list without waiting on the review queue", reason)
	}
}

// assertListedOnReads verifies the offer reads as listed with empty review
// fields: an advisory flag rides on the submit response only.
func assertListedOnReads(t *testing.T, r *gin.Engine, makerKey, offerID string) {
	t.Helper()
	_, got := doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	requireReview(t, got, nil, nil)
	if got["status"] != "listed" {
		t.Fatalf("offer status %v", got["status"])
	}
	_, offers := doJSON(t, r, http.MethodGet, "/v1/maker/offers", makerKey, nil)
	requireReview(t, itemByID(t, offers, "offers", offerID), nil, nil)
	_, listings := doJSON(t, r, http.MethodGet, "/v1/listings", makerKey, nil)
	requireReview(t, itemByID(t, listings, "listings", offerID), nil, nil)
}

func createDraftOffer(t *testing.T, r *gin.Engine, makerKey, name, website, terms string, commission float64) (string, string) {
	t.Helper()
	rec, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             name,
		"website_url":      website,
		"product_type":     "saas",
		"primary_category": "software",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create product: %d %v", rec.Code, product)
	}
	payload := map[string]interface{}{
		"product_id":       product["id"],
		"commission_type":  "percentage",
		"commission_value": commission,
	}
	if terms != "" {
		payload["terms"] = terms
	}
	rec, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("create offer: %d %v", rec.Code, offer)
	}
	return product["id"].(string), offer["id"].(string)
}

func passingTerms(name string, commission int) string {
	return fmt.Sprintf("%s pays %d percent to publishers.", name, commission)
}

func setReadiness(t *testing.T, db *database.DB, productID, status, checks string) {
	t.Helper()
	if _, err := db.InsertReadinessAssessment(context.Background(), productID, 10, []byte(checks), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET readiness_status=$2 WHERE id=$1`, productID, status); err != nil {
		t.Fatal(err)
	}
}

func requireReview(t *testing.T, item map[string]interface{}, reason, message interface{}) {
	t.Helper()
	gotReason, okR := item["review_reason"]
	gotMessage, okM := item["message"]
	if !okR || !okM || gotReason != reason || gotMessage != message {
		t.Fatalf("review fields reason=%v present=%v message=%v present=%v, want %v / %v", gotReason, okR, gotMessage, okM, reason, message)
	}
}

func itemByID(t *testing.T, body map[string]interface{}, key, id string) map[string]interface{} {
	t.Helper()
	items, _ := body[key].([]interface{})
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		if item["id"] == id {
			return item
		}
	}
	t.Fatalf("%s %s not found in %v", key, id, body)
	return nil
}

func queueLen(t *testing.T, r *gin.Engine) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/offer-review/queue", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", rec.Code, rec.Body.String())
	}
	var queueBody struct {
		Queue []map[string]interface{} `json:"queue"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queueBody); err != nil {
		t.Fatal(err)
	}
	return len(queueBody.Queue)
}
