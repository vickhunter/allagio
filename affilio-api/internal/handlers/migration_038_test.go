package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
)

func TestMigration038ConvertsStoredUTCFaces(t *testing.T) {
	db, cleanup := setupTestDBThrough(t, "UTC", "037_reserved_sale_events.sql")
	defer cleanup()
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp without time zone")
	assertColumnType(t, db, "conversions", "occurred_at", "timestamp without time zone")
	assertColumnType(t, db, "ledger_entries", "clears_at", "timestamp without time zone")

	before := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)
	after := time.Date(2026, 10, 25, 1, 30, 0, 0, time.UTC)
	rows := []struct {
		table, column, beforeID, afterID string
	}{
		{
			table: "clicks", column: "clicked_at",
			beforeID: insertReturningID(t, db, `INSERT INTO clicks (clicked_at) VALUES ('2026-10-25 00:30:00') RETURNING id`),
			afterID:  insertReturningID(t, db, `INSERT INTO clicks (clicked_at) VALUES ('2026-10-25 01:30:00') RETURNING id`),
		},
		{
			table: "conversions", column: "occurred_at",
			beforeID: insertReturningID(t, db, `INSERT INTO conversions (provider, provider_event_id, kind, amount, occurred_at) VALUES ('stripe', 'face-before', 'sale', 10, '2026-10-25 00:30:00') RETURNING id`),
			afterID:  insertReturningID(t, db, `INSERT INTO conversions (provider, provider_event_id, kind, amount, occurred_at) VALUES ('stripe', 'face-after', 'sale', 10, '2026-10-25 01:30:00') RETURNING id`),
		},
		{
			table: "ledger_entries", column: "clears_at",
			beforeID: insertReturningID(t, db, `INSERT INTO ledger_entries (kind, amount, clears_at) VALUES ('commission', 1, '2026-10-25 00:30:00') RETURNING id`),
			afterID:  insertReturningID(t, db, `INSERT INTO ledger_entries (kind, amount, clears_at) VALUES ('commission', 1, '2026-10-25 01:30:00') RETURNING id`),
		},
	}
	for _, row := range rows {
		assertClockFace(t, db, row.table, row.column, row.beforeID, "2026-10-25 00:30:00")
		assertClockFace(t, db, row.table, row.column, row.afterID, "2026-10-25 01:30:00")
	}

	if _, err := db.Pool.Exec(context.Background(), readMigration(t, "038_timestamptz_instants.sql")); err != nil {
		t.Fatalf("apply 038: %v", err)
	}
	for _, row := range rows {
		assertColumnType(t, db, row.table, row.column, "timestamp with time zone")
		gotBefore := scanInstant(t, db, row.table, row.column, row.beforeID)
		gotAfter := scanInstant(t, db, row.table, row.column, row.afterID)
		if !gotBefore.Equal(before) || !gotAfter.Equal(after) {
			t.Fatalf("%s.%s instants %s and %s, want %s and %s", row.table, row.column, gotBefore.Format(time.RFC3339Nano), gotAfter.Format(time.RFC3339Nano), before.Format(time.RFC3339Nano), after.Format(time.RFC3339Nano))
		}
		if gotBefore.Equal(gotAfter) || gotAfter.Sub(gotBefore) != time.Hour {
			t.Fatalf("%s.%s collapsed the fold: %s and %s", row.table, row.column, gotBefore.Format(time.RFC3339Nano), gotAfter.Format(time.RFC3339Nano))
		}
	}

	if _, err := db.Pool.Exec(context.Background(), readMigration(t, filepath.Join("down", "038_timestamptz_instants.sql"))); err != nil {
		t.Fatalf("down 038: %v", err)
	}
	for _, row := range rows {
		assertColumnType(t, db, row.table, row.column, "timestamp without time zone")
		assertClockFace(t, db, row.table, row.column, row.beforeID, "2026-10-25 00:30:00")
		assertClockFace(t, db, row.table, row.column, row.afterID, "2026-10-25 01:30:00")
	}

	if _, err := db.Pool.Exec(context.Background(), readMigration(t, "038_timestamptz_instants.sql")); err != nil {
		t.Fatalf("re-apply 038: %v", err)
	}
	for _, row := range rows {
		assertColumnType(t, db, row.table, row.column, "timestamp with time zone")
		if got := scanInstant(t, db, row.table, row.column, row.beforeID); !got.Equal(before) {
			t.Fatalf("%s.%s after second up: %s", row.table, row.column, got.Format(time.RFC3339Nano))
		}
		if got := scanInstant(t, db, row.table, row.column, row.afterID); !got.Equal(after) {
			t.Fatalf("%s.%s after second up: %s", row.table, row.column, got.Format(time.RFC3339Nano))
		}
	}
}

