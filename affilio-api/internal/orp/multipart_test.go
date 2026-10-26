package orp

import (
	"encoding/json"
	"math/rand"
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
	cubot := "Mozilla/5.0 (Linux; Android 11; CUBOT NOTE 20) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.104 Mobile Safari/537.36"
	safariPreview := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (preview)"
	iphone := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	firefox := "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
	edge := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"
	samsung := "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36"
	opera := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 OPR/116.0.0.0"
	abbott := "Mozilla/5.0 (Linux; Android 10; Abbott) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	robotek := "Mozilla/5.0 (Linux; Android 10; Robotek) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	brave := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Brave/1.73.91"
	ddg := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1 Ddg/7.140.0"
	instagram := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 321.0.0.0.0 (iPhone15,3; iOS 17_4; en_US; en; scale=3.00; 1290x2796; 123456789)"
	facebook := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/450.0.0.29.118;FBBV/123456;FBDV/iPhone15,3;FBMD/iPhone;FBSN/iOS;FBSV/17.4;FBSS/3;FBID/phone;FBLC/en_US;FBOP/5]"
	linkedin := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [LinkedInApp]/9.29.5093"
	telegramWeb := "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/131.0.0.0 Mobile Safari/537.36 Telegram-Android/11.1.2"
	whatsappChrome := "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/131.0.0.0 Mobile Safari/537.36 WhatsApp/2.24.20.78"
	applebotMac := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)"
	applebotPhone := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1 (Applebot/0.1; +http://www.apple.com/go/applebot)"
	cases := []struct {
		name, method, ua, purpose, query, want string
	}{
		{"empty user agent", "GET", "", "", "", "bot"},
		{"whitespace user agent", "GET", "  \t  ", "", "", "bot"},
		{"cubot device", "GET", cubot, "", "", "human"},
		{"safari preview comment", "GET", safariPreview, "", "", "human"},
		{"chrome 131", "GET", chrome, "", "", "human"},
		{"iphone safari", "GET", iphone, "", "", "human"},
		{"firefox 128", "GET", firefox, "", "", "human"},
		{"edge 131", "GET", edge, "", "", "human"},
		{"samsung internet", "GET", samsung, "", "", "human"},
		{"opera", "GET", opera, "", "", "human"},
		{"abbott device", "GET", abbott, "", "", "human"},
		{"robotek device", "GET", robotek, "", "", "human"},
		{"brave", "GET", brave, "", "", "human"},
		{"duckduckgo", "GET", ddg, "", "", "human"},
		{"instagram", "GET", instagram, "", "", "human"},
		{"facebook in app", "GET", facebook, "", "", "human"},
		{"linkedin in app", "GET", linkedin, "", "", "human"},
		{"telegram webview", "GET", telegramWeb, "", "", "human"},
		{"whatsapp opened chrome", "GET", whatsappChrome, "", "", "human"},
		{"nodebb", "GET", "nodebb/1.2.3", "", "", "human"},
		{"comment word u", "GET", "Mozilla/5.0 (U) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "", "", "human"},
		{"preview product name", "GET", "Slack-Preview/1.0", "", "", "human"},
		{"unclosed cubot", "GET", "(Linux; Android 11; CUBOT NOTE 20", "", "", "human"},
		{"unclosed khtml", "GET", "(KHTML, like Gecko", "", "", "human"},
		{"head", "HEAD", chrome, "", "", "preview"},
		{"head with googlebot", "HEAD", "Googlebot/2.1", "", "", "preview"},
		{"head with cubot", "HEAD", cubot, "", "", "preview"},
		{"prefetch", "GET", chrome, "prefetch", "", "preview"},
		{"prerender", "GET", chrome, "prerender", "", "preview"},
		{"orp test stays test", "GET", "curl/8.0.1", "", "orp_test=1", "test"},
		{"googlebot", "GET", "Googlebot/2.1", "", "", "bot"},
		{"googlebot url", "GET", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "", "", "bot"},
		{"bot html", "GET", "+http://www.google.com/bot.html", "", "", "bot"},
		{"twitterbot", "GET", "Twitterbot/1.0", "", "", "bot"},
		{"bingbot", "GET", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", "", "", "bot"},
		{"curl", "GET", "curl/8.0.1", "", "", "bot"},
		{"wget", "GET", "Wget/1.21", "", "", "bot"},
		{"python requests", "GET", "python-requests/2.31.0", "", "", "bot"},
		{"python urllib", "GET", "Python-urllib/3.11", "", "", "bot"},
		{"go http client", "GET", "Go-http-client/1.1", "", "", "bot"},
		{"okhttp", "GET", "okhttp/4.12.0", "", "", "bot"},
		{"libwww", "GET", "libwww-perl/6.72", "", "", "bot"},
		{"headless chrome", "GET", "Mozilla/5.0 HeadlessChrome/120.0.0.0", "", "", "bot"},
		{"facebook", "GET", "facebookexternalhit/1.1", "", "", "bot"},
		{"slackbot", "GET", "Slackbot-LinkExpanding 1.0", "", "", "bot"},
		{"whatsapp", "GET", "WhatsApp/2.23.20", "", "", "bot"},
		{"telegram", "GET", "TelegramBot (like TwitterBot)", "", "", "bot"},
		{"discord", "GET", "Discordbot/2.0", "", "", "bot"},
		{"discord compatible", "GET", "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", "", "", "bot"},
		{"scrapy", "GET", "Scrapy/2.11.0", "", "", "bot"},
		{"axios", "GET", "axios/1.6.0", "", "", "bot"},
		{"node fetch", "GET", "node-fetch/3.3.2", "", "", "bot"},
		{"postman", "GET", "PostmanRuntime/7.36.0", "", "", "bot"},
		{"crawler", "GET", "generic-crawler/1.0", "", "", "bot"},
		{"spider", "GET", "Mozilla/5.0 (compatible; Baiduspider/2.0)", "", "", "bot"},
		{"slurp", "GET", "Mozilla/5.0 (compatible; Yahoo! Slurp)", "", "", "bot"},
		{"applebot mac", "GET", applebotMac, "", "", "bot"},
		{"applebot iphone", "GET", applebotPhone, "", "", "bot"},
		{"applebot compatible", "GET", "Mozilla/5.0 (compatible; Applebot/0.1; +http://www.apple.com/go/applebot)", "", "", "bot"},
		{"unclosed pixel bot", "GET", "(Linux; Pixel bot/1.0", "", "", "bot"},
		{"unclosed android pixel bot", "GET", "(Linux; Android 11; Pixel bot/1.0", "", "", "bot"},
		{"mediapartners", "GET", "Mozilla/5.0 (compatible; Mediapartners-Google/2.1; +http://www.google.com/bot.html)", "", "", "bot"},
		{"adsbot", "GET", "AdsBot-Google (+http://www.google.com/adsbot.html)", "", "", "bot"},
		{"adsbot mobile", "GET", "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.69 Mobile Safari/537.36 (compatible; AdsBot-Google-Mobile; +http://www.google.com/mobile/adsbot.html)", "", "", "bot"},
		{"skype preview", "GET", "SkypeUriPreview Preview/0.5", "", "", "bot"},
		{"java", "GET", "Java/17.0.1", "", "", "bot"},
		{"apache httpclient", "GET", "Apache-HttpClient/4.5.13 (Java/11.0.11)", "", "", "bot"},
		{"iframely", "GET", "Iframely/1.3.1 (+https://iframely.com/docs/about)", "", "", "bot"},
		{"node", "GET", "node", "", "", "bot"},
		{"node version", "GET", "node/20.11.0", "", "", "bot"},
		{"undici", "GET", "undici", "", "", "bot"},
		{"undici version", "GET", "undici/6.19.8", "", "", "bot"},
		{"google inspection", "GET", "Mozilla/5.0 (compatible; Google-InspectionTool/1.0;)", "", "", "bot"},
		{"google read aloud", "GET", "Mozilla/5.0 (Linux; Android 7.0; SM-G930V Build/NRD90M) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.125 Mobile Safari/537.36 (compatible; Google-Read-Aloud; +https://support.google.com/webmasters/answer/1061943)", "", "", "bot"},
		{"storebot", "GET", "Mozilla/5.0 (X11; Linux x86_64; Storebot-Google/1.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36", "", "", "bot"},
		{"lighthouse", "GET", "Lighthouse/10.4.0", "", "", "bot"},
		{"chrome lighthouse", "GET", "Mozilla/5.0 (Linux; Android 7.0; Moto G (4)) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4590.2 Mobile Safari/537.36 Chrome-Lighthouse", "", "", "bot"},
		{"ptst", "GET", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 PTST/240201.144200", "", "", "bot"},
		{"pingdom", "GET", "Pingdom.com_bot_version_1.4_(http://www.pingdom.com/)", "", "", "bot"},
		{"uptimerobot", "GET", "Mozilla/5.0 (compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)", "", "", "bot"},
		{"datadog", "GET", "Datadog/Synthetics", "", "", "bot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClickClass(tc.method, tc.ua, tc.purpose, tc.query); got != tc.want {
				t.Fatalf("ClickClass(%q, %q) = %s, want %s", tc.method, tc.ua, got, tc.want)
			}
		})
	}
}

func TestClickClassFuzzNoPanic(t *testing.T) {
	const size = 100 * 1024
	rng := rand.New(rand.NewSource(20261026))
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte(rng.Intn(95) + 32)
	}
	for i := 0; i+8 < len(buf); i += 97 {
		copy(buf[i:], []byte("(bot/1;"))
	}
	start := time.Now()
	check := func(ua string) {
		t.Helper()
		switch got := ClickClass("GET", ua, "", ""); got {
		case "bot", "human", "preview", "test":
		default:
			t.Fatalf("class %s", got)
		}
	}
	check(string(buf))
	for off := 0; off < len(buf); off += 4096 {
		end := off + 4096
		if end > len(buf) {
			end = len(buf)
		}
		check(string(buf[off:end]))
	}
	check(strings.Repeat("(", size))
	check(strings.Repeat("(Linux; Android 11; CUBOT NOTE 20", 2048))
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("100KB scan took %s", elapsed)
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
