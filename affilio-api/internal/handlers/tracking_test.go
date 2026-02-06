package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrackingSummaryAndTrends(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID, err := db.CreateSite(context.Background(), "test-key", "example.com", "Example Site")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	articleID, err := db.UpsertArticle(context.Background(), siteID, 11, "Analytics Post", "https://example.com/a", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Analytics Program", "Example", "analytics-program")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, "Good fit.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	recID := lookupRecommendationID(t, db, articleID)
	programID2 := insertProgram(t, db, "Analytics Program 2", "Example", "analytics-program-2")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID2, nil, 75, "Also good fit.", "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation 2: %v", err)
	}
	var recID2 string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM recommendations WHERE article_id=$1 AND id<>$2 ORDER BY created_at DESC LIMIT 1`, articleID, recID).Scan(&recID2); err != nil {
		t.Fatalf("lookup second recommendation: %v", err)
	}
	_, err = db.Pool.Exec(context.Background(), `
INSERT INTO clicks (site_id, article_id, recommendation_id, program_id, user_agent, referrer, ip_hash)
VALUES ($1,$2,$3,$4,'test-agent','https://example.com','abc123')`, siteID, articleID, recID, programID)
	if err != nil {
		t.Fatalf("insert click: %v", err)
	}
	_, err = db.Pool.Exec(context.Background(), `
