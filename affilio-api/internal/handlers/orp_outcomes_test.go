package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

func inboxOutcomes(t *testing.T, db *database.DB, ownerID string) []map[string]any {
	t.Helper()
	messages, _, err := db.ListORPInbox(context.Background(), ownerID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	out := []map[string]any{}
	for _, raw := range messages {
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err == nil && doc["type"] == "outcome" {
			out = append(out, doc)
		}
	}
	return out
}

func outcomeRows(t *testing.T, db *database.DB) int {
	t.Helper()
	return countRows(t, db, `SELECT COUNT(*) FROM orp_outcomes`)
}

// The weekly sweep sends each party an outcome for an active tracked deal,
// dedupes a repeated run, opens a new period after a week, and sends exactly
// one final when the deal ends.
func TestORPOutcomeSweep(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	clockNow := time.Now().UTC()
	host.Now = func() time.Time { return clockNow }
	advance := func(d time.Duration) { clockNow = clockNow.Add(d) }
	ctx := context.Background()

	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, db, r, pubToken, publisher, dealID, termsHash)

	dealRec, dealDoc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(makerToken))
	tracking, _ := dealDoc["body"].(map[string]any)["tracking"].([]any)
	if dealRec.Code != http.StatusOK || len(tracking) != 1 {
		t.Fatalf("tracking %s", dealRec.Body.String())
	}
	link, _ := tracking[0].(map[string]any)["link"].(string)

	clickClassed(t, r, link, browserUA)
	advance(6 * time.Second)
	clickClassed(t, r, link, browserUA)
	clickClassed(t, r, link, "")

	sweepAt := clockNow.Add(time.Hour)
	stats, err := SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, sweepAt)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Deals != 1 || stats.Sent != 1 || stats.Finals != 0 {
		t.Fatalf("first sweep %+v", stats)
	}
	for _, party := range []string{maker, publisher} {
		outcomes := inboxOutcomes(t, entry.DB, ownerOfParty(party))
		if len(outcomes) != 1 {
			t.Fatalf("inbox %s has %d outcomes", party, len(outcomes))
		}
		doc := outcomes[0]
		if err := orp.ValidateAgainst(schemaPath(t, "messages/outcome.schema.json"), doc); err != nil {
			t.Fatalf("outcome schema: %v %s", err, mustJSON(doc))
		}
		if doc["orp"] != "0.3" || doc["deal"] != dealID || doc["to"] != party || doc["from"] != "orp:127.0.0.1:8080/host" {
			t.Fatalf("envelope %s", mustJSON(doc))
		}
		body, _ := doc["body"].(map[string]any)
		rows, _ := body["rows"].([]any)
		if len(rows) != 1 || body["final"] != nil {
			t.Fatalf("body %s", mustJSON(doc))
		}
		row, _ := rows[0].(map[string]any)
		clicks, _ := row["clicks"].(map[string]any)
		if row["obligation"] != "place" || clicks["human"] != float64(2) || clicks["bot"] != float64(1) {
			t.Fatalf("row %s", mustJSON(row))
		}
		if _, ok := clicks["repeat"]; ok {
			t.Fatalf("repeat counted %s", mustJSON(row))
		}
	}
	if outcomeRows(t, entry.DB) != 1 {
		t.Fatal("outcome marker not stored")
	}

	stats, err = SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, sweepAt)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sent != 0 || len(inboxOutcomes(t, entry.DB, ownerOfParty(maker))) != 1 {
		t.Fatalf("repeat sweep sent %+v", stats)
	}

	week := sweepAt.Add(8 * 24 * time.Hour)
	stats, err = SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, week)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sent != 1 || outcomeRows(t, entry.DB) != 2 {
		t.Fatalf("weekly sweep %+v", stats)
	}
	latest := inboxOutcomes(t, entry.DB, ownerOfParty(maker))[1]
	latestBody, _ := latest["body"].(map[string]any)
	latestRow, _ := latestBody["rows"].([]any)[0].(map[string]any)
	latestClicks, _ := latestRow["clicks"].(map[string]any)
	if latestClicks["human"] != float64(0) || latestBody["final"] != nil {
		t.Fatalf("second period %s", mustJSON(latest))
	}

	revoke := orpEnvelope("revoke", "m_out_rvk1", publisher, map[string]any{})
	revoke["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", revoke, bearerHeader(pubToken))
	inner, _ := body["body"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "revoked" {
		t.Fatalf("revoke %d %s", rec.Code, rec.Body.String())
	}

	endAt := week.Add(24 * time.Hour)
	stats, err = SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, endAt)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sent != 1 || stats.Finals != 1 || outcomeRows(t, entry.DB) != 3 {
		t.Fatalf("final sweep %+v", stats)
	}
	for _, party := range []string{maker, publisher} {
		outcomes := inboxOutcomes(t, entry.DB, ownerOfParty(party))
		if len(outcomes) != 3 {
			t.Fatalf("final inbox %s has %d outcomes", party, len(outcomes))
		}
		final := outcomes[2]
		if err := orp.ValidateAgainst(schemaPath(t, "messages/outcome.schema.json"), final); err != nil {
			t.Fatalf("final schema: %v %s", err, mustJSON(final))
		}
		finalBody, _ := final["body"].(map[string]any)
		if finalBody["final"] != true {
			t.Fatalf("final flag %s", mustJSON(final))
		}
	}

	stats, err = SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, endAt)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sent != 0 || len(inboxOutcomes(t, entry.DB, ownerOfParty(maker))) != 3 {
		t.Fatalf("second final sent %+v", stats)
	}
	stats, err = SweepORPOutcomes(ctx, entry.DB, entry.BaseURL, endAt.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sent != 0 || len(inboxOutcomes(t, entry.DB, ownerOfParty(maker))) != 3 {
		t.Fatalf("outcome after final %+v", stats)
	}
}
