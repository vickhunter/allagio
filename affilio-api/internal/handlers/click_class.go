package handlers

import (
	"net/http"
	"strings"
)

const (
	ClickClassVerifiedAgent = "verified_agent"
	ClickClassDeclaredAgent = "declared_agent"
	ClickClassBrowser       = "browser"
	ClickClassUnknown       = "unknown"
	// ClickClassTest is a hit on a demo site. It is stored and it is not paid.
	ClickClassTest = "test"
)

// aiCrawlerMarkers are known AI fetchers. A hit from one of these is a
// verified agent: it can still carry a sale, and it is not a paid click.
var aiCrawlerMarkers = []string{
	"gptbot",
	"chatgpt-user",
	"oai-searchbot",
	"claudebot",
	"claude-web",
	"anthropic-ai",
	"perplexitybot",
	"perplexity-user",
	"google-extended",
	"googleother",
	"ccbot",
	"bytespider",
	"amazonbot",
	"meta-externalagent",
	"facebookbot",
	"cohere-ai",
	"youbot",
	"diffbot",
	"applebot-extended",
}

// emailScannerMarkers are link scanners and prefetchers that follow a URL
// before a person does. They are not paid clicks and they do not earn a ref.
var emailScannerMarkers = []string{
	"safelinks",
	"proofpoint",
	"urldefense",
	"mimecast",
	"barracuda",
	"barracudacentral",
	"trendmicro",
	"fireeye",
	"symantec",
	"messagelabs",
	"ppbot",
}

// browserEngineTokens are the engine names a real browser sends next to
// Mozilla/. A User-Agent that only says Mozilla, or that names no engine,
// is not a browser.
var browserEngineTokens = []string{
	"applewebkit",
	"chrome/",
	"crios/",
	"firefox/",
	"fxios/",
	"safari/",
	"edg/",
	"edgios/",
	"opr/",
	"opera/",
	"gecko/",
	"trident/",
}

// ClassifyClick labels a go-link hit. Only a browser click is paid.
// Verified and declared agents stay attributable so a completed sale still
// earns commission. An empty User-Agent, a non-browser client, scanners,
// prefetch, and HEAD are unknown.
func ClassifyClick(method, userAgent string, header http.Header) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if containsMarker(ua, aiCrawlerMarkers) {
		return ClickClassVerifiedAgent
	}
	if method == http.MethodHead || isPrefetch(header) || containsMarker(ua, emailScannerMarkers) || isPreviewBot(userAgent) {
		return ClickClassUnknown
	}
	if declaredAgent(ua, header) {
		return ClickClassDeclaredAgent
	}
	if !looksLikeBrowser(ua) {
		return ClickClassUnknown
	}
	return ClickClassBrowser
}

func looksLikeBrowser(ua string) bool {
	if ua == "" || !strings.Contains(ua, "mozilla/") {
		return false
	}
	return containsMarker(ua, browserEngineTokens)
}

// PaidClick is the pay-per-click rule: a browser click counts.
// verified_agent, declared_agent, unknown, and test do not.
func PaidClick(class string) bool {
	return class == ClickClassBrowser
}

// AttributableClick is a hit a later sale can join. Scanners and prefetch
// stay on the plain URL.
func AttributableClick(class string) bool {
	switch class {
	case ClickClassBrowser, ClickClassVerifiedAgent, ClickClassDeclaredAgent:
		return true
	default:
		return false
	}
}

func containsMarker(ua string, markers []string) bool {
	if ua == "" {
		return false
	}
	for _, marker := range markers {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
}

func isPrefetch(header http.Header) bool {
	if header == nil {
		return false
	}
	for _, key := range []string{"Purpose", "X-Purpose", "Sec-Purpose", "X-Moz"} {
		value := strings.ToLower(strings.TrimSpace(header.Get(key)))
		if value == "" {
			continue
		}
		if strings.Contains(value, "prefetch") || strings.Contains(value, "preview") {
			return true
		}
	}
	return false
}

func declaredAgent(ua string, header http.Header) bool {
	if header != nil && strings.TrimSpace(header.Get("X-Affilio-Agent")) != "" {
		return true
	}
	return strings.Contains(ua, "affilioagent")
}
