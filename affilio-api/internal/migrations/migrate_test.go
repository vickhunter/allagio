package migrations

import (
	"context"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration038IsOneTransaction(t *testing.T) {
	data, err := files.ReadFile("038_timestamptz_instants.sql")
	if err != nil {
		t.Fatal(err)
	}
	body, ok := scriptOwnsTransaction(string(data))
	if !ok {
		t.Fatal("038 is not one BEGIN/COMMIT transaction")
	}
	if !strings.Contains(body, "RAISE EXCEPTION") || !strings.Contains(body, "ALTER TABLE clicks") || !strings.Contains(body, "ALTER TABLE conversions") || !strings.Contains(body, "ALTER TABLE ledger_entries") {
		t.Fatalf("038 transaction body dropped work: %s", body)
	}
	if strings.Contains(string(data), "'Universal'") || strings.Contains(string(data), "'Zulu'") {
		t.Fatal("038 allowlist includes a timezone name Postgres rejects")
	}
}

func TestUTCSessionNames(t *testing.T) {
	for _, zone := range []string{"UTC", "Etc/UTC", "GMT", "Etc/GMT", "Etc/Universal"} {
		if !utcSession(zone) {
			t.Fatalf("utc session rejected %s", zone)
		}
	}
	for _, zone := range []string{"Europe/Rome", "Universal", "Zulu", ""} {
		if utcSession(zone) {
			t.Fatalf("utc session accepted %q", zone)
		}
	}
}

func TestSetDSNTimeZone(t *testing.T) {
	got, err := setDSNTimeZone("postgresql://affilio:password@127.0.0.1:5432/affilio_migration_test?sslmode=disable", "UTC")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("TimeZone") != "UTC" || u.Query().Get("sslmode") != "disable" {
		t.Fatalf("url session: %s", got)
	}
	replaced, err := setDSNTimeZone(got, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	u, err = url.Parse(replaced)
	if err != nil || u.Query().Get("TimeZone") != "UTC" || len(u.Query()["TimeZone"]) != 1 {
		t.Fatalf("url session replaced: %s", replaced)
	}
	keyword, err := setDSNTimeZone("host=127.0.0.1 port=5432 dbname=affilio_migration_test user=affilio password=secret sslmode=disable TimeZone=Europe/Rome", "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(keyword, "TimeZone=") != 1 || !strings.Contains(keyword, "TimeZone=UTC") || strings.Contains(keyword, "Europe/Rome") {
		t.Fatalf("keyword session: %s", keyword)
	}
}

func TestAutoMigrateRefusesNonUTCSession(t *testing.T) {
	raw := migrationDatabaseURL(t)
	rome := openTimeZone(t, raw, "Europe/Rome")
	err := RequireUTCSession(context.Background(), rome)
	if err == nil {
		t.Fatal("Europe/Rome session was accepted")
	}
	msg := err.Error()
	for _, part := range []string{
		"AUTO_MIGRATE refuses to start",
		"Europe/Rome",
		"postgresql.conf",
		"SHOW timezone",
		"does not change the session to UTC",
		"Leave AUTO_MIGRATE off",
	} {
		if !strings.Contains(msg, part) {
			t.Fatalf("startup error %q missing %q", msg, part)
		}
	}
	utc := openTimeZone(t, raw, "UTC")
	if err = RequireUTCSession(context.Background(), utc); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationHistoryAndConcurrentStartup(t *testing.T) {
	raw := os.Getenv("AFFILIO_MIGRATION_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated migration database not configured")
	}
	// Apply every migration, including 038, on an explicit UTC session.
	// A server-level timezone of Europe/Rome in postgresql.conf must not
	// reach the 038 guard through this test. Production Apply does not
	// set TimeZone; AUTO_MIGRATE refuses that session instead.
	dsn, err := setDSNTimeZone(raw, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var zone string
	if err = pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone != "UTC" {
		t.Fatalf("migration test session TimeZone is %s, want UTC (dsn %s)", zone, dsn)
	}
	if pool.Config().ConnConfig.Database != "affilio_migration_test" {
		t.Fatal("use the isolated affilio_migration_test database")
	}
	ctx := context.Background()
	if _, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE;CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- Apply(ctx, pool) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 38 {
		t.Fatalf("history: %d %v", count, err)
	}
	for _, col := range []struct{ table, column string }{
		{"clicks", "clicked_at"},
		{"conversions", "occurred_at"},
		{"ledger_entries", "clears_at"},
	} {
		var typ string
		if err = pool.QueryRow(ctx, `SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`, col.table, col.column).Scan(&typ); err != nil || typ != "timestamp with time zone" {
			t.Fatalf("%s.%s after Apply: %s %v", col.table, col.column, typ, err)
		}
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal("repeat migration failed", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE schema_migrations SET checksum='changed' WHERE name='001_initial.sql'`); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err == nil || !strings.Contains(err.Error(), "changed after application") {
		t.Fatal("changed migration was accepted")
	}
	if _, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE;CREATE SCHEMA public;CREATE TABLE existing_data(id INT)`); err != nil {
		t.Fatal(err)
	}
	if err = Apply(ctx, pool); err == nil {
		t.Fatal("untracked existing schema was migrated")
	}
}

func migrationDatabaseURL(t *testing.T) string {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("AFFILIO_MIGRATION_TEST_DATABASE_URL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("AFFILIO_TEST_DATABASE_URL"))
	}
	if raw == "" {
		t.Skip("migration database not configured")
	}
	return raw
}

func openTimeZone(t *testing.T, raw, zone string) *pgxpool.Pool {
	t.Helper()
	dsn, err := setDSNTimeZone(raw, zone)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var got string
	if err = pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != zone {
		t.Fatalf("session TimeZone %q, want %q", got, zone)
	}
	return pool
}

// setDSNTimeZone sets TimeZone on a URL or a keyword connection string.
// pgx sends that as the session startup parameter. libpq rejects TimeZone
// as a URI query parameter; this helper is for the pgx test pools only.
func setDSNTimeZone(raw, zone string) (string, error) {
	zone = strings.TrimSpace(zone)
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("TimeZone", zone)
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
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
	return strings.Join(out, " "), nil
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
