package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRevenuePreviewMatchesStagingPost2615(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)

	var makerID string
	err := db.Pool.QueryRow(context.Background(), `
INSERT INTO makers(email, api_key)
VALUES (gen_random_uuid()::text || '@example.test', gen_random_uuid()::text)
RETURNING id`).Scan(&makerID)
	if err != nil {
		t.Fatal(err)
	}
	keywords := []string{"landing page", "landing page builder", "page builder", "no-code", "templates", "conversion", "lead generation", "email marketing"}
	var offerID string
	err = db.Pool.QueryRow(context.Background(), `
WITH product AS (
  INSERT INTO products(maker_id, name, tagline, website_url, product_type, primary_category, keywords)
  VALUES ($1, $2, $3, 'https://example.test', 'other', $4, $5)
  RETURNING id
)
INSERT INTO offers(product_id, status, commission_type, commission_value, commission_currency, landing_url, terms)
SELECT id, 'listed', 'percentage', 30, 'USD', 'https://example.test', $6 FROM product
RETURNING id`,
		makerID,
		"Offer Rails Demo",
		"Demo landing page builder: launch a landing page without code",
		"landing page builder",
		keywords,
		"Commission paid directly by merchant.",
	).Scan(&offerID)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join("..", "services", "testdata", "staging_post_2615.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	h := &PlacementsHandler{DB: db}
	r.POST("/v1/revenue/preview", h.Find)
	rec, body := doJSON(t, r, http.MethodPost, "/v1/revenue/preview", "", map[string]interface{}{"context": string(raw)})
	if rec.Code != http.StatusOK || body["status"] != "matched" {
		t.Fatalf("preview: %d %v", rec.Code, body)
	}
	opps, _ := body["opportunities"].([]interface{})
	if len(opps) != 1 {
		t.Fatalf("opportunities: %v", body["opportunities"])
	}
	opp := opps[0].(map[string]interface{})
	if opp["offer_id"] != offerID {
		t.Fatalf("offer_id %v, want the hosted Demo offer", opp["offer_id"])
	}
	relevance, _ := opp["relevance"].(float64)
	if relevance < 32 {
		t.Fatalf("hosted Demo offer relevance %v, want >= 32", opp["relevance"])
	}
}
