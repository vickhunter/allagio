package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicStatsServesCounters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	entry := &AgentEntry{DB: db}
	r := gin.New()
	r.GET("/v1/stats", entry.PublicStats)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/stats", nil))
	if w.Code != 200 {
		t.Fatalf("stats: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Deals    int64 `json:"deals"`
		Offers   int64 `json:"offers"`
		Products int64 `json:"products"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("stats body: %v", err)
	}
}

func TestPublicOffersListsOnlyListed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID, err := db.CreateMaker(context.Background(), "hash", "pref", "feed@example.com", "Feed Maker")
	if err != nil {
		t.Fatal(err)
	}
	var productID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id,name,website_url,product_type,primary_category)
VALUES($1,'Feed Product','https://feed.example.com','saas','devtools') RETURNING id`, makerID).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	var listedID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
VALUES($1,'listed','percentage',30,'https://feed.example.com') RETURNING id`, productID).Scan(&listedID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
VALUES($1,'draft','percentage',40,'https://feed.example.com')`, productID); err != nil {
		t.Fatal(err)
	}
	entry := &AgentEntry{DB: db}
	r := gin.New()
	r.GET("/v1/offers", entry.PublicOffers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/offers", nil))
	if w.Code != 200 {
		t.Fatalf("offers feed: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Offers []struct {
			OfferID string `json:"offer_id"`
		} `json:"offers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("offers body: %v", err)
	}
	if len(body.Offers) != 1 || body.Offers[0].OfferID != listedID {
		t.Fatalf("expected only the listed offer, got %s", w.Body.String())
	}
}
