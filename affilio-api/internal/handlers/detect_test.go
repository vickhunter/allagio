package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

func TestDetectLinksReturnsDetections(t *testing.T) {
	t.Parallel()
	handler := &DetectHandler{Classifier: &services.AIClassifier{}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/detect/links", handler.DetectLinks)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/detect/links", newJSONRequest(t, http.MethodPost, "/detect/links", map[string]interface{}{
		"content": "Try https://example.com/?ref=123 for details.",
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	detections, ok := payload["detections"].([]interface{})
	if !ok || len(detections) == 0 {
		t.Fatalf("expected detections in response: %s", rec.Body.String())
	}
}
