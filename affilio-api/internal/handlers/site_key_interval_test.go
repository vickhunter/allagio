package handlers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
)

func TestRotateSiteAPIKeyUsesASingleHourInterval(t *testing.T) {
	body := rotateSiteAPIKeySource(t)
	if strings.Contains(body, `' hours'`) {
		t.Fatal("RotateSiteAPIKey appends ' hours' onto a value that already names the unit, which PostgreSQL reads as '24 hours hours'")
	}
	if !strings.Contains(body, "AT TIME ZONE 'UTC'") {
		t.Fatal("RotateSiteAPIKey must store the UTC face of the timestamp columns")
	}

	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	ctx := context.Background()
	var zone string
	if err := db.Pool.QueryRow(ctx, `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone != "Europe/Rome" {
		t.Fatalf("session timezone %q, want Europe/Rome", zone)
	}

	// 2026-10-25 01:00 UTC is 03:00 CEST, the Europe/Rome fallback to CET.
	fallback := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)
	samples := []struct {
		name string
		at   time.Time
	}{
		{name: "before", at: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)},
		{name: "across", at: time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC)},
		{name: "after", at: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)},
	}
	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			siteID := createTestSite(t, db, "interval-"+sample.name, sample.name+".example.com", "Interval "+sample.name)
			restore := database.SetSiteKeyNowForTest(func() time.Time { return sample.at })
			defer restore()
			if err := db.RotateSiteAPIKey(ctx, siteID, services.HashAPIKey("interval-next-"+sample.name), services.KeyPrefix("interval-next-"+sample.name), 24*time.Hour); err != nil {
				t.Fatal(err)
			}
			var hours float64
			var rotated, expires time.Time
			err := db.Pool.QueryRow(ctx, `
SELECT EXTRACT(EPOCH FROM (
         (api_key_previous_expires_at AT TIME ZONE 'UTC')
       - (api_key_rotated_at AT TIME ZONE 'UTC')
       )) / 3600,
       api_key_rotated_at AT TIME ZONE 'UTC',
       api_key_previous_expires_at AT TIME ZONE 'UTC'
FROM sites WHERE id=$1`, siteID).Scan(&hours, &rotated, &expires)
			if err != nil {
				t.Fatal(err)
			}
			if hours < 23.99 || hours > 24.01 {
				t.Fatalf("previous key grace = %v hours, want 24", hours)
			}
			rotated, expires = rotated.UTC(), expires.UTC()
			switch sample.name {
			case "before":
				if !expires.Before(fallback) {
					t.Fatalf("before window expires at %s, want it before %s", expires, fallback)
				}
			case "across":
				if !rotated.Before(fallback) || !expires.After(fallback) {
					t.Fatalf("across window %s .. %s does not cover %s", rotated, expires, fallback)
				}
			case "after":
				if !rotated.After(fallback) {
					t.Fatalf("after window starts at %s, want it after %s", rotated, fallback)
				}
			default:
				t.Fatalf("unknown sample %s", sample.name)
			}
		})
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
