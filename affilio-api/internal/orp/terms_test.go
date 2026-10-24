package orp

import "testing"

func TestCatalogTermsPercent(t *testing.T) {
	terms, hash, ok := CatalogTerms(Candidate{
		ID:              "prog",
		Source:          "catalog",
		Name:            "Percent Notes",
		Website:         "https://example.com/notes",
		CommissionUnit:  "percent",
		CommissionValue: 12.5,
		CookieDays:      30,
	})
	if !ok {
		t.Fatal("catalog terms refused")
	}
	if hash != TermsHash(terms) {
		t.Fatalf("hash %s", hash)
	}
	if !Places(terms, "acceptor") || Places(terms, "offerer") {
		t.Fatalf("roles %+v", terms["obligations"])
	}
	pay := ObligationMap(terms, "commission")
	if pay["window"] != "P30D" || pay["rate_bps"] != 1250 {
		t.Fatalf("pay %+v", pay)
	}
}

func TestTermsRejectEmailAndBadDuration(t *testing.T) {
	terms := map[string]any{
		"title": "Notes for a@b.co",
		"obligations": []any{map[string]any{
			"id": "place", "action": "place", "by": "acceptor", "proof": []any{"proof_url"},
		}},
		"disclosure": map[string]any{"label": "Affiliate link"},
	}
	problem, bad := CheckTerms(terms, "/body/terms")
	if !bad || problem.Code != "invalid_message" {
		t.Fatalf("email %+v bad=%v", problem, bad)
	}
	pay := map[string]any{
		"title": "Pay",
		"obligations": []any{map[string]any{
			"id": "commission", "action": "pay", "by": "offerer", "per": "conversion",
			"rate_bps": 100, "rail": "manual", "window": "P",
		}},
	}
	if _, bad = CheckTerms(pay, "/body/terms"); !bad {
		t.Fatal("P accepted")
	}
	pay["obligations"].([]any)[0].(map[string]any)["window"] = "PT"
	if _, bad = CheckTerms(pay, "/body/terms"); !bad {
		t.Fatal("PT accepted")
	}
	pay["obligations"].([]any)[0].(map[string]any)["window"] = "P30D"
	if _, bad = CheckTerms(pay, "/body/terms"); bad {
		t.Fatal("P30D refused")
	}
}
