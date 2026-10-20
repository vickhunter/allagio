package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/entry"
)

func putOwnerModes(t *testing.T, r http.Handler, h *HostedHandler, cookie *http.Cookie, csrf string, modes map[string]any) {
	t.Helper()
	rec, body := serveOwner(t, r, h, cookie, csrf, http.MethodPut, "/v1/step-modes", modes)
	if rec.Code != http.StatusOK || body["modes"] == nil {
		t.Fatalf("owner modes: %d %s", rec.Code, rec.Body.String())
	}
}

func approvalIDFromPage(t *testing.T, body string) string {
	t.Helper()
	const marker = `data-approval-id="`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("approval id missing: %s", body)
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		t.Fatalf("approval id missing: %s", body)
	}
	return rest[:end]
}

func TestOwnerConfirmReceivedOffAndAsk(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	owner, cookie := testOwnerSession(t, h, "owner-receipt", "owner-receipt@example.com")
	siteID := createTestSite(t, db, "owner-receipt-key", "owner-receipt.example", "Receipt Blog")
	makerID, err := db.CreateMaker(context.Background(), "owner-receipt-maker", "owner-receipt-maker@example.com", "Receipt Maker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	payoutID := insertPaidPayout(t, db, siteID, makerID, "paid", "live")
	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"payout": "off"})

	page := httptest.NewRecorder()
	pageReq := httptest.NewRequest(http.MethodGet, "/owner", nil)
	pageReq.AddCookie(cookie)
	r.ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "step_off") || !strings.Contains(page.Body.String(), "Payout actions are off") || strings.Contains(page.Body.String(), "/owner/payouts/"+payoutID+"/confirm-received") {
		t.Fatalf("dashboard off: %d %s", page.Code, page.Body.String())
	}

	off := ownerForm(t, h, r, cookie, "/owner/payouts/"+payoutID+"/confirm-received", url.Values{"csrf": {owner.CSRF}})
	if off.Code != http.StatusForbidden || !strings.Contains(off.Body.String(), "step_off") || !strings.Contains(off.Body.String(), "Payout actions are off") || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NULL`, payoutID) != 1 {
		t.Fatalf("form off: %d %s", off.Code, off.Body.String())
	}
	offJSON, offJSONBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/payouts/"+payoutID+"/confirm-received", map[string]any{})
	if offJSON.Code != http.StatusForbidden || offJSONBody["reason"] != "step_off" || offJSONBody["step"] != "payout" || offJSONBody["error"] != offJSONBody["message"] {
		t.Fatalf("json off: %d %s", offJSON.Code, offJSON.Body.String())
	}

	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"payout": "ask"})
	asked := ownerForm(t, h, r, cookie, "/owner/payouts/"+payoutID+"/confirm-received", url.Values{"csrf": {owner.CSRF}})
	if asked.Code != http.StatusAccepted || !strings.Contains(asked.Body.String(), "step_ask") || !strings.Contains(asked.Body.String(), "This payout action is waiting for approval.") || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NULL`, payoutID) != 1 {
		t.Fatalf("form ask: %d %s", asked.Code, asked.Body.String())
	}
	approved := approveStep(t, r, approvalIDFromPage(t, asked.Body.String()))
	if approved["result"].(map[string]any)["status"] != "received" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NOT NULL`, payoutID) != 1 {
		t.Fatalf("approved receipt: %v", approved)
	}
}

func TestOwnerSignupOffAndAsk(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	owner, cookie := testOwnerSession(t, h, "owner-signup", "owner-signup@example.com")
	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"site_signup": "off"})

	off := ownerForm(t, h, r, cookie, "/owner/accounts", url.Values{"csrf": {owner.CSRF}, "role": {"publisher"}, "domain": {"owner-signup-off.example"}})
	if off.Code != http.StatusForbidden || !strings.Contains(off.Body.String(), "step_off") || !strings.Contains(off.Body.String(), "Site signup is off") || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='owner-signup-off.example'`) != 0 {
		t.Fatalf("signup off: %d %s", off.Code, off.Body.String())
	}

	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"site_signup": "ask"})
	asked := ownerForm(t, h, r, cookie, "/owner/accounts", url.Values{"csrf": {owner.CSRF}, "role": {"publisher"}, "domain": {"owner-signup-ask.example"}})
	if asked.Code != http.StatusAccepted || !strings.Contains(asked.Body.String(), "step_ask") || !strings.Contains(asked.Body.String(), "Site signup is waiting for approval.") || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='owner-signup-ask.example'`) != 0 {
		t.Fatalf("signup ask: %d %s", asked.Code, asked.Body.String())
	}
	approved := approveStep(t, r, approvalIDFromPage(t, asked.Body.String()))
	if approved["result"].(map[string]any)["site_id"] == "" || countRows(t, db, `SELECT COUNT(*) FROM owners WHERE id=$1 AND site_id IS NOT NULL`, owner.ID) != 1 {
		t.Fatalf("approved signup: %v", approved)
	}
}

func TestOwnerMatchPublishAndCommitOffAndAsk(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	owner, cookie := testOwnerSession(t, h, "owner-publish", "owner-publish@example.com")
	siteID := createTestSite(t, db, "owner-publish-key", "owner-publish.example", "Publish Blog")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "owner-publish-maker", "owner-publish-maker@example.com", "CodeLens")
	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"offer_match": "off", "placement_publish": "off"})

	offMatch := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/opportunities", map[string]any{"context": stepArticle("CodeLens")})
	var offMatchBody map[string]any
	if err := json.Unmarshal(offMatch.Body.Bytes(), &offMatchBody); err != nil || offMatch.Code != http.StatusForbidden || offMatchBody["reason"] != "step_off" || offMatchBody["step"] != "offer_match" {
		t.Fatalf("match off: %d %s", offMatch.Code, offMatch.Body.String())
	}
	offPublish := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://owner-publish.example/posts/off", "context": stepArticle("CodeLens"), "copy": stepCopy, "post_id": 1,
	})
	var offPublishBody map[string]any
	if err := json.Unmarshal(offPublish.Body.Bytes(), &offPublishBody); err != nil || offPublish.Code != http.StatusForbidden || offPublishBody["reason"] != "step_off" || countRows(t, db, `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1`, siteID) != 0 {
		t.Fatalf("publish off: %d %s", offPublish.Code, offPublish.Body.String())
	}

	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"offer_match": "ask", "placement_publish": "ask"})
	askedMatch := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/opportunities", map[string]any{"context": stepArticle("CodeLens")})
	var askedMatchBody map[string]any
	if err := json.Unmarshal(askedMatch.Body.Bytes(), &askedMatchBody); err != nil || askedMatch.Code != http.StatusAccepted {
		t.Fatalf("match ask: %d %s", askedMatch.Code, askedMatch.Body.String())
	}
	requireHold(t, askedMatchBody, "step_ask")
	matched := approveStep(t, r, askedMatchBody["approval_id"].(string))
	found, _ := matched["result"].(map[string]any)["opportunities"].([]any)
	if len(found) == 0 || found[0].(map[string]any)["offer_id"] != offerID {
		t.Fatalf("approved match: %v", matched)
	}

	askedPublish := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://owner-publish.example/posts/asked", "context": stepArticle("CodeLens"), "copy": stepCopy, "post_id": 2,
	})
	var askedPublishBody map[string]any
	if err := json.Unmarshal(askedPublish.Body.Bytes(), &askedPublishBody); err != nil {
		t.Fatal(err)
	}
	placement, _ := askedPublishBody["placement"].(map[string]any)
	placementID, _ := placement["id"].(string)
	if askedPublish.Code != http.StatusCreated || placement["status"] != "pending" || placementID == "" {
		t.Fatalf("publish ask: %d %s", askedPublish.Code, askedPublish.Body.String())
	}
	requireHold(t, askedPublishBody, "step_ask")
	approved := ownerForm(t, h, r, cookie, "/owner/placements/"+placementID+"/approve", url.Values{"csrf": {owner.CSRF}})
	if approved.Code != http.StatusSeeOther {
		t.Fatalf("owner approve: %d %s", approved.Code, approved.Body.String())
	}
	committed := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/placements/"+placementID+"/commit", map[string]any{})
	if committed.Code != http.StatusOK || countRows(t, db, `SELECT COUNT(*) FROM placement_proposals WHERE id=$1 AND status='committed'`, placementID) != 1 {
		t.Fatalf("commit while ask: %d %s", committed.Code, committed.Body.String())
	}

	second := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://owner-publish.example/posts/second", "context": stepArticle("CodeLens"), "copy": stepCopy, "post_id": 3,
	})
	var secondBody map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatal(err)
	}
	secondID, _ := secondBody["placement"].(map[string]any)["id"].(string)
	if second.Code != http.StatusCreated || secondID == "" {
		t.Fatalf("second ask: %d %s", second.Code, second.Body.String())
	}
	if again := ownerForm(t, h, r, cookie, "/owner/placements/"+secondID+"/approve", url.Values{"csrf": {owner.CSRF}}); again.Code != http.StatusSeeOther {
		t.Fatalf("second approve: %d %s", again.Code, again.Body.String())
	}
	putOwnerModes(t, r, h, cookie, owner.CSRF, map[string]any{"placement_publish": "off"})
	blocked := ownerJSON(t, h, r, cookie, owner.CSRF, http.MethodPost, "/owner/api/placements/"+secondID+"/commit", map[string]any{})
	var blockedBody map[string]any
	if err := json.Unmarshal(blocked.Body.Bytes(), &blockedBody); err != nil || blocked.Code != http.StatusForbidden || blockedBody["reason"] != "step_off" || blockedBody["step"] != "placement_publish" || countRows(t, db, `SELECT COUNT(*) FROM placement_proposals WHERE id=$1 AND status='approved'`, secondID) != 1 {
		t.Fatalf("commit off: %d %s", blocked.Code, blocked.Body.String())
	}
}
