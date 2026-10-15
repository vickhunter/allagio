package handlers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"

	"github.com/jackc/pgx/v5"
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

// TestMigration038PsqlFileLeavesNaiveColumnsUnderRome runs the file the way
// psql -f does: each statement autocommits, and a failed statement does not
// stop the next one. The guard has to share a transaction with the ALTERs,
// or the columns still change.
func TestMigration038PsqlFileLeavesNaiveColumnsUnderRome(t *testing.T) {
	db, cleanup := setupTestDBThrough(t, "UTC", "037_reserved_sale_events.sql")
	defer cleanup()
	runMigrationFileLikePsql(t, "Europe/Rome", "038_timestamptz_instants.sql")
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp without time zone")
	assertColumnType(t, db, "conversions", "occurred_at", "timestamp without time zone")
	assertColumnType(t, db, "ledger_entries", "clears_at", "timestamp without time zone")
}

func TestMigration038DownPsqlFileLeavesTimestamptzUnderRome(t *testing.T) {
	db, cleanup := setupTestDBThrough(t, "UTC", "038_timestamptz_instants.sql")
	defer cleanup()
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp with time zone")
	runMigrationFileLikePsql(t, "Europe/Rome", filepath.Join("down", "038_timestamptz_instants.sql"))
	assertColumnType(t, db, "clicks", "clicked_at", "timestamp with time zone")
	assertColumnType(t, db, "conversions", "occurred_at", "timestamp with time zone")
	assertColumnType(t, db, "ledger_entries", "clears_at", "timestamp with time zone")
}

func TestCohortWindowUsesUTCArticleFaceUnderRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	siteID := createTestSite(t, db, "site-key-cohort-window", "cohort-window.example.com", "Cohort Window")
	// Thirty minutes inside a one-day UTC window. Europe/Rome is at least an
	// hour ahead, so the same face is outside the window when the session
	// casts the naive timestamp.
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO articles (site_id, wp_post_id, title, created_at) VALUES ($1, 7, 'boundary', (NOW() AT TIME ZONE 'UTC') - INTERVAL '1 day' + INTERVAL '30 minutes')`, siteID); err != nil {
		t.Fatal(err)
	}
	rows, err := db.ListCohortMetrics(context.Background(), siteID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Articles != 1 {
		t.Fatalf("cohort metrics %#v, want the article inside the UTC window", rows)
	}
}

func TestRetentionWindowUsesUTCArticleFaceUnderRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	siteID := createTestSite(t, db, "site-key-retention-window", "retention-window.example.com", "Retention Window")
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO articles (site_id, wp_post_id, title, created_at) VALUES ($1, 8, 'boundary', (NOW() AT TIME ZONE 'UTC') - INTERVAL '1 day' + INTERVAL '30 minutes')`, siteID); err != nil {
		t.Fatal(err)
	}
	rows, err := db.ListRetentionCohorts(context.Background(), siteID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Articles != 1 {
		t.Fatalf("retention %#v, want the article inside the UTC window", rows)
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

func runMigrationFileLikePsql(t *testing.T, zone, name string) {
	t.Helper()
	path := filepath.Join("..", "migrations", name)
	if _, err := exec.LookPath("psql"); err == nil {
		raw := strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
		// libpq rejects TimeZone as a URI parameter. PGOPTIONS sets it
		// before the script, which is the session psql -f uses.
		cmd := exec.Command("psql", keywordDSN(t, raw), "-f", path)
		cmd.Env = append(os.Environ(), "PGOPTIONS=-c TimeZone="+zone)
		// Plain psql -f, not ON_ERROR_STOP and not -1. Each statement
		// autocommits, and psql keeps going after an error.
		out, err := cmd.CombinedOutput()
		t.Logf("psql %s: %s", name, out)
		if err != nil {
			t.Fatalf("psql did not run %s: %v\n%s", name, err, out)
		}
		return
	}
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(context.Background(), mustSessionDSN(t, zone))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range splitSQLStatements(string(script)) {
		if _, err := db.Pool.Exec(context.Background(), stmt); err != nil {
			t.Logf("statement error: %v", err)
		}
	}
}

func mustSessionDSN(t *testing.T, zone string) string {
	t.Helper()
	dsn, err := withSessionTimeZone(strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL")), zone)
	if err != nil {
		t.Fatal(err)
	}
	return dsn
}

// splitSQLStatements splits a script on semicolons outside quotes and
// dollar-quotes, the same cuts psql -f makes.
func splitSQLStatements(script string) []string {
	var stmts []string
	var b strings.Builder
	dollar := ""
	inSingle := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if inLineComment {
			b.WriteByte(c)
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			b.WriteByte(c)
			if c == '*' && i+1 < len(script) && script[i+1] == '/' {
				b.WriteByte('/')
				i++
				inBlockComment = false
			}
			continue
		}
		if dollar != "" {
			if strings.HasPrefix(script[i:], dollar) {
				b.WriteString(dollar)
				i += len(dollar) - 1
				dollar = ""
				continue
			}
			b.WriteByte(c)
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if c == '-' && i+1 < len(script) && script[i+1] == '-' {
			b.WriteString("--")
			i++
			inLineComment = true
			continue
		}
		if c == '/' && i+1 < len(script) && script[i+1] == '*' {
			b.WriteString("/*")
			i++
			inBlockComment = true
			continue
		}
		if c == '\'' {
			b.WriteByte(c)
			inSingle = true
			continue
		}
		if c == '$' {
			tag, ok := dollarTagAt(script, i)
			if ok {
				dollar = tag
				b.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}
		if c == ';' {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

func dollarTagAt(script string, i int) (string, bool) {
	if i >= len(script) || script[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(script) {
		c := script[j]
		if c == '$' {
			return script[i : j+1], true
		}
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return "", false
		}
		j++
	}
	return "", false
}

func scanInstantOn(t *testing.T, conn interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, table, column, id string) time.Time {
	t.Helper()
	var got time.Time
	query := "SELECT " + column + " FROM " + table + " WHERE id=$1"
	if err := conn.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func romeWall(t *testing.T, conn interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, table, column, id string) string {
	t.Helper()
	var got string
	query := "SELECT to_char(" + column + " AT TIME ZONE 'Europe/Rome', 'YYYY-MM-DD HH24:MI:SS') FROM " + table + " WHERE id=$1"
	if err := conn.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
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