INSERT INTO clicks (site_id, article_id, recommendation_id, program_id, user_agent, referrer, ip_hash)
VALUES ($1,$2,$3,$4,'test-agent','https://example.com','abc123')`, siteID, articleID, recID2, programID2)
	if err != nil {
		t.Fatalf("insert click 2: %v", err)
	}

	handler := &TrackingHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/track/summary", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Summary(c)
	})
	r.GET("/track/trends", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Trends(c)
	})
	r.GET("/track/attribution", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Attribution(c)
	})
	r.GET("/track/funnel", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Funnel(c)
	})
	r.GET("/track/cohorts", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Cohorts(c)
	})
	r.GET("/track/export", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Export(c)
	})
	r.GET("/track/recommendation-quality", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.RecommendationQuality(c)
	})
	r.GET("/track/retention", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Retention(c)
	})
	r.GET("/track/recommendation-tuning", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.RecommendationTuning(c)
	})

	summaryRec := httptest.NewRecorder()
	summaryReq := httptest.NewRequest(http.MethodGet, "/track/summary?days=7", nil)
	r.ServeHTTP(summaryRec, summaryReq)
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", summaryRec.Code)
	}
	var summaryPayload map[string]interface{}
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summaryPayload); err != nil {
		t.Fatalf("invalid summary response: %v", err)
	}
	summary, ok := summaryPayload["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing summary payload: %s", summaryRec.Body.String())
	}
	if int(summary["total_clicks"].(float64)) < 1 {
		t.Fatalf("expected total clicks >= 1, got %v", summary["total_clicks"])
	}

	trendsRec := httptest.NewRecorder()
	trendsReq := httptest.NewRequest(http.MethodGet, "/track/trends?days=7", nil)
	r.ServeHTTP(trendsRec, trendsReq)
	if trendsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", trendsRec.Code)
	}
	var trendsPayload map[string]interface{}
	if err := json.Unmarshal(trendsRec.Body.Bytes(), &trendsPayload); err != nil {
		t.Fatalf("invalid trends response: %v", err)
	}
	trends, ok := trendsPayload["trends"].([]interface{})
	if !ok || len(trends) == 0 {
		t.Fatalf("expected trend points in response: %s", trendsRec.Body.String())
	}

	attributionRec := httptest.NewRecorder()
	attributionReq := httptest.NewRequest(http.MethodGet, "/track/attribution?days=7&model=linear_session", nil)
	r.ServeHTTP(attributionRec, attributionReq)
	if attributionRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", attributionRec.Code)
	}
	var attributionPayload map[string]interface{}
	if err := json.Unmarshal(attributionRec.Body.Bytes(), &attributionPayload); err != nil {
		t.Fatalf("invalid attribution response: %v", err)
	}
	programs, ok := attributionPayload["programs"].([]interface{})
	if !ok || len(programs) == 0 {
		t.Fatalf("expected attribution programs in response: %s", attributionRec.Body.String())
	}
	meta, ok := attributionPayload["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected attribution meta in response: %s", attributionRec.Body.String())
	}
	if meta["model_id"] != "linear_session" {
		t.Fatalf("expected linear_session model id, got: %v", meta["model_id"])
	}
	if meta["estimated_total_revenue_usd"] == nil {
		t.Fatalf("expected estimated_total_revenue_usd in response: %s", attributionRec.Body.String())
	}
	firstProgram, ok := programs[0].(map[string]interface{})
	if !ok || firstProgram["attributed_clicks"] == nil {
		t.Fatalf("expected attributed_clicks in attribution response: %s", attributionRec.Body.String())
	}

	funnelRec := httptest.NewRecorder()
	funnelReq := httptest.NewRequest(http.MethodGet, "/track/funnel?days=7", nil)
	r.ServeHTTP(funnelRec, funnelReq)
	if funnelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", funnelRec.Code)
	}
	var funnelPayload map[string]interface{}
	if err := json.Unmarshal(funnelRec.Body.Bytes(), &funnelPayload); err != nil {
		t.Fatalf("invalid funnel response: %v", err)
	}
	funnel, ok := funnelPayload["funnel"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected funnel payload in response: %s", funnelRec.Body.String())
	}
	if funnel["clicks"] == nil || funnel["inserted"] == nil || funnel["dismissed"] == nil {
		t.Fatalf("expected funnel fields in response: %s", funnelRec.Body.String())
	}

	cohortRec := httptest.NewRecorder()
	cohortReq := httptest.NewRequest(http.MethodGet, "/track/cohorts?days=90", nil)
	r.ServeHTTP(cohortRec, cohortReq)
	if cohortRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", cohortRec.Code)
	}
	var cohortPayload map[string]interface{}
	if err := json.Unmarshal(cohortRec.Body.Bytes(), &cohortPayload); err != nil {
		t.Fatalf("invalid cohort response: %v", err)
	}
	cohorts, ok := cohortPayload["cohorts"].([]interface{})
	if !ok || len(cohorts) == 0 {
		t.Fatalf("expected cohorts in response: %s", cohortRec.Body.String())
	}

	qualityRec := httptest.NewRecorder()
	qualityReq := httptest.NewRequest(http.MethodGet, "/track/recommendation-quality?days=90", nil)
	r.ServeHTTP(qualityRec, qualityReq)
	if qualityRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", qualityRec.Code)
	}
	var qualityPayload map[string]interface{}
	if err := json.Unmarshal(qualityRec.Body.Bytes(), &qualityPayload); err != nil {
		t.Fatalf("invalid quality response: %v", err)
	}
	if qualityPayload["summary"] == nil {
		t.Fatalf("expected quality summary in response: %s", qualityRec.Body.String())
	}
	if qualityPrograms, ok := qualityPayload["programs"].([]interface{}); !ok || len(qualityPrograms) == 0 {
		t.Fatalf("expected quality programs in response: %s", qualityRec.Body.String())
	}

	retentionRec := httptest.NewRecorder()
	retentionReq := httptest.NewRequest(http.MethodGet, "/track/retention?days=90", nil)
	r.ServeHTTP(retentionRec, retentionReq)
	if retentionRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", retentionRec.Code)
	}
	var retentionPayload map[string]interface{}
	if err := json.Unmarshal(retentionRec.Body.Bytes(), &retentionPayload); err != nil {
		t.Fatalf("invalid retention response: %v", err)
	}
	if _, ok := retentionPayload["retention"].([]interface{}); !ok {
		t.Fatalf("expected retention payload in response: %s", retentionRec.Body.String())
	}

	tuningRec := httptest.NewRecorder()
	tuningReq := httptest.NewRequest(http.MethodGet, "/track/recommendation-tuning?days=90", nil)
	r.ServeHTTP(tuningRec, tuningReq)
	if tuningRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", tuningRec.Code)
	}
	var tuningPayload map[string]interface{}
	if err := json.Unmarshal(tuningRec.Body.Bytes(), &tuningPayload); err != nil {
		t.Fatalf("invalid tuning response: %v", err)
	}
	if _, ok := tuningPayload["tuning"].([]interface{}); !ok {
		t.Fatalf("expected tuning payload in response: %s", tuningRec.Body.String())
	}

	exportJSONRec := httptest.NewRecorder()
	exportJSONReq := httptest.NewRequest(http.MethodGet, "/track/export?days=7&format=json", nil)
	r.ServeHTTP(exportJSONRec, exportJSONReq)
	if exportJSONRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", exportJSONRec.Code)
	}
	if contentType := exportJSONRec.Header().Get("Content-Type"); contentType == "" {
		t.Fatalf("expected content type for json export")
	}
	var exportJSON map[string]interface{}
	if err := json.Unmarshal(exportJSONRec.Body.Bytes(), &exportJSON); err != nil {
		t.Fatalf("invalid export json response: %v", err)
	}
	if exportJSON["summary"] == nil || exportJSON["funnel"] == nil || exportJSON["attribution"] == nil {
		t.Fatalf("expected export payload sections: %s", exportJSONRec.Body.String())
	}

	exportCSVRec := httptest.NewRecorder()
	exportCSVReq := httptest.NewRequest(http.MethodGet, "/track/export?days=7&format=csv", nil)
	r.ServeHTTP(exportCSVRec, exportCSVReq)
	if exportCSVRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", exportCSVRec.Code)
	}
	if contentType := exportCSVRec.Header().Get("Content-Type"); contentType == "" {
		t.Fatalf("expected content type for csv export")
	}
	if body := exportCSVRec.Body.String(); body == "" || !strings.HasPrefix(body, "section") {
		t.Fatalf("expected csv body, got: %s", body)
	}
}
