package handlers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/services"
)

func TestRotateSiteAPIKeyUsesASingleHourInterval(t *testing.T) {
	body := rotateSiteAPIKeySource(t)
	if strings.Contains(body, `' hours'`) {
		t.Fatal("RotateSiteAPIKey appends ' hours' onto a value that already names the unit, which PostgreSQL reads as '24 hours hours'")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "interval-rotation-key", "example.com", "Interval Site")
	if err := db.RotateSiteAPIKey(context.Background(), siteID, services.HashAPIKey("interval-next-key"), services.KeyPrefix("interval-next-key"), 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	var hours float64
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXTRACT(EPOCH FROM (api_key_previous_expires_at - api_key_rotated_at)) / 3600 FROM sites WHERE id=$1`, siteID).Scan(&hours); err != nil {
		t.Fatal(err)
	}
	if hours < 23.9 || hours > 24.1 {
		t.Fatalf("previous key grace = %v hours, want 24", hours)
	}
}

func rotateSiteAPIKeySource(t *testing.T) string {
	t.Helper()
	var src []byte
	var err error
	for _, path := range []string{"internal/database/queries.go", "../database/queries.go"} {
		src, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (d *DB) RotateSiteAPIKey")
	if start < 0 {
		t.Fatal("RotateSiteAPIKey is missing")
	}
	body = body[start:]
	if end := strings.Index(body, "\nfunc "); end > 0 {
		body = body[:end]
	}
	return body
}
