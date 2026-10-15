package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	if strings.Contains(lower, "strong contextual") || strings.Contains(lower, "relevance") || strings.Contains(html, "80") {
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
