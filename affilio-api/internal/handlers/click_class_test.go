package handlers

import (
	"net/http"
	"testing"
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
		{name: "browser", method: http.MethodGet, ua: "Mozilla/5.0 reader", want: ClickClassBrowser, paid: true, ref: true},
		{name: "reader product", method: http.MethodGet, ua: "ForwardedReader/9", want: ClickClassBrowser, paid: true, ref: true},
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
		{name: "head", method: http.MethodHead, ua: "Mozilla/5.0 reader", want: ClickClassUnknown},
		{name: "prefetch", method: http.MethodGet, ua: "Mozilla/5.0 reader", header: map[string]string{"Purpose": "prefetch"}, want: ClickClassUnknown},
		{name: "sec purpose", method: http.MethodGet, ua: "Mozilla/5.0 reader", header: map[string]string{"Sec-Purpose": "prefetch"}, want: ClickClassUnknown},
		{name: "empty", method: http.MethodGet, ua: "", want: ClickClassBrowser, paid: true, ref: true},
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
