package orp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMatchExactProductRanksFirstAndHidesPrivate(t *testing.T) {
	product := Candidate{
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Source: "product", Name: "TEST Zqtesticle",
		Website: "https://zqtesticle.example", Blurb: "write owner@agents.invalid",
		Category: "Tools", OwnerID: "11111111-1111-1111-1111-111111111111",
		Keywords: []string{"secret-keyword"},
	}
	program := Candidate{
		ID: "prog", Source: "program", Name: "AAA TEST Zqtesticle",
		Blurb: "TEST Zqtesticle catalog", Website: "https://other.example",
		CommissionUnit: "percent", CommissionType: "percent", CommissionValue: 20, Currency: "USD",
	}
	items, empty := Match("127.0.0.1:8080", SuggestRequest{Text: "TEST Zqtesticle", Haystack: "TEST Zqtesticle", Limit: 5}, []Candidate{program, product})
	if empty != "" || len(items) < 2 {
		t.Fatalf("items %d empty %q", len(items), empty)
	}
	first, _ := items[0].(map[string]any)
	card, _ := first["product"].(map[string]any)
	if card["name"] != "TEST Zqtesticle" {
		t.Fatalf("first %#v", first)
	}
	if _, ok := card["tagline"]; ok {
		t.Fatalf("tagline with an email was kept: %#v", card)
	}
	if card["website"] != "https://zqtesticle.example" || card["category"] != "tools" {
		t.Fatalf("card %#v", card)
	}
	if first["pool"] != "product" {
		t.Fatalf("pool %#v", first["pool"])
	}
	party, _ := first["party"].(string)
	if !strings.HasPrefix(party, "orp:127.0.0.1:8080/") {
		t.Fatalf("party %s", party)
	}
	fit, _ := first["fit"].(map[string]any)
	if fit["score"] != 100 {
		t.Fatalf("score %#v", fit)
	}
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, leak := range []string{"owner@", "secret-keyword", "offer", "space", "email", "docs_url", "payment_provider", "pricing_url", "keywords"} {
		if strings.Contains(body, leak) {
			t.Fatalf("product card leaked %s in %s", leak, body)
		}
	}
	if _, ok := first["offer"]; ok {
		t.Fatal("product card has an offer")
	}
	if _, ok := first["space"]; ok {
		t.Fatal("product card has a space")
	}
	second, _ := items[1].(map[string]any)
	if _, ok := second["offer"]; !ok {
		t.Fatalf("catalog pitch lost its offer: %#v", second)
	}
}

func TestPromoteExactProductBeforeFreshPitches(t *testing.T) {
	product := map[string]any{"product": map[string]any{"id": "p1", "name": "TEST Zqtesticle"}, "pitch": "Registered product: TEST Zqtesticle."}
	offer := map[string]any{"offer": map[string]any{"id": "o1"}, "pitch": "A listed offer."}
	items := PromoteExactProduct([]any{offer, product}, "TEST Zqtesticle")
	first, _ := items[0].(map[string]any)
	card, _ := first["product"].(map[string]any)
	if card["name"] != "TEST Zqtesticle" {
		t.Fatalf("exact product was not first: %#v", items)
	}
}
