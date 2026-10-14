package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFirstProductEditKeepsCategoryAndKeywords(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	owner, cookie := linkedMaker(t, h, "taxonomy-owner")
	keywords := []string{"postgres", "monitoring", "debugging", "alerts", "metrics", "logs", "tracing", "slo"}

	created := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/products", map[string]any{
		"name":             "Offer Rails Demo",
		"website_url":      "https://demo.example",
		"tagline":          "A product with no topics yet",
		"primary_category": "developer tools",
		"keywords":         keywords,
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	product := decodeProduct(t, created.Body.Bytes())
	id := product["id"].(string)
	assertTaxonomy(t, "create", product, keywords)

	// The edit form posts every field. The first save must keep the topics the maker set.
	saved := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPatch, "/owner/api/products/"+id, map[string]any{
		"name":             "Offer Rails Demo",
		"website_url":      "https://demo.example",
		"tagline":          "A product with no topics yet",
		"docs_url":         "",
		"pricing_url":      "",
		"primary_category": "developer tools",
		"keywords":         keywords,
	})
	if saved.Code != http.StatusOK {
		t.Fatalf("first save: %d %s", saved.Code, saved.Body.String())
	}
	assertTaxonomy(t, "first save", decodeProduct(t, saved.Body.Bytes()), keywords)

	listed := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodGet, "/owner/api/products", nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("reload: %d %s", listed.Code, listed.Body.String())
	}
	assertTaxonomy(t, "reload", productByID(t, listed.Body.Bytes(), id), keywords)

	// A later save that does not mention topics must leave them in place.
	renamed := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPatch, "/owner/api/products/"+id, map[string]any{
		"name": "Offer Rails Demo renamed",
	})
	if renamed.Code != http.StatusOK {
		t.Fatalf("untouched topics save: %d %s", renamed.Code, renamed.Body.String())
	}
	assertTaxonomy(t, "untouched save", decodeProduct(t, renamed.Body.Bytes()), keywords)

	// The edit form always includes these fields. A blank capture is not a request to erase topics.
	blank := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPatch, "/owner/api/products/"+id, map[string]any{
		"name":             "Offer Rails Demo renamed",
		"website_url":      "https://demo.example",
		"tagline":          "A product with no topics yet",
		"docs_url":         "",
		"pricing_url":      "",
		"primary_category": "",
		"keywords":         []string{},
	})
	if blank.Code != http.StatusOK {
		t.Fatalf("blank topics save: %d %s", blank.Code, blank.Body.String())
	}
	assertTaxonomy(t, "blank save", decodeProduct(t, blank.Body.Bytes()), keywords)

	reloaded := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodGet, "/owner/api/products", nil)
	if reloaded.Code != http.StatusOK {
		t.Fatalf("reload after blank: %d %s", reloaded.Code, reloaded.Body.String())
	}
	assertTaxonomy(t, "reload after blank", productByID(t, reloaded.Body.Bytes(), id), keywords)

	replaced := []string{"queues", "workers", "retries"}
	changed := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPatch, "/owner/api/products/"+id, map[string]any{
		"primary_category": "background jobs",
		"keywords":         replaced,
	})
	if changed.Code != http.StatusOK {
		t.Fatalf("replace topics: %d %s", changed.Code, changed.Body.String())
	}
	got := decodeProduct(t, changed.Body.Bytes())
	if got["primary_category"] != "background jobs" {
		t.Fatalf("replace category = %v", got["primary_category"])
	}
	if listed := stringList(got["keywords"]); !sameStrings(listed, replaced) {
		t.Fatalf("replace keywords = %v", listed)
	}
}

func decodeProduct(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var product map[string]any
	if err := json.Unmarshal(body, &product); err != nil {
		t.Fatal(err)
	}
	return product
}

func productByID(t *testing.T, body []byte, id string) map[string]any {
	t.Helper()
	var payload struct {
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	for _, product := range payload.Products {
		if product["id"] == id {
			return product
		}
	}
	t.Fatalf("product %s missing from list", id)
	return nil
}

func assertTaxonomy(t *testing.T, when string, product map[string]any, keywords []string) {
	t.Helper()
	if product["primary_category"] != "developer tools" {
		t.Fatalf("%s category = %v", when, product["primary_category"])
	}
	if got := stringList(product["keywords"]); !sameStrings(got, keywords) {
		t.Fatalf("%s keywords = %v", when, got)
	}
}

func stringList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
