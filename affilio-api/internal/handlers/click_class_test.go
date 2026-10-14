package handlers

import (
	"net/http"
	"testing"
)

const (
	testBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	testFirefoxUA = "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0"
	testSafariUA  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15"
)

func TestClassifyClick(t *testing.T) {
	cases := []struct {
		name   string
		method string
		ua     string
		header map[string]string
		want   string
		paid   bool
		ref    bool
	}{
		{name: "chrome", method: http.MethodGet, ua: testBrowserUA, want: ClickClassBrowser, paid: true, ref: true},
		{name: "firefox", method: http.MethodGet, ua: testFirefoxUA, want: ClickClassBrowser, paid: true, ref: true},
		{name: "safari", method: http.MethodGet, ua: testSafariUA, want: ClickClassBrowser, paid: true, ref: true},
		{name: "mozilla without an engine", method: http.MethodGet, ua: "Mozilla/5.0 reader", want: ClickClassUnknown},
		{name: "mozilla alone", method: http.MethodGet, ua: "Mozilla/5.0", want: ClickClassUnknown},
		{name: "reader product", method: http.MethodGet, ua: "ForwardedReader/9", want: ClickClassUnknown},
		{name: "curl", method: http.MethodGet, ua: "curl/8.5.0", want: ClickClassUnknown},
		{name: "gptbot", method: http.MethodGet, ua: "Mozilla/5.0 (compatible; GPTBot/1.2; +https://openai.com/gptbot)", want: ClickClassVerifiedAgent, ref: true},
		{name: "claudebot", method: http.MethodGet, ua: "ClaudeBot/1.0", want: ClickClassVerifiedAgent, ref: true},
		{name: "perplexity", method: http.MethodGet, ua: "PerplexityBot/1.0", want: ClickClassVerifiedAgent, ref: true},
		{name: "google extended", method: http.MethodGet, ua: "Mozilla/5.0 (compatible; Google-Extended)", want: ClickClassVerifiedAgent, ref: true},
		{name: "oai search", method: http.MethodGet, ua: "OAI-SearchBot/1.0", want: ClickClassVerifiedAgent, ref: true},
		{name: "declared header", method: http.MethodGet, ua: "Mozilla/5.0", header: map[string]string{"X-Affilio-Agent": "1"}, want: ClickClassDeclaredAgent, ref: true},
		{name: "declared ua", method: http.MethodGet, ua: "AffilioAgent/0.4", want: ClickClassDeclaredAgent, ref: true},
		{name: "safe links", method: http.MethodGet, ua: "Mozilla/5.0 (compatible; Microsoft SafeLinks)", want: ClickClassUnknown},
		{name: "proofpoint", method: http.MethodGet, ua: "Proofpoint/1.0", want: ClickClassUnknown},
		{name: "mimecast", method: http.MethodGet, ua: "Mimecast/1.0", want: ClickClassUnknown},
		{name: "barracuda", method: http.MethodGet, ua: "Barracuda/1.0", want: ClickClassUnknown},
		{name: "twitter preview", method: http.MethodGet, ua: "Twitterbot/1.0", want: ClickClassUnknown},
		{name: "head", method: http.MethodHead, ua: testBrowserUA, want: ClickClassUnknown},
		{name: "prefetch", method: http.MethodGet, ua: testBrowserUA, header: map[string]string{"Purpose": "prefetch"}, want: ClickClassUnknown},
		{name: "sec purpose", method: http.MethodGet, ua: testBrowserUA, header: map[string]string{"Sec-Purpose": "prefetch"}, want: ClickClassUnknown},
		{name: "empty", method: http.MethodGet, ua: "", want: ClickClassUnknown},
		{name: "blank", method: http.MethodGet, ua: "   ", want: ClickClassUnknown},
		{name: "head of gptbot", method: http.MethodHead, ua: "GPTBot/1.0", want: ClickClassVerifiedAgent, ref: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			for key, value := range tc.header {
				header.Set(key, value)
			}
			got := ClassifyClick(tc.method, tc.ua, header)
			if got != tc.want {
				t.Fatalf("class %s, want %s", got, tc.want)
			}
			if PaidClick(got) != tc.paid {
				t.Fatalf("paid %v for %s", PaidClick(got), got)
			}
			if AttributableClick(got) != tc.ref {
				t.Fatalf("ref %v for %s", AttributableClick(got), got)
			}
		})
	}
}
