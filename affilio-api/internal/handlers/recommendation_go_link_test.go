package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

func TestGenerateContentPublishesTrackedGoLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}
	rec := postGenerate(t, db, siteID, scriptedGenerator(t, []string{"Una frase utile per questo articolo."}), map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "it_IT",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("generate status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode generate: %v", err)
	}
	html, _ := body["html"].(string)
	tracked := "/v1/go/" + recID
	if !strings.Contains(html, `href="`+tracked+`"`) {
		t.Fatalf("reader html did not use the tracked go link: %s", html)
	}
	if !strings.Contains(html, `rel="nofollow sponsored"`) {
		t.Fatalf("reader html dropped rel=nofollow sponsored: %s", html)
	}
	if strings.Contains(html, "example.com/tool") || strings.Contains(html, "ref=staging") {
		t.Fatalf("reader html pointed at the referral url: %s", html)
	}
	lower := strings.ToLower(html)
	// The relevance score is 80. Strip the tracked go link first: its UUID can
	// contain those digits, and that is not the score.
	outsideLink := strings.ReplaceAll(html, tracked, "")
	if strings.Contains(lower, "strong contextual") || strings.Contains(lower, "relevance") || strings.Contains(outsideLink, "80") {
		t.Fatalf("reader html included a reason or score: %s", html)
	}
	if body["tracked_link"] != tracked {
		t.Fatalf("tracked_link = %#v, want %s", body["tracked_link"], tracked)
	}
}

func TestRecommendationGoLinkRecordsClick(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/go/:code", (&RedirectsHandler{DB: db, IPSaltSecret: "rec-go-salt"}).Go)

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+recID, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15")
	req.RemoteAddr = "203.0.113.8:443"
	router.ServeHTTP(first, req)
	if first.Code != http.StatusFound {
		t.Fatalf("go status %d body %s", first.Code, first.Body.String())
	}
	if got := first.Header().Get("Location"); got != referral {
		t.Fatalf("location = %q, want the stored referral url %q", got, referral)
	}
	var clicks int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE recommendation_id=$1`, recID).Scan(&clicks); err != nil {
		t.Fatalf("count clicks: %v", err)
	}
	if clicks != 1 {
		t.Fatalf("recommendation clicks = %d, want 1", clicks)
	}

	second := httptest.NewRecorder()
	again := httptest.NewRequest(http.MethodGet, "/v1/go/"+recID, nil)
	again.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15")
	again.RemoteAddr = "203.0.113.8:443"
	router.ServeHTTP(second, again)
	if second.Code != http.StatusFound || second.Header().Get("Location") != referral {
		t.Fatalf("deduped go = %d %s", second.Code, second.Header().Get("Location"))
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE recommendation_id=$1`, recID).Scan(&clicks); err != nil {
		t.Fatalf("recount clicks: %v", err)
	}
	if clicks != 1 {
		t.Fatalf("deduped recommendation clicks = %d, want 1", clicks)
	}

	unknown := httptest.NewRecorder()
	miss := httptest.NewRequest(http.MethodGet, "/v1/go/ffffffff-ffff-ffff-ffff-ffffffffffff", nil)
	router.ServeHTTP(unknown, miss)
	if unknown.Code != http.StatusNotFound {
		body, _ := io.ReadAll(unknown.Body)
		t.Fatalf("unknown go status %d body %s", unknown.Code, body)
	}
}

func TestRecommendationPluginProxyRecordsReaderClick(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}
	router := recommendationGoRouter(t, db)
	before := recommendationTotalClicks(t, db, siteID)
	readerHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	headers := map[string]string{
		"X-API-Key":        "test-key",
		"X-OR-Client-Hash": readerHash,
		"X-OR-User-Agent":  testBrowserUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	first := recommendationPluginHit(router, recID, "192.0.2.10:443", headers)
	if first.Code != http.StatusFound || first.Header().Get("Location") != referral {
		t.Fatalf("plugin go = %d %s", first.Code, first.Header().Get("Location"))
	}
	if got := recommendationTotalClicks(t, db, siteID); got != before+1 {
		t.Fatalf("total_clicks = %d, want %d", got, before+1)
	}
	var ua, referrer, ipHash, class string
	if err := db.Pool.QueryRow(context.Background(), `SELECT user_agent, referrer, ip_hash, click_class FROM clicks WHERE recommendation_id=$1`, recID).Scan(&ua, &referrer, &ipHash, &class); err != nil {
		t.Fatalf("read click: %v", err)
	}
	if ua != testBrowserUA || referrer != "https://reader.example/article" || ipHash != readerHash || class != ClickClassBrowser {
		t.Fatalf("plugin click stored server context: ua=%q ref=%q hash=%q class=%q", ua, referrer, ipHash, class)
	}
	recommendationPluginHit(router, recID, "203.0.113.20:443", headers)
	if got := recommendationTotalClicks(t, db, siteID); got != before+1 {
		t.Fatalf("same reader hash was counted again from another server: total_clicks=%d", got)
	}
}

