package entry

import (
	"os"
	"path/filepath"
	"strings"
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
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<html><body><script>var x = \"</style>\"; var n = 20;</script><p>no number</p></body></html>"}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("a fake style closer inside a script must not leak the commission: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<html><body><script>var n = 20;<p>no number</p></body></html>"}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("an unclosed script must not supply the commission: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<html><body><script>var t = '<meta content=\"Northwind Ledger\" property=\"og:title\">';</script><div id=\"root\"></div></body></html>"}, in)
	if reason != PageReasonCard {
		t.Fatalf("a meta tag inside a script must not satisfy the card: %q", reason)
	}
	split := "<html><head><title>Northwind</title><meta name=\"description\" content=\"Ledger software for small publishers.\"></head><body><div id=\"root\"></div></body></html>"
	reason, _ = checker.Check(PageDocument{OK: true, Body: split}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != PageReasonCard {
		t.Fatalf("a title and the next description must not join into the card name: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: strings.Repeat("İ", 40) + "<script>var n = 99;</script><meta name=\"description\" content=\"Publishers earn 20 percent.\">"}, PageClaimInput{CommissionValue: 20})
	if reason != "" {
		t.Fatalf("unicode before a script: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<script src=\"/type=application/ld+json\">var n = 20;</script>"}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("a src path must not make a bundle into JSON-LD: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<p>For teams < 50, Northwind Ledger is here ></p>"}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a comparison sign must not swallow the claim: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<title>For teams < 50</title><meta name=\"description\" content=\"Northwind Ledger\">"}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a less-than in the title must not swallow the meta claim: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<script>var x = \"</scripting>\"; var n = 20;</script><p>no number</p>"}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("scripting closer must not leak the commission: %q", reason)
	}
	commented := "<!-- <script src=\"a.js\"> --><p>Northwind Ledger</p><script>var n = 20;</script>"
	reason, _ = checker.Check(PageDocument{OK: true, Body: commented}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a script inside a comment must not hide the page: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: commented}, PageClaimInput{CommissionValue: 20})
	if reason != PageReasonCommission {
		t.Fatalf("a script inside a comment must not keep the bundle: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<meta name=\"description\" content=\"Northwind Ledger > extra\">"}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a greater-than inside a meta value must stay: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<p>Northwind&#160;Ledger</p>"}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a character reference must match the card name: %q", reason)
	}
	reason, _ = checker.Check(PageDocument{OK: true, Body: "<p>Plans start at &#36;29 a month</p>"}, PageClaimInput{Card: ListingCard{PricingSummary: "Plans start at $29 a month"}, HasCard: true})
	if reason != "" {
		t.Fatalf("a numeric character reference must match the price: %q", reason)
	}
	for _, body := range []string{
		`<script type="application/ld+json">{"z":"Ledger","a":"Northwind"}</script>`,
		`<script type="application/ld+json">{"a":"Northwind","z":"Ledger"}</script>`,
	} {
		first, _ := checker.Check(PageDocument{OK: true, Body: body}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
		second, _ := checker.Check(PageDocument{OK: true, Body: body}, PageClaimInput{Card: ListingCard{Name: "Northwind Ledger"}, HasCard: true})
		if first != PageReasonCard || second != first {
			t.Fatalf("json-ld values joined into the card name: %q then %q", first, second)
		}
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
