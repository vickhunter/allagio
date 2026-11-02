package orp

import (
	"strings"
	"testing"
)

func TestProofPageSaysAllagioDoesNotHoldMoney(t *testing.T) {
	const sentence = "Allagio does not hold money"
	pages := []string{
		ProofPage("Affiliate link", "awaiting", false, nil),
		ProofPage("Affiliate link", "active", false, nil),
		ProofPage("Affiliate link", "active", true, nil),
	}
	for _, page := range pages {
		if !strings.Contains(page, sentence) {
			t.Fatalf("proof page missing %q: %s", sentence, page)
		}
	}
}
