package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func TestInsertConfirmAndTrackClick(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	articleID, err := db.UpsertArticle(context.Background(), siteID, 10, "Test Article", "https://example.com/post", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Example Affiliate", "Example", "example-aff")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, "Good fit.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)
	if err := db.UpdateRecommendationStatus(context.Background(), siteID, recID, "approved"); err != nil {
		t.Fatalf("approve recommendation: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	generate := &GenerateHandler{DB: db}
	tracking := &TrackingHandler{DB: db}

	r.POST("/insert/confirm", func(c *gin.Context) {
		setSiteID(c, siteID)
		generate.ConfirmInsert(c)
	})
	r.POST("/track/click", func(c *gin.Context) {
		setSiteID(c, siteID)
		tracking.TrackClick(c)
	})

	confirmRec := httptest.NewRecorder()
	confirmReq := httptest.NewRequest(http.MethodPost, "/insert/confirm", newJSONRequest(t, http.MethodPost, "/insert/confirm", map[string]interface{}{
		"recommendation_id": recID,
	}))
	r.ServeHTTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", confirmRec.Code)
	}
	assertRecommendationInserted(t, db, recID)

	clickRec := httptest.NewRecorder()
	clickReq := httptest.NewRequest(http.MethodPost, "/track/click", newJSONRequest(t, http.MethodPost, "/track/click", map[string]interface{}{
		"article_id":        articleID,
		"recommendation_id": recID,
		"program_id":        programID,
	}))
	r.ServeHTTP(clickRec, clickReq)
	if clickRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", clickRec.Code)
	}
	assertClickRecorded(t, db, recID)
}

func TestInsertConfirmRequiresApprovedStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	articleID, err := db.UpsertArticle(context.Background(), siteID, 11, "Pending Insert Article", "https://example.com/pending", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Pending Affiliate", "Example", "pending-aff")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 75, "Good fit.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	generate := &GenerateHandler{DB: db}
	r.POST("/insert/confirm", func(c *gin.Context) {
		setSiteID(c, siteID)
		generate.ConfirmInsert(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/insert/confirm", newJSONRequest(t, http.MethodPost, "/insert/confirm", map[string]interface{}{
		"recommendation_id": recID,
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for pending recommendation confirm, got %d", rec.Code)
	}
}

func assertRecommendationInserted(t *testing.T, db *database.DB, recID string) {
	t.Helper()
	var status string
	var insertedAt *time.Time
	err := db.Pool.QueryRow(context.Background(), `SELECT status, inserted_at FROM recommendations WHERE id=$1`, recID).Scan(&status, &insertedAt)
	if err != nil {
		t.Fatalf("lookup recommendation: %v", err)
	}
	if status != "inserted" {
		t.Fatalf("expected status inserted, got %s", status)
	}
	if insertedAt == nil {
		t.Fatalf("expected inserted_at to be set")
	}
}

func assertClickRecorded(t *testing.T, db *database.DB, recID string) {
	t.Helper()
	var count int
	err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE recommendation_id=$1`, recID).Scan(&count)
	if err != nil {
		t.Fatalf("count clicks: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 click, got %d", count)
	}
}
