package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

// sessionTimeZoneEnv sets the Postgres session TimeZone for setupTestDB.
// Europe/Rome reproduces timestamp-without-time-zone failures on a UTC server.
const sessionTimeZoneEnv = "AFFILIO_TEST_TIMEZONE"

func issuePluginKey(t *testing.T, db *database.DB, siteID, raw string) map[string]string {
	t.Helper()
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO plugin_keys(key_hash,key_prefix,site_id) VALUES($1,$2,$3)`, services.HashAPIKey(raw), services.KeyPrefix(raw), siteID); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"X-Plugin-Key": raw}
}

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	return setupTestDBInZone(t, os.Getenv(sessionTimeZoneEnv))
}

// setupTestDBInZone connects like setupTestDB and sets the session TimeZone.
// An empty zone leaves the server default. A named zone overrides it, including
// any TimeZone already present on AFFILIO_TEST_DATABASE_URL.
func setupTestDBInZone(t *testing.T, zone string) (*database.DB, func()) {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	dsn, err := withSessionTimeZone(raw, zone)
	if err != nil {
		t.Fatalf("session timezone: %v", err)
	}
	ctx := context.Background()
	db, err := database.Connect(ctx, dsn)
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

func withSessionTimeZone(rawURL, zone string) (string, error) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return rawURL, nil
	}
	if strings.Contains(rawURL, "://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("TimeZone", zone)
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	return setKeywordTimeZone(rawURL, zone), nil
}

// setKeywordTimeZone sets TimeZone on a host=... dbname=... connection string.
// Spaces inside single or double quotes stay inside that parameter.
func setKeywordTimeZone(raw, zone string) string {
	parts := splitKeywordDSN(raw)
	out := make([]string, 0, len(parts)+1)
	replaced := false
	for _, part := range parts {
		key, _, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(strings.Trim(key, `"'`), "timezone") {
			if !replaced {
				out = append(out, "TimeZone="+zone)
				replaced = true
			}
			continue
		}
		out = append(out, part)
	}
	if !replaced {
		out = append(out, "TimeZone="+zone)
	}
	return strings.Join(out, " ")
}

func splitKeywordDSN(raw string) []string {
	var parts []string
	var b strings.Builder
	var quote rune
	for _, r := range raw {
		switch {
		case quote != 0:
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			b.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if b.Len() > 0 {
				parts = append(parts, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

func applyMigrations(t *testing.T, db *database.DB) {
	t.Helper()
	applyMigrationsThrough(t, db, "049_site_key_grace_timestamptz.sql")
}

// setupTestDBThrough connects in zone and applies migrations through last,
// inclusive. last is a file name from migrationFiles, such as 037.
func setupTestDBThrough(t *testing.T, zone, last string) (*database.DB, func()) {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("AFFILIO_TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	dsn, err := withSessionTimeZone(raw, zone)
	if err != nil {
		t.Fatalf("session timezone: %v", err)
	}
	ctx := context.Background()
	db, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}
	applyMigrationsThrough(t, db, last)
	cleanup := func() {
		_ = truncateAll(db)
		db.Close()
	}
	return db, cleanup
}

func applyMigrationsThrough(t *testing.T, db *database.DB, last string) {
	t.Helper()
	ctx := context.Background()
	// 038 refuses a session that is not UTC. Apply every file on one
	// connection in UTC, then put that connection back on the test session.
	// Pool.Exec is not pinned to one connection, so the SET has to live here.
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration conn: %v", err)
	}
	defer conn.Release()
	var original string
	if err := conn.QueryRow(ctx, `SHOW timezone`).Scan(&original); err != nil {
		t.Fatalf("show timezone: %v", err)
	}
	if _, err := conn.Exec(ctx, `SET TIME ZONE 'UTC'`); err != nil {
		t.Fatalf("set utc for migrations: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT set_config('TimeZone', $1, false)`, original)
	}()
	// Reset public schema so destructive migrations (e.g. column drops) can
	// be re-applied cleanly when multiple tests share the same database.
	if _, err := conn.Exec(ctx, `
		DROP SCHEMA IF EXISTS public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO PUBLIC;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	root := filepath.Join("..", "migrations")
	applied := false
	for _, f := range migrationFiles() {
		path := filepath.Join(root, f)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := conn.Exec(ctx, string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", f, err)
		}
		if f == last {
			applied = true
			break
		}
	}
	if !applied {
		t.Fatalf("migration %s is not in the harness list", last)
	}
}

func migrationFiles() []string {
	return []string{
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
		"013_hosted_identity.sql",
		"014_verified_stripe_connections.sql",
		"015_site_owner_connection.sql",
		"016_placement_removed_at.sql",
		"017_owner_attribution_source.sql",
		"018_d032_agent_entry.sql",
		"019_d032_account_keys.sql",
		"020_d032_kill_switch.sql",
		"021_d032_interface.sql",
		"022_d032_restore.sql",
		"023_d032_plugin_restore.sql",
		"024_d032_plugin_placed.sql",
		"025_d032_plugin_failed.sql",
		"026_d032_failed_action.sql",
		"027_d032_restore_unconfirmed.sql",
		"028_d032_placement_live.sql",
		"029_d032_post_url.sql",
		"030_d032_ui_wiring.sql",
		"031_d032_committed_live_plugin_keys.sql",
		"032_d032_box_identity.sql",
		"033_click_classes.sql",
		"034_refund_trust.sql",
		"035_refund_seller_scope.sql",
		"036_refund_sale_link.sql",
		"037_reserved_sale_events.sql",
		"038_timestamptz_instants.sql",
		"039_landing_events.sql",
		"040_operator_approval.sql",
		"041_d063b_domain_verification.sql",
		"042_d062_step_modes.sql",
		"043_demo_test_clicks.sql",
		"044_d088_new_site_publish_ask.sql",
		"045_d093a_agent_join_tokens.sql",
		"046_d093a_open_signup_cap.sql",
		"047_d093b_placement_publish_ask.sql",
		"048_d097_monetization_suggestions.sql",
		"048_d092c_orp_test_deals.sql",
		"049_site_key_grace_timestamptz.sql",
	}
}

func truncateAll(db *database.DB) error {
	_, _ = db.Pool.Exec(context.Background(), `
DO $$ BEGIN
    IF to_regclass('public.orp_messages') IS NOT NULL THEN
        EXECUTE 'TRUNCATE TABLE orp_inbox, orp_deals, orp_offers, orp_messages RESTART IDENTITY CASCADE';
    END IF;
END $$`)
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
