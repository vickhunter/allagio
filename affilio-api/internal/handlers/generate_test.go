package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateContentFallbackTemplate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	articleID, err := db.UpsertArticle(context.Background(), siteID, 1, "Test", "https://example.com/post", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Semrush Affiliate Program", "Semrush", "semrush-test")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, "Great fit for SEO workflows.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)

	handler := &GenerateHandler{DB: db, Generator: nil}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/generate/content", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.GenerateContent(c)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate/content", newJSONRequest(t, http.MethodPost, "/generate/content", map[string]interface{}{
		"recommendation_id": recID,
		"insertion_type":    "inline_mention",
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Semrush") {
		t.Fatalf("expected program name in response: %s", body)
	}
	if !strings.Contains(body, "Great fit for SEO workflows") {
		t.Fatalf("expected reason in response: %s", body)
	}
}
