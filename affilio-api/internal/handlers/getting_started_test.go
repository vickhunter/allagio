package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"affilio-api/internal/services"
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

// A fresh join token (what the MCP installer hands an agent) must be able to
// call get_started first: that call is what creates its maker.
func TestGettingStartedJoinTokenOnboards(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r, _ := mountJoinTokens(t, db, 8)

	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, rec, body)

	gs, gsBody := postJSON(t, r, http.MethodGet, "/v1/maker/getting-started", nil, bearerHeader(token))
	if gs.Code != http.StatusOK {
		t.Fatalf("fresh join token on getting-started: %d %s", gs.Code, gs.Body.String())
	}
	next := gsBody["next_step"].(map[string]interface{})
	if next["tool"] != "register_product" {
		t.Fatalf("a fresh join token should be told to register_product, got %v", next)
	}
}

// An offer-scoped project key gets the recipe for its own offer only, and
// never reads the maker's other products or offers.
func TestGettingStartedOfferScopedKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	makerID, offerID := seedListedOffer(t, db, "scoped-maker-key", "scoped@example.com", "Scoped Product")
	// A second product + offer the scoped key must not see.
	var otherProduct string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id,name,website_url,product_type,primary_category)
VALUES($1,'Other Product','https://other.example.com','saas','devtools') RETURNING id`, makerID).Scan(&otherProduct); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
VALUES($1,'draft','percentage',20,'https://other.example.com')`, otherProduct); err != nil {
		t.Fatal(err)
	}
	var ownerID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO owners(issuer,subject,email,name,source,agent_only,human_login)
VALUES('https://test','getting-started-scope','scope-owner@example.com','Scope Owner','direct',true,false) RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	offerKey := "or_getting_started_scope"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,offer_id,maker_id,expires_at)
VALUES($1,'or_gets',$2,'offer',$3,$4,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(offerKey), ownerID, offerID, makerID); err != nil {
		t.Fatal(err)
	}

	r := newMakerRouter(db)
	rec, body := doJSON(t, r, http.MethodGet, "/v1/maker/getting-started", offerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("scoped key on getting-started: %d %s", rec.Code, rec.Body.String())
	}
	if body["offer_scoped"] != true {
		t.Fatalf("expected offer_scoped answer, got %s", rec.Body.String())
	}
	next := body["next_step"].(map[string]interface{})
	if next["args"].(map[string]interface{})["offer_id"] != offerID {
		t.Fatalf("scoped key should be guided to its own offer %s, got %v", offerID, next)
	}
	if rec.Body.String() == "" || jsonContains(rec.Body.String(), "Other Product") {
		t.Fatalf("scoped answer leaked maker state: %s", rec.Body.String())
	}
}

func jsonContains(body, needle string) bool {
	return strings.Contains(body, needle)
}
