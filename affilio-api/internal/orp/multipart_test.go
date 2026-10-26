package orp

import (
	"encoding/json"
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

func TestClickReadout(t *testing.T) {
	seats := []Seat{
		{ID: "orp:h/a", Terms: map[string]any{"percent": 50}},
		{ID: "orp:h/b", Terms: map[string]any{"percent": 30}},
		{ID: "orp:h/c", Terms: map[string]any{"percent": 20}},
	}
	split := []any{
		map[string]any{"party": "orp:h/a", "percent": float64(50)},
		map[string]any{"party": "orp:h/b", "percent": float64(30)},
		map[string]any{"party": "orp:h/c", "percent": float64(20)},
	}
	doc := Many{Seats: seats, Clicks: []Click{
		{Obligation: "place", Class: "bot", Shares: split},
		{Obligation: "place", Class: "preview", Shares: split},
		{Obligation: "place", Class: "test", Shares: split},
		{Obligation: "place", Class: "human", Shares: split},
		{Obligation: "other", Class: "human", Shares: split},
		{Obligation: "place", Class: "human", Shares: split},
	}}
	clicks, share := ClickReadout(doc, "place")
	if clicks != 2 {
		t.Fatalf("clicks %d", clicks)
	}
	raw, err := json.Marshal(share)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"percent":1.0`) || !strings.Contains(text, `"percent":0.6`) || !strings.Contains(text, `"percent":0.4`) {
		t.Fatalf("share %s", text)
	}
	zeroClicks, zeroShare := ClickReadout(Many{Seats: seats}, "place")
	zeroRaw, err := json.Marshal(zeroShare)
	if err != nil || zeroClicks != 0 || !strings.Contains(string(zeroRaw), `"percent":0.0`) {
		t.Fatalf("zero %d %s %v", zeroClicks, zeroRaw, err)
	}
}

func TestClickClass(t *testing.T) {
	chrome := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	safari := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
	firefox := "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
	edge := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"
	cases := []struct {
		name, method, ua, purpose, query, want string
	}{
		{"empty user agent", "GET", "", "", "", "bot"},
		{"chrome", "GET", chrome, "", "", "human"},
		{"safari", "GET", safari, "", "", "human"},
		{"firefox", "GET", firefox, "", "", "human"},
		{"edge", "GET", edge, "", "", "human"},
		{"head", "HEAD", chrome, "", "", "preview"},
		{"head with googlebot", "HEAD", "Googlebot/2.1", "", "", "preview"},
		{"prefetch", "GET", chrome, "prefetch", "", "preview"},
		{"prerender", "GET", chrome, "prerender", "", "preview"},
		{"orp test stays test", "GET", "curl/8.0.1", "", "orp_test=1", "test"},
		{"googlebot", "GET", "Googlebot/2.1", "", "", "bot"},
		{"twitterbot", "GET", "Twitterbot/1.0", "", "", "bot"},
		{"curl", "GET", "curl/8.0.1", "", "", "bot"},
		{"wget", "GET", "Wget/1.21", "", "", "bot"},
		{"python requests", "GET", "python-requests/2.31.0", "", "", "bot"},
		{"python urllib", "GET", "Python-urllib/3.11", "", "", "bot"},
		{"go http client", "GET", "Go-http-client/1.1", "", "", "bot"},
		{"java httpclient", "GET", "Apache-HttpClient/4.5.14", "", "", "bot"},
		{"okhttp", "GET", "okhttp/4.12.0", "", "", "bot"},
		{"libwww", "GET", "libwww-perl/6.72", "", "", "bot"},
		{"headless chrome", "GET", "Mozilla/5.0 HeadlessChrome/120.0.0.0", "", "", "bot"},
		{"facebook", "GET", "facebookexternalhit/1.1", "", "", "bot"},
		{"slackbot", "GET", "Slackbot-LinkExpanding 1.0", "", "", "bot"},
		{"whatsapp", "GET", "WhatsApp/2.23.20", "", "", "bot"},
		{"telegram", "GET", "TelegramBot (like TwitterBot)", "", "", "bot"},
		{"discord", "GET", "Discordbot/2.0", "", "", "bot"},
		{"preview", "GET", "Slack-Preview/1.0", "", "", "bot"},
		{"scrapy", "GET", "Scrapy/2.11.0", "", "", "bot"},
		{"axios", "GET", "axios/1.6.0", "", "", "bot"},
		{"node fetch", "GET", "node-fetch/3.3.2", "", "", "bot"},
		{"postman", "GET", "PostmanRuntime/7.36.0", "", "", "bot"},
		{"crawler", "GET", "generic-crawler/1.0", "", "", "bot"},
		{"spider", "GET", "Mozilla/5.0 (compatible; Baiduspider/2.0)", "", "", "bot"},
		{"slurp", "GET", "Mozilla/5.0 (compatible; Yahoo! Slurp)", "", "", "bot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClickClass(tc.method, tc.ua, tc.purpose, tc.query); got != tc.want {
				t.Fatalf("ClickClass(%q, %q) = %s, want %s", tc.method, tc.ua, got, tc.want)
			}
		})
	}
	for _, token := range automationUA {
		ua := "Check-" + token + "-1"
		if got := ClickClass("GET", ua, "", ""); got != "bot" {
			t.Fatalf("token %q ua %q class %s", token, ua, got)
		}
		upper := strings.ToUpper(ua)
		if got := ClickClass("GET", upper, "", ""); got != "bot" {
			t.Fatalf("token %q upper %q class %s", token, upper, got)
		}
		if got := ClickClass("HEAD", ua, "", ""); got != "preview" {
			t.Fatalf("HEAD token %q class %s", token, got)
		}
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