func TestRecommendationPluginProxyIgnoresBadKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}
	router := recommendationGoRouter(t, db)
	before := recommendationTotalClicks(t, db, siteID)
	headers := map[string]string{
		"X-API-Key":        "wrong-site-key",
		"X-OR-Client-Hash": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		"X-OR-User-Agent":  testBrowserUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	hit := recommendationPluginHit(router, recID, "192.0.2.40:443", headers)
	if hit.Code != http.StatusFound || hit.Header().Get("Location") != referral {
		t.Fatalf("bad key go = %d %s", hit.Code, hit.Header().Get("Location"))
	}
	if got := recommendationTotalClicks(t, db, siteID); got != before {
		t.Fatalf("bad key changed total_clicks from %d to %d", before, got)
	}
}

func TestRecommendationPluginProxyIgnoresMissingKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}
	router := recommendationGoRouter(t, db)
	before := recommendationTotalClicks(t, db, siteID)
	headers := map[string]string{
		"X-OR-Client-Hash": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		"X-OR-User-Agent":  testBrowserUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	hit := recommendationPluginHit(router, recID, "192.0.2.41:443", headers)
	if hit.Code != http.StatusFound || hit.Header().Get("Location") != referral {
		t.Fatalf("missing key go = %d %s", hit.Code, hit.Header().Get("Location"))
	}
	if got := recommendationTotalClicks(t, db, siteID); got != before {
		t.Fatalf("missing key changed total_clicks from %d to %d", before, got)
	}
}

func TestRecommendationPluginProxyIgnoresOtherSiteKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, "Strong contextual fit for the article.")
	otherKey := "other-site-plugin-key"
	createTestSite(t, db, otherKey, "other-site.example", "Other Site")
	referral := "https://example.com/tool?ref=staging"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET referral_url=$1 WHERE slug='semrush-reader-copy'`, referral); err != nil {
		t.Fatalf("set referral url: %v", err)
	}
	router := recommendationGoRouter(t, db)
	before := recommendationTotalClicks(t, db, siteID)
	headers := map[string]string{
		"X-API-Key":        otherKey,
		"X-OR-Client-Hash": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"X-OR-User-Agent":  testBrowserUA,
		"X-OR-Referer":     "https://reader.example/article",
	}
	hit := recommendationPluginHit(router, recID, "192.0.2.42:443", headers)
	if hit.Code != http.StatusFound || hit.Header().Get("Location") != referral {
		t.Fatalf("other site key go = %d %s", hit.Code, hit.Header().Get("Location"))
	}
	if got := recommendationTotalClicks(t, db, siteID); got != before {
		t.Fatalf("other site key changed total_clicks from %d to %d", before, got)
	}
}

func recommendationGoRouter(t *testing.T, db *database.DB) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/go/:code", (&RedirectsHandler{DB: db, IPSaltSecret: "rec-go-salt"}).Go)
	return router
}

func recommendationPluginHit(router http.Handler, recID, remote string, headers map[string]string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+recID+"?via=plugin", nil)
	req.RemoteAddr = remote
	req.Header.Set("User-Agent", "WordPress/6.7.1; https://blog.example")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func recommendationTotalClicks(t *testing.T, db *database.DB, siteID string) int {
	t.Helper()
	summary, err := db.GetClickSummary(context.Background(), siteID, 0)
	if err != nil {
		t.Fatalf("click summary: %v", err)
	}
	return summary.TotalClicks
}
