package migrations

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationHistoryAndConcurrentStartup(t *testing.T) {
	url := os.Getenv("AFFILIO_MIGRATION_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated migration database not configured")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
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
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 24 {
		t.Fatalf("history: %d %v", count, err)
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
