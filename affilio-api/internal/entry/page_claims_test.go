package entry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPageClaimsAgainstFixtures(t *testing.T) {
	card := ListingCard{
		Name:           "Northwind Ledger",
		Summary:        "Ledger software for small publishers.",
		Category:       "software",
		Features:       []string{"invoices"},
		PricingSummary: "Plans start at $29 a month",
	}
	terms := "Northwind Ledger pays 20 percent to publishers."
	match := PageClaimInput{Card: card, HasCard: true, Terms: terms, CommissionValue: 20}

	cases := []struct {
		name   string
		file   string
		ok     bool
		in     PageClaimInput
		reason string
	}{
		{name: "match", file: "merchant_match.html", ok: true, in: match, reason: ""},
		{name: "price", file: "merchant_price.html", ok: true, in: match, reason: PageReasonPrice},
		{name: "commission", file: "merchant_commission.html", ok: true, in: match, reason: PageReasonCommission},
		{name: "card", file: "merchant_card.html", ok: true, in: match, reason: PageReasonCard},
		{name: "terms", file: "merchant_match.html", ok: true, in: PageClaimInput{Card: card, HasCard: true, Terms: "Northwind Ledger pays 30 percent to publishers.", CommissionValue: 20}, reason: PageReasonTerms},
		{name: "unreachable", file: "merchant_match.html", ok: false, in: match, reason: PageReasonUnreachable},
		{name: "no card", file: "merchant_match.html", ok: true, in: PageClaimInput{Terms: terms, CommissionValue: 20}, reason: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := PageDocument{OK: tc.ok, Body: readFixture(t, tc.file)}
			firstReason, firstMessage := CheckPageClaims(doc, tc.in)
			secondReason, secondMessage := CheckPageClaims(doc, tc.in)
			if firstReason != tc.reason || secondReason != firstReason || secondMessage != firstMessage {
				t.Fatalf("CheckPageClaims() = %q %q then %q %q; want %q", firstReason, firstMessage, secondReason, secondMessage, tc.reason)
			}
			if tc.reason != "" && firstMessage != PageClaimMessage(tc.reason) {
				t.Fatalf("message %q", firstMessage)
			}
			if tc.reason == "" && firstMessage != "" {
				t.Fatalf("pass returned %q", firstMessage)
			}
		})
	}
}

func TestPageClaimsReadClientRenderedPages(t *testing.T) {
	card := ListingCard{
		Name:           "Northwind Ledger",
		Summary:        "Ledger software for small publishers.",
		Category:       "software",
		Features:       []string{"invoices"},
		PricingSummary: "Plans start at $29 a month",
	}
	in := PageClaimInput{Card: card, HasCard: true, Terms: "Northwind Ledger pays 20 percent to publishers.", CommissionValue: 20}
	page := clientRenderedNorthwind("Plans start at $29 a month", "20")
	checker := PageClaimChecker{}
	reason, message := checker.Check(PageDocument{OK: true, Body: page}, in)
	if reason != "" || message != "" {
		t.Fatalf("client-rendered page: %q %q", reason, message)
	}
	reason, message = checker.Check(PageDocument{OK: true, Body: clientRenderedNorthwind("Plans start at $49 a month", "20")}, in)
	if reason != PageReasonPrice || message != PageClaimMessage(PageReasonPrice) {
		t.Fatalf("price miss: %q %q", reason, message)
	}
	reason, message = checker.Check(PageDocument{OK: true, Body: "<html><head><script type=\"application/ld+json\">{</script></head><body><div id=\"root\"></div></body></html>"}, in)
	if reason != PageReasonCard || message != PageClaimMessage(PageReasonCard) {
		t.Fatalf("broken page: %q %q", reason, message)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<html><body><div id=\"root\"></div><script>var n = 20;</script></body></html>"}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("a script bundle must not satisfy the commission: %q", reason)
	}
}

func clientRenderedNorthwind(price, commission string) string {
	return `<!DOCTYPE html><html><head><title>App</title>` +
		`<meta content="Northwind Ledger" property="og:title">` +
		`<meta name="og:description" content="Ledger software for small publishers.">` +
		`<meta name="description" content="Category: software">` +
		`<script type="application/ld+json">{"name":"Northwind Ledger","featureList":["invoices"],"offers":{"description":"` + price + `","price":` + commission + `}}</script>` +
		`</head><body><div id="root"></div><script>var bundle = "ignore 99";</script></body></html>`
}

func TestPageClaimsNumberBoundary(t *testing.T) {
	doc := PageDocument{OK: true, Body: "<p>The plan is 20.5 and the year is 2020.</p>"}
	reason, _ := CheckPageClaims(doc, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("20 must not match 20.5 or 2020, got %q", reason)
	}
	reason, message := CheckPageClaims(doc, PageClaimInput{CommissionValue: 20.5})
	if reason != "" || message != "" {
		t.Fatalf("20.5 should pass, got %q %q", reason, message)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
