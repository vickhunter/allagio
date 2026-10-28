package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDirectoryMakersEmptyWhenDatabaseAnswers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{DB: db}).Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 0 || body["next_cursor"] != nil || body["total"] != float64(0) {
		t.Fatalf("empty list %#v", body)
	}
	if _, listed := body["error"]; listed {
		t.Fatalf("healthy read named an error %#v", body)
	}
}

func TestDirectoryMakersClosedPoolIsUnavailable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{DB: db}).Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != `{"error":"directory unavailable"}` {
		t.Fatalf("closed pool %d %s", rec.Code, rec.Body.String())
	}
}
