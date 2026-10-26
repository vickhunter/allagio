package orp

import (
	"strings"
	"testing"
	"time"
)

func TestValidatePartiesSplitsAndHolders(t *testing.T) {
	offerer := "orp:localhost/p_offerer01"
	other := "orp:localhost/p_acceptor1"
	pair := func(left, right map[string]any) []any {
		return []any{
			map[string]any{"id": offerer, "role": "offerer", "required": true, "terms": left},
			map[string]any{"id": other, "role": "acceptor", "required": true, "terms": right},
		}
	}
	eur := func(n int) map[string]any {
		return map[string]any{"amount_minor": n, "currency": "EUR"}
	}
	bad := func(parties []any, needle string) {
		t.Helper()
		problem, failed := ValidateParties(parties, "orp:localhost/host")
		if !failed || !strings.Contains(problem.Message, needle) {
			t.Fatalf("want %q got %+v failed=%v", needle, problem, failed)
		}
	}
	ok := func(parties []any) {
		t.Helper()
		problem, failed := ValidateParties(parties, "orp:localhost/host")
		if failed {
			t.Fatalf("rejected %+v", problem)
		}
	}
	bad(pair(map[string]any{"percent": 60}, map[string]any{"percent": 41}), "100")
	bad(pair(map[string]any{"percent": 40, "hours": 2, "cap": eur(1000)}, map[string]any{"percent": 40, "hours": 1}), "100")
	bad(pair(map[string]any{"percent": 40, "amount": eur(500)}, map[string]any{"amount": eur(500)}), "cap")
	bad(pair(map[string]any{"amount": eur(0)}, map[string]any{"deposit": map[string]any{"amount": eur(2000), "holder": "OfferRails", "purpose": "viewing"}}), "does not hold money")
	bad(pair(map[string]any{"amount": eur(0)}, map[string]any{"deposit": map[string]any{"amount": eur(2000), "holder": "orp:localhost/host", "purpose": "viewing"}}), "does not hold money")
	ok(pair(map[string]any{"percent": 50, "hours": 2}, map[string]any{"percent": 50, "per": "hour"}))
	ok(pair(map[string]any{"percent": 50}, map[string]any{"percent": 50, "deposit": map[string]any{"amount": eur(2000), "holder": "Viewing Desk", "purpose": "viewing"}}))
	ok(pair(map[string]any{"percent": 40, "amount": eur(500), "cap": eur(1000)}, map[string]any{"percent": 40, "amount": eur(500), "cap": eur(1000)}))
	if HolderKey("Offer.Rails") != "offerrails" || HolderKey("hosted api") != "hostedapi" {
		t.Fatal("holder key")
	}
}

func TestSignatureStateAndExpiry(t *testing.T) {
	seats := []Seat{
		{ID: "a", Required: true, Signed: true},
		{ID: "b", Required: false, Signed: false},
	}
	if SignatureState(seats, nil) != "active" {
		t.Fatal("optional blocks")
	}
	if SignatureState(seats, []Vacancy{{ID: "vac_1", Role: "facilitator"}}) != "pending" {
		t.Fatal("vacancy ignored")
	}
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	terms := NormalizeClaims(map[string]any{"title": "Notes", "claims": map[string]any{"commission": 30, "destination": "https://notes.example"}}, now)
	if terms["claims"].(map[string]any)["commission"] != "unknown" || terms["claims"].(map[string]any)["destination"] != "unknown" {
		t.Fatalf("claims %#v", terms["claims"])
	}
	if _, ok := terms["claims"].(map[string]any)["audience"]; ok {
		t.Fatal("invented audience")
	}
	checked, _ := time.Parse(time.RFC3339, terms["checked_at"].(string))
	expires, _ := time.Parse(time.RFC3339, terms["expires_at"].(string))
	if expires.Sub(checked) != freshWindow {
		t.Fatalf("window %s", expires.Sub(checked))
	}
	if OfferExpired(terms, expires.Add(-time.Second)) || !OfferExpired(terms, expires) {
		t.Fatal("expiry boundary")
	}
	if OfferExpired(map[string]any{"title": "classic"}, expires) {
		t.Fatal("0.3 offer expired")
	}
}
