package entry

import (
	"strings"
	"testing"
)

func validListing() string {
	return `{
		"product_id": "11111111-1111-1111-1111-111111111111",
		"card": {
			"name": "Prompt Forge",
			"summary": "A writing tool for developers.",
			"category": "devtools",
			"features": ["templates"],
			"pricing_summary": "Free tier"
		},
		"signals": [{"name": "payments", "pass": true}, {"name": "offer_link", "pass": false}],
		"score": 72,
		"payments_check": "pass",
		"offer_link_health": "broken"
	}`
}

func TestListingUploadAcceptsCard(t *testing.T) {
	got, err := ParseListingUpload([]byte(validListing()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != 72 || got.PaymentsCheck != "pass" || got.OfferLinkHealth != "broken" || len(got.Signals) != 2 {
		t.Fatalf("%+v", got)
	}
	if !got.Signals[0].Pass || got.Signals[1].Pass {
		t.Fatalf("signals: %+v", got.Signals)
	}
}

func TestListingUploadRejectsSchemaAndSecrets(t *testing.T) {
	cases := []string{
		strings.Replace(validListing(), `"score": 72`, `"score": 72, "repo": "package main"`, 1),
		strings.Replace(validListing(), "A writing tool for developers.", "sk_live_notarealsecret", 1),
		strings.Replace(validListing(), "templates", "func main() {", 1),
		strings.Replace(validListing(), `"score": 72`, `"score": 140`, 1),
		strings.Replace(validListing(), `"payments_check": "pass"`, `"payments_check": "maybe"`, 1),
		strings.Replace(validListing(), `"signals": [{"name": "payments", "pass": true}, {"name": "offer_link", "pass": false}]`, `"signals": []`, 1),
		`{"card":{"name":"n","summary":"` + strings.Repeat("x", 281) + `","category":"c","features":[],"pricing_summary":""},"signals":[{"name":"payments","pass":true}],"score":1,"payments_check":"unknown","offer_link_health":"unknown"}`,
		strings.Replace(validListing(), "A writing tool for developers.", "<?php echo $key;", 1),
	}
	for i, body := range cases {
		if _, err := ParseListingUpload([]byte(body)); err == nil {
			t.Fatalf("case %d was accepted", i)
		}
	}
	if _, err := ParseListingUpload([]byte(strings.Repeat("a", listingBodyLimit+1))); err == nil {
		t.Fatal("oversize body accepted")
	}
}
