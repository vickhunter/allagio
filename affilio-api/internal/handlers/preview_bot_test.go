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

	siteID := createTestSite(t, db, "site-key-preview-bots", "previews.example.com", "Previews")
	_, offerID := seedListedOffer(t, db, "preview-maker-key", "preview-maker@example.com", "Preview Product")
	id := insertPlacement(t, db, siteID, offerID, "https://previews.example.com/the-post", "Preview Product is worth a look.")
	markPlacementLive(t, db, siteID, offerID, id, "preview-go", "https://previews.example.com/the-post")
	r := newAttributionRouter(db)

	browser := followGo(t, r, http.MethodGet, id, testBrowserUA)
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
		rec := followGo(t, r, http.MethodGet, id, ua)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: expected 302, got %d", ua, rec.Code)
		}
		if strings.Contains(rec.Header().Get("Location"), "via=") {
			t.Fatalf("%s: preview redirect must not carry attribution, got %q", ua, rec.Header().Get("Location"))
		}
		if !strings.Contains(rec.Header().Get("Location"), "example.com") {
			t.Fatalf("%s: expected the offer landing URL, got %q", ua, rec.Header().Get("Location"))
		}
	}
	head := followGo(t, r, http.MethodHead, id, "Mozilla/5.0")
	if head.Code != http.StatusFound {
		t.Fatalf("HEAD: expected 302, got %d %s", head.Code, head.Body.String())
	}
	if strings.Contains(head.Header().Get("Location"), "via=") {
		t.Fatalf("HEAD must not carry attribution, got %q", head.Header().Get("Location"))
	}
	want := 1 + len(previews) + 1
	if n := clickCount(t, db); n != want {
		t.Fatalf("previews and HEAD are stored as unknown clicks, got %d want %d", n, want)
	}
	var paid, unknown int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FILTER (WHERE click_class='browser'), COUNT(*) FILTER (WHERE click_class='unknown') FROM clicks`).Scan(&paid, &unknown); err != nil {
		t.Fatal(err)
	}
	if paid != 1 || unknown != len(previews)+1 {
		t.Fatalf("paid %d unknown %d", paid, unknown)
	}
}

func followGo(t *testing.T, r http.Handler, method, code, userAgent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/v1/go/"+code, nil)
	req.Header.Set("User-Agent", userAgent)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	requireGoLinkHeaders(t, rec)
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
