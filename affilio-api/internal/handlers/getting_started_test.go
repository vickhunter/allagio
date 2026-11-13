package handlers

import (
	"net/http"
	"testing"
)

func TestGettingStartedWalksTheMakerRecipe(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)

	rec, body := doJSON(t, r, http.MethodGet, "/v1/maker/getting-started", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", rec.Code, body)
	}
	next := body["next_step"].(map[string]interface{})
	if next["tool"] != "register_product" {
		t.Fatalf("a fresh maker should be told to register_product, got %v", next)
	}

	rec, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":        "PromptForge",
		"website_url": "https://promptforge.example.com",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create product: expected 200, got %d: %v", rec.Code, product)
	}
	productID := product["id"].(string)

	_, body = doJSON(t, r, http.MethodGet, "/v1/maker/getting-started", makerKey, nil)
	next = body["next_step"].(map[string]interface{})
	if next["tool"] != "run_assessment" || next["args"].(map[string]interface{})["product_id"] != productID {
		t.Fatalf("unassessed product should point at run_assessment with the product id, got %v", next)
	}

	markProductReady(t, db, productID)
	_, body = doJSON(t, r, http.MethodGet, "/v1/maker/getting-started", makerKey, nil)
	next = body["next_step"].(map[string]interface{})
	if next["tool"] != "create_offer" {
		t.Fatalf("ready product without offers should point at create_offer, got %v", next)
	}

	rec, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "percentage",
		"commission_value": 30,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create offer: expected 200, got %d: %v", rec.Code, offer)
	}

	_, body = doJSON(t, r, http.MethodGet, "/v1/maker/getting-started", makerKey, nil)
	next = body["next_step"].(map[string]interface{})
	if next["tool"] != "submit_offer" || next["args"].(map[string]interface{})["offer_id"] != offer["id"].(string) {
		t.Fatalf("a draft offer should point at submit_offer with the offer id, got %v", next)
	}
}
