package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
)

func TestPreviewBotsAndHeadRedirectWithoutClicks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newAttributionRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-preview-bots"
	createTestSite(t, db, siteKey, "previews.example.com", "Previews")
	_, join := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	code := join["tracking_code"].(string)

	browser := followGo(r, http.MethodGet, code, "Mozilla/5.0")
	if browser.Code != http.StatusFound || !strings.Contains(browser.Header().Get("Location"), "via=") {
		t.Fatalf("browser click must stay a 302 with via=, got %d %q", browser.Code, browser.Header().Get("Location"))
	}
	if n := clickCount(t, db); n != 1 {
		t.Fatalf("expected one browser click, got %d", n)
	}

	previews := []string{
		"Twitterbot/1.0",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"TelegramBot (like TwitterBot)",
		"WhatsApp/2.23.20.0",
		"Slack-ImgProxy (+https://api.slack.com/robots)",
		"Pinterest/0.2 (+http://www.pinterest.com/)",
		"redditbot/1.0",
		"Embedly/0.2",
		"Quora Link Preview/1.0",
		"Iframely/1.3.1",
		"bingpreview/1.0b",
		"SkypeUriPreview/1.0",
	}
	for _, ua := range previews {
		rec := followGo(r, http.MethodGet, code, ua)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: expected 302, got %d", ua, rec.Code)
		}
		if strings.Contains(rec.Header().Get("Location"), "via=") {
			t.Fatalf("%s: preview redirect must not carry attribution, got %q", ua, rec.Header().Get("Location"))
		}
		if !strings.Contains(rec.Header().Get("Location"), "inboxpilot.example.com") {
			t.Fatalf("%s: expected the offer landing URL, got %q", ua, rec.Header().Get("Location"))
		}
	}
	head := followGo(r, http.MethodHead, code, "Mozilla/5.0")
	if head.Code != http.StatusFound {
		t.Fatalf("HEAD: expected 302, got %d %s", head.Code, head.Body.String())
	}
	if strings.Contains(head.Header().Get("Location"), "via=") {
		t.Fatalf("HEAD must not carry attribution, got %q", head.Header().Get("Location"))
	}
	if n := clickCount(t, db); n != 1 {
		t.Fatalf("previews and HEAD must not insert clicks, got %d", n)
	}
}

func followGo(r http.Handler, method, code, userAgent string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/v1/go/"+code, nil)
	req.Header.Set("User-Agent", userAgent)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func clickCount(t *testing.T, db *database.DB) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
