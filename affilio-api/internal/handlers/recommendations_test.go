package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func TestRecommendationsListAndPatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, err := db.CreateSite(context.Background(), "test-key", "example.com", "Example Site")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	articleID, err := db.UpsertArticle(context.Background(), siteID, 1, "Test", "https://example.com/post", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Semrush Affiliate Program", "Semrush", "semrush-recs")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, "Great fit for SEO workflows.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)

	handler := &RecommendationsHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/recommendations", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.List(c)
	})
	r.PATCH("/recommendations/:id", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Patch(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var listPayload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	recs, ok := listPayload["recommendations"].([]interface{})
	if !ok || len(recs) == 0 {
		t.Fatalf("expected recommendations in response: %s", rec.Body.String())
	}
	firstRec, ok := recs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected recommendation object payload")
	}
	expectedImpact, _ := firstRec["expected_impact"].(string)
	if strings.TrimSpace(expectedImpact) == "" {
		t.Fatalf("expected expected_impact field in recommendation payload")
	}
	placementWhy, _ := firstRec["placement_why"].(string)
	if strings.TrimSpace(placementWhy) == "" {
		t.Fatalf("expected placement_why field in recommendation payload")
	}

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/recommendations/"+recID, newJSONRequest(t, http.MethodPatch, "/recommendations/"+recID, map[string]interface{}{
		"status": "dismissed",
	}))
	r.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", patchRec.Code)
	}
	assertRecommendationStatus(t, db, recID, "dismissed")

	invalidRec := httptest.NewRecorder()
	invalidReq := httptest.NewRequest(http.MethodPatch, "/recommendations/"+recID, newJSONRequest(t, http.MethodPatch, "/recommendations/"+recID, map[string]interface{}{
		"status": "inserted",
	}))
	r.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid patch status, got %d", invalidRec.Code)
	}

	conflictRec := httptest.NewRecorder()
	conflictReq := httptest.NewRequest(http.MethodPatch, "/recommendations/"+recID, newJSONRequest(t, http.MethodPatch, "/recommendations/"+recID, map[string]interface{}{
		"status": "approved",
	}))
	r.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for invalid transition, got %d", conflictRec.Code)
	}
}

func assertRecommendationStatus(t *testing.T, db *database.DB, recID, status string) {
	t.Helper()
	var got string
	err := db.Pool.QueryRow(context.Background(), `SELECT status FROM recommendations WHERE id=$1`, recID).Scan(&got)
	if err != nil {
		t.Fatalf("lookup recommendation status: %v", err)
	}
	if got != status {
		t.Fatalf("expected status %s, got %s", status, got)
	}
}

func TestRecommendationStatusDBGuardrail(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, err := db.CreateSite(context.Background(), "test-key", "example.com", "Example Site")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	articleID, err := db.UpsertArticle(context.Background(), siteID, 22, "Guardrail", "https://example.com/guardrail", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Guardrail Program", "Example", "guardrail-program")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, "Good fit.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)
	_, err = db.Pool.Exec(context.Background(), `UPDATE recommendations SET status='inserted' WHERE id=$1`, recID)
	if err == nil {
		t.Fatalf("expected DB guardrail error for pending->inserted transition")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "invalid recommendation status transition") {
		t.Fatalf("unexpected DB error: %v", err)
	}
}
