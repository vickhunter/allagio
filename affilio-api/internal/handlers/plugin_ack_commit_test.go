package handlers

import (
	"context"
	"net/http"
	"testing"

	"affilio-api/internal/entry"
)

func TestPluginAckRefusesUntilCommitted(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "ack-gate-owner", "ack-gate@example.com")
	siteID := createTestSite(t, db, "ack-gate-key", "ack-gate.example", "Gate")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "ack-gate-maker", "ack-gate-maker@example.com", "Gate Product")
	pluginKey := issuePluginKey(t, db, siteID, "ack-gate-plugin")
	siteKey := map[string]string{"X-API-Key": "ack-gate-key"}

	type stored struct {
		status, wait, reason, action string
	}
	row := func(id string) stored {
		t.Helper()
		var got stored
		err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait, plugin_failed_reason, failed_action FROM placement_proposals WHERE id=$1`, id).Scan(&got.status, &got.wait, &got.reason, &got.action)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	setStatus := func(id, status string) {
		t.Helper()
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status=$2 WHERE id=$1`, id, status); err != nil {
			t.Fatal(err)
		}
	}
	ack := func(id, action string, extra map[string]any) (int, map[string]any) {
		t.Helper()
		body := map[string]any{"action": action, "rel_sponsored": true}
		for key, value := range extra {
			body[key] = value
		}
		rec, parsed := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", body, pluginKey)
		return rec.Code, parsed
	}
	expectConflict := func(id, action, reason string, extra map[string]any, before stored) {
		t.Helper()
		want, ok := entry.RestoreRefusalByReason(reason)
		if !ok {
			t.Fatalf("missing reason %s", reason)
		}
		code, body := ack(id, action, extra)
		if code != http.StatusConflict || body["reason"] != reason || body["message"] != want.Text {
			t.Fatalf("%s %s: %d %v", before.status, action, code, body)
		}
		if got := row(id); got != before {
			t.Fatalf("%s %s wrote the row: %+v", before.status, action, got)
		}
	}

	pending := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/pending", "Gate Product note.")
	fresh := row(pending)
	if fresh.status != "pending" || fresh.wait != "" {
		t.Fatalf("new row: %+v", fresh)
	}
	expectConflict(pending, "placed", "not_committed", nil, fresh)
	expectConflict(pending, "restored", "not_committed", nil, fresh)
	expectConflict(pending, "failed", "not_committed", map[string]any{"reason": "re-read failed"}, fresh)
	expectConflict(pending, "removed", "nothing_to_ack", nil, fresh)

	setStatus(pending, "approved")
	approved := row(pending)
	expectConflict(pending, "placed", "not_committed", nil, approved)
	expectConflict(pending, "restored", "not_committed", nil, approved)
	expectConflict(pending, "failed", "not_committed", map[string]any{"reason": "re-read failed"}, approved)

	setStatus(pending, "committed")
	code, body := ack(pending, "placed", map[string]any{"rel_sponsored": false})
	if code != http.StatusUnprocessableEntity || body["check_id"] != "rel_sponsored" {
		t.Fatalf("committed without rel: %d %v", code, body)
	}
	if got := row(pending); got.wait != "" || got.status != "committed" {
		t.Fatalf("rel refusal wrote the row: %+v", got)
	}
	code, body = ack(pending, "placed", nil)
	if code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("committed placed: %d %v", code, body)
	}
	if got := row(pending); got.status != "committed" || got.wait != "placed" {
		t.Fatalf("placed row: %+v", got)
	}
	code, body = ack(pending, "placed", nil)
	if code != http.StatusConflict || body["reason"] != "nothing_to_ack" {
		t.Fatalf("second placed: %d %v", code, body)
	}

	again := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/again", "Gate Product note.")
	setStatus(again, "committed")
	code, body = ack(again, "restored", nil)
	if code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("committed restored: %d %v", code, body)
	}

	failedInsert := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/fail-insert", "Gate Product note.")
	setStatus(failedInsert, "committed")
	code, body = ack(failedInsert, "failed", map[string]any{"reason": "re-read failed"})
	if code != http.StatusOK || body["plugin_state"] != "failed" || body["failed_action"] != "restored" {
		t.Fatalf("committed failed insert: %d %v", code, body)
	}
	if got := row(failedInsert); got.status != "committed" || got.reason != "re-read failed" || got.action != "restored" {
		t.Fatalf("failed insert row: %+v", got)
	}

	reopened := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/reopened", "Gate Product note.")
	setStatus(reopened, "committed")
	setStatus(reopened, "pending")
	expectConflict(reopened, "placed", "not_committed", nil, row(reopened))

	restoreApproved := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/restore-approved", "Gate Product note.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', plugin_wait='restore' WHERE id=$1`, restoreApproved); err != nil {
		t.Fatal(err)
	}
	expectConflict(restoreApproved, "restored", "not_committed", nil, row(restoreApproved))
	expectConflict(restoreApproved, "failed", "not_committed", map[string]any{"reason": "re-read failed"}, row(restoreApproved))

	restoreCommitted := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/restore-committed", "Gate Product note.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='restore' WHERE id=$1`, restoreCommitted); err != nil {
		t.Fatal(err)
	}
	code, body = ack(restoreCommitted, "restored", nil)
	if code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("restore ack: %d %v", code, body)
	}

	removal := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/removal", "Gate Product note.")
	setStatus(removal, "committed")
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+removal+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	revoked := row(removal)
	if revoked.status != "revoked" || revoked.wait != "remove" {
		t.Fatalf("undo row: %+v", revoked)
	}
	expectConflict(removal, "placed", "not_committed", nil, revoked)
	expectConflict(removal, "restored", "not_committed", nil, revoked)
	code, body = ack(removal, "removed", nil)
	if code != http.StatusOK || body["plugin_state"] != "removed" {
		t.Fatalf("removed: %d %v", code, body)
	}
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+removal+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore after removal: %d %s", restored.Code, restored.Body.String())
	}
	if got := row(removal); got.status != "committed" || got.wait != "restore" {
		t.Fatalf("restored committed row: %+v", got)
	}
	code, body = ack(removal, "restored", nil)
	if code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("ack after real restore: %d %v", code, body)
	}

	failedRemoval := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/fail-removal", "Gate Product note.")
	setStatus(failedRemoval, "committed")
	undone, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+failedRemoval+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("removal undo: %d %s", undone.Code, undone.Body.String())
	}
	code, body = ack(failedRemoval, "failed", map[string]any{"reason": "remove re-read failed"})
	if code != http.StatusOK || body["plugin_state"] != "failed" || body["failed_action"] != "removed" {
		t.Fatalf("failed removal: %d %v", code, body)
	}
	if got := row(failedRemoval); got.status != "revoked" || got.wait != "remove" || got.action != "removed" {
		t.Fatalf("failed removal row: %+v", got)
	}

	live := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/live", "Gate Product note.")
	setStatus(live, "committed")
	sibling := insertPlacement(t, db, siteID, offerID, "https://ack-gate.example/live", "Gate Product second note.")
	setStatus(sibling, "approved")
	expectConflict(sibling, "placed", "not_committed", nil, row(sibling))
	setStatus(sibling, "committed")
	code, body = ack(sibling, "placed", nil)
	if code != http.StatusUnprocessableEntity || body["check_id"] != "one_per_post" {
		t.Fatalf("committed second box: %d %v", code, body)
	}
	if got := row(sibling); got.status != "committed" || got.wait != "" {
		t.Fatalf("one_per_post wrote the row: %+v", got)
	}
}
