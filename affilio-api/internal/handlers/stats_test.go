package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicStatsServesCounters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	entry := &AgentEntry{DB: db}
	r := gin.New()
	r.GET("/v1/stats", entry.PublicStats)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/stats", nil))
	if w.Code != 200 {
		t.Fatalf("stats: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Deals    int64 `json:"deals"`
		Offers   int64 `json:"offers"`
		Products int64 `json:"products"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("stats body: %v", err)
	}
}
