package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestScanSingleCreatesRecommendations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	handler := &ScanHandler{DB: db, Classifier: nil}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/scan/single", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.ScanSingle(c)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scan/single", newJSONRequest(t, http.MethodPost, "/scan/single", map[string]interface{}{
		"wp_post_id": 1,
		"title":      "SEO Tools",
		"content":    "We compared Semrush for SEO research and ranking insights.",
		"url":        "https://example.com/seo-tools",
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	recs, ok := payload["recommendations"].([]interface{})
	if !ok || len(recs) == 0 {
		t.Fatalf("expected recommendations in response: %s", rec.Body.String())
	}
}
