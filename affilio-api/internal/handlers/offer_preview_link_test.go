package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreatedOfferShowsAGoLinkThatDoesNotRecordAClick(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)

	rec, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name": "Rail", "tagline": "Referral rail", "website_url": "https://rail.example.com",
		"product_type": "saas", "payment_provider": "stripe", "primary_category": "software", "keywords": []string{"email"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("product: %d %v", rec.Code, product)
	}
	rec, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id": product["id"], "commission_type": "percentage", "commission_value": 20,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("offer: %d %v", rec.Code, offer)
	}
	offerID, _ := offer["id"].(string)
	want := "/v1/go/" + strings.ReplaceAll(offerID, "-", "")
	if offer["tracked_link"] != want {
		t.Fatalf("tracked_link = %v want %s", offer["tracked_link"], want)
	}
	var before int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	goReq := httptest.NewRequest(http.MethodGet, want, nil)
	goReq.Header.Set("User-Agent", "Mozilla/5.0")
	goRes := httptest.NewRecorder()
	r.ServeHTTP(goRes, goReq)
	if goRes.Code != http.StatusFound || goRes.Header().Get("Location") != "https://rail.example.com" || strings.Contains(goRes.Header().Get("Location"), "via=") {
		t.Fatalf("preview redirect: %d %s", goRes.Code, goRes.Header().Get("Location"))
	}
	var after int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("preview link recorded a click: %d -> %d", before, after)
	}
}
