package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	ctx := context.Background()
	db, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}
	applyMigrations(t, db)
	cleanup := func() {
		_ = truncateAll(db)
		db.Close()
	}
	return db, cleanup
}

func applyMigrations(t *testing.T, db *database.DB) {
	t.Helper()
	// Reset public schema so destructive migrations (e.g. column drops) can
	// be re-applied cleanly when multiple tests share the same database.
	if _, err := db.Pool.Exec(context.Background(), `
		DROP SCHEMA IF EXISTS public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO PUBLIC;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	root := filepath.Join("..", "migrations")
	files := []string{
		"001_initial.sql",
		"002_seed_programs.sql",
		"003_add_user_program_id.sql",
		"004_discovery_reviews.sql",
		"005_api_key_rotation.sql",
		"006_recommendation_status_guardrails.sql",
		"007_distributoor_core.sql",
		"008_attribution_payouts.sql",
		"009_scale_fraud_trust.sql",
		"010_audit_events.sql",
		"011_hash_api_keys.sql",
		"012_agent_revenue_poc.sql",
	}
	for _, f := range files {
		path := filepath.Join(root, f)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := db.Pool.Exec(context.Background(), string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", f, err)
		}
	}
}

func truncateAll(db *database.DB) error {
	_, err := db.Pool.Exec(context.Background(), `
TRUNCATE TABLE
    audit_events,
    payouts,
    ledger_entries,
    conversions,
    webhook_events,
    campaigns,
    offers,
    readiness_assessments,
    products,
    makers,
    site_key_rotations,
    program_discovery_reviews,
    program_discovery_queue,
    link_detections,
    user_program_links,
    user_programs,
    site_affiliate_accounts,
    clicks,
    recommendations,
    articles,
    sites,
    affiliate_programs
RESTART IDENTITY CASCADE`)
	return err
}

// createTestSite is a helper that creates a site with a hashed key for tests.
func createTestSite(t *testing.T, db *database.DB, plaintextKey, domain, name string) string {
	t.Helper()
	siteID, err := db.CreateSite(context.Background(), services.HashAPIKey(plaintextKey), services.KeyPrefix(plaintextKey), domain, name)
	if err != nil {
		t.Fatalf("create test site: %v", err)
	}
	return siteID
}

func newJSONRequest(t *testing.T, method, path string, payload interface{}) *bytes.Reader {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(body)
}

func setSiteID(c *gin.Context, siteID string) {
	c.Set("site_id", siteID)
}

func insertProgram(t *testing.T, db *database.DB, name, company, slug string) string {
	t.Helper()
	var id string
	err := db.Pool.QueryRow(context.Background(), `
INSERT INTO affiliate_programs (name, company, slug, description, website_url, commission_type, commission_value, commission_unit, cookie_duration_days, network, signup_url, primary_category, tags, keywords, program_quality_score, status, is_verified)
VALUES ($1,$2,$3,'Desc','https://example.com','one_time',10,'fixed',30,'direct','https://example.com/aff','marketing',ARRAY['marketing'],ARRAY['example'],70,'active',true)
RETURNING id`, name, company, slug).Scan(&id)
	if err != nil {
		t.Fatalf("insert program: %v", err)
	}
	return id
}

func lookupRecommendationID(t *testing.T, db *database.DB, articleID string) string {
	t.Helper()
	var id string
	err := db.Pool.QueryRow(context.Background(), `SELECT id FROM recommendations WHERE article_id=$1 LIMIT 1`, articleID).Scan(&id)
	if err != nil {
		t.Fatalf("lookup recommendation: %v", err)
	}
	return id
}
