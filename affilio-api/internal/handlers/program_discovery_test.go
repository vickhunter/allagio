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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProgramDiscoveryPromote(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`<html><head><title>Example Affiliate Program | Example</title><meta name="description" content="Partner with Example and earn recurring payouts."></head></html>`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}

	_ = createTestSite(t, db, "test-key", "example.com", "Example Site")
	var discoveryID string
	err = db.Pool.QueryRow(context.Background(), `
INSERT INTO program_discovery_queue (domain, sample_url, sites_count, status)
VALUES ('example.com',$1,1,'pending')
RETURNING id`, "https://example.com/affiliates").Scan(&discoveryID)
	if err != nil {
		t.Fatalf("insert discovery: %v", err)
	}

	handler := &ProgramDiscoveryHandler{DB: db, HTTPClient: client}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/program-discovery/queue/:id/promote", func(c *gin.Context) {
		handler.Promote(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/program-discovery/queue/"+discoveryID+"/promote", newJSONRequest(t, http.MethodPost, "/program-discovery/queue/"+discoveryID+"/promote", map[string]interface{}{}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if payload["status"] != "promoted" {
		t.Fatalf("expected promoted status, got %v", payload["status"])
	}

	var name, category string
	if err := db.Pool.QueryRow(context.Background(), `SELECT name, primary_category FROM affiliate_programs ORDER BY created_at DESC LIMIT 1`).Scan(&name, &category); err != nil {
		t.Fatalf("load promoted program: %v", err)
	}
	if name == "" {
		t.Fatalf("expected enriched name")
	}
	if category == "" {
		t.Fatalf("expected enriched category")
	}
}