func TestMigration038RejectsRomeSession(t *testing.T) {
	db, cleanup := setupTestDBThrough(t, "Europe/Rome", "037_reserved_sale_events.sql")
	defer cleanup()
	var zone string
	if err := db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone != "Europe/Rome" {
		t.Fatalf("session timezone %s", zone)
	}
	_, upErr := db.Pool.Exec(context.Background(), readMigration(t, "038_timestamptz_instants.sql"))
	assertUTCSessionRequired(t, upErr, "038")
	_, downErr := db.Pool.Exec(context.Background(), readMigration(t, filepath.Join("down", "038_timestamptz_instants.sql")))
	assertUTCSessionRequired(t, downErr, "down 038")
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp without time zone")
}

func TestDailyClicksStayOnUTCDayUnderRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	siteID := createTestSite(t, db, "site-key-utc-day", "utc-day.example.com", "UTC Day")
	clickAt := time.Date(2026, 6, 15, 23, 30, 0, 0, time.UTC)
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO clicks (site_id, clicked_at) VALUES ($1, $2)`, siteID, clickAt); err != nil {
		t.Fatal(err)
	}
	rows, err := db.ListDailyClickCounts(context.Background(), siteID, 400)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Day != "2026-06-15" || rows[0].Clicks != 1 {
		t.Fatalf("daily clicks %#v, want 2026-06-15", rows)
	}
}

func TestRetentionCohortReadsArticleFaceAsUTCUnderRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	siteID := createTestSite(t, db, "site-key-cohort-utc", "cohort-utc.example.com", "Cohort UTC")
	var articleID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO articles (site_id, wp_post_id, title, created_at) VALUES ($1, 42, 'June', '2026-06-15 12:00:00') RETURNING id`, siteID).Scan(&articleID); err != nil {
		t.Fatal(err)
	}
	clickAt := time.Date(2026, 6, 15, 11, 30, 0, 0, time.UTC)
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO clicks (site_id, article_id, clicked_at) VALUES ($1, $2, $3)`, siteID, articleID, clickAt); err != nil {
		t.Fatal(err)
	}
	rows, err := db.ListRetentionCohorts(context.Background(), siteID, 400)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Articles != 1 || rows[0].Week0Active != 0 {
		t.Fatalf("cohort %#v, want the 11:30 UTC click before the 12:00 UTC article face", rows)
	}
}

func insertReturningID(t *testing.T, db *database.DB, query string) string {
	t.Helper()
	var id string
	if err := db.Pool.QueryRow(context.Background(), query).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertClockFace(t *testing.T, db *database.DB, table, column, id, want string) {
	t.Helper()
	var got string
	query := "SELECT to_char(" + column + ", 'YYYY-MM-DD HH24:MI:SS') FROM " + table + " WHERE id=$1"
	if err := db.Pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s.%s face %s, want %s", table, column, got, want)
	}
}

func scanInstant(t *testing.T, db *database.DB, table, column, id string) time.Time {
	t.Helper()
	var got time.Time
	query := "SELECT " + column + " FROM " + table + " WHERE id=$1"
	if err := db.Pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertUTCSessionRequired(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s applied outside a UTC session", label)
	}
	msg := err.Error()
	if !strings.Contains(msg, "SHOW timezone") || !strings.Contains(msg, "UTC") {
		t.Fatalf("%s error %v", label, err)
	}
}
