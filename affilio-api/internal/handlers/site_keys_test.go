package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRotateSiteKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := createTestSite(t, db, "test-key-for-rotation-test", "example.com", "Example Site")

	handler := &SiteKeysHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sites/rotate-key", func(c *gin.Context) {
		handler.Rotate(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites/rotate-key", newJSONRequest(t, http.MethodPost, "/sites/rotate-key", map[string]interface{}{
		"site_id": siteID,
		"actor":   "tester",
		"note":    "rotation test",
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if payload["api_key"] == "" {
		t.Fatalf("expected api_key in response")
	}
}
