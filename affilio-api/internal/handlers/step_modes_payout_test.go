package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
)

func TestLinkChangeAskAndOff(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, r, "link-gate@example.com", "link-gate.example")
	_, offerA := seedListedOffer(t, db, "link-maker-a", "link-maker-a@example.com", "CodeLens")
	_, offerB := seedListedOffer(t, db, "link-maker-b", "link-maker-b@example.com", "Northstar")

	joined, joinedBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerA+"/join", nil, map[string]string{"X-API-Key": key})
	if joined.Code != http.StatusOK || joinedBody["tracking_code"] == "" || countRows(t, db, `SELECT COUNT(*) FROM campaigns WHERE site_id=$1 AND offer_id=$2`, siteID, offerA) != 1 {
		t.Fatalf("auto join: %d %s", joined.Code, joined.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"link_change": "off"})
	off, offBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerB+"/join", nil, map[string]string{"X-API-Key": key})
	if off.Code != http.StatusForbidden || offBody["reason"] != "step_off" || offBody["step"] != "link_change" || !strings.Contains(offBody["message"].(string), "Link change is off") || countRows(t, db, `SELECT COUNT(*) FROM campaigns WHERE site_id=$1 AND offer_id=$2`, siteID, offerB) != 0 {
		t.Fatalf("off join: %d %s", off.Code, off.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"link_change": "ask"})
	asked, askedBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerB+"/join", nil, map[string]string{"X-API-Key": key})
	if asked.Code != http.StatusAccepted || askedBody["reason"] != "step_ask" || countRows(t, db, `SELECT COUNT(*) FROM campaigns WHERE site_id=$1 AND offer_id=$2`, siteID, offerB) != 0 {
		t.Fatalf("ask join: %d %s", asked.Code, asked.Body.String())
	}
	approved := approveStep(t, r, askedBody["approval_id"].(string))
	if approved["result"].(map[string]any)["tracking_code"] == "" || countRows(t, db, `SELECT COUNT(*) FROM campaigns WHERE site_id=$1 AND offer_id=$2`, siteID, offerB) != 1 {
		t.Fatalf("approved join: %v", approved)
	}
}

func insertPaidPayout(t *testing.T, db *database.DB, siteID, makerID, status, mode string) string {
	t.Helper()
	var id string
	err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id, amount, currency, method, status, maker_id, payment_mode, paid_at)
VALUES($1, 40, 'USD', 'manual', $2, $3, $4, CASE WHEN $5::bool THEN NOW() ELSE NULL END) RETURNING id::text`, siteID, status, makerID, mode, status == "paid").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPayoutActionsAskAndOff(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	owner, cookie := testOwnerSession(t, h, "payout-owner", "payout-owner@example.com")
	siteID := createTestSite(t, db, "payout-site-key", "payout-site.example", "Payout Blog")
	makerID, err := db.CreateMaker(context.Background(), "payout-maker-key", "payout-maker@example.com", "Payout Maker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	key := "payout-site-key"
	headers := map[string]string{"X-API-Key": key}
	admin := map[string]string{"X-Admin-Key": "test-admin"}

	putStepModes(t, r, key, map[string]any{"payout": "off"})
	paidOff := insertPaidPayout(t, db, siteID, makerID, "paid", "live")
	offConfirm, offConfirmBody := postJSON(t, r, http.MethodPost, "/v1/payouts/"+paidOff+"/confirm-received", nil, headers)
	if offConfirm.Code != http.StatusForbidden || offConfirmBody["reason"] != "step_off" || offConfirmBody["step"] != "payout" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NULL`, paidOff) != 1 {
		t.Fatalf("off confirm: %d %s", offConfirm.Code, offConfirm.Body.String())
	}
	pendingOff := insertPaidPayout(t, db, siteID, makerID, "pending", "live")
	offPaid, offPaidBody := postJSON(t, r, http.MethodPost, "/v1/payout-batches/"+pendingOff+"/mark-paid", map[string]any{"provider_reference": "off-ref"}, admin)
	if offPaid.Code != http.StatusForbidden || offPaidBody["reason"] != "step_off" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND status='pending'`, pendingOff) != 1 {
		t.Fatalf("off mark paid: %d %s", offPaid.Code, offPaid.Body.String())
	}
	offProfile, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payout-profile", map[string]any{"instructions": "wire to off"})
	if offProfile.Code != http.StatusForbidden || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE id=$1 AND payout_instructions=''`, siteID) != 1 {
		t.Fatalf("off profile: %d %s", offProfile.Code, offProfile.Body.String())
	}
	offReport, offReportBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payments/"+pendingOff+"/report", map[string]any{"sent": true, "reference": "off-pay"})
	if offReport.Code != http.StatusForbidden || offReportBody["reason"] != "step_off" {
		t.Fatalf("off report: %d %s", offReport.Code, offReport.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"payout": "ask"})
	askConfirm, askConfirmBody := postJSON(t, r, http.MethodPost, "/v1/payouts/"+paidOff+"/confirm-received", nil, headers)
	if askConfirm.Code != http.StatusAccepted || askConfirmBody["reason"] != "step_ask" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NULL`, paidOff) != 1 {
		t.Fatalf("ask confirm: %d %s", askConfirm.Code, askConfirm.Body.String())
	}
	if approveStep(t, r, askConfirmBody["approval_id"].(string))["result"].(map[string]any)["status"] != "received" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND received_at IS NOT NULL`, paidOff) != 1 {
		t.Fatal("approved confirm did not stamp receipt")
	}
	askPaid, askPaidBody := postJSON(t, r, http.MethodPost, "/v1/payout-batches/"+pendingOff+"/mark-paid", map[string]any{"provider_reference": "ask-ref"}, admin)
	if askPaid.Code != http.StatusAccepted || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND status='pending'`, pendingOff) != 1 {
		t.Fatalf("ask mark paid: %d %s", askPaid.Code, askPaid.Body.String())
	}
	if approveStep(t, r, askPaidBody["approval_id"].(string))["result"].(map[string]any)["status"] != "paid" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND status='paid' AND provider_reference='ask-ref'`, pendingOff) != 1 {
		t.Fatal("approved mark paid did not pay")
	}
	askProfile, askProfileBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payout-profile", map[string]any{"instructions": "wire to ask"})
	if askProfile.Code != http.StatusAccepted || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE id=$1 AND payout_instructions=''`, siteID) != 1 {
		t.Fatalf("ask profile: %d %s", askProfile.Code, askProfile.Body.String())
	}
	if approveStep(t, r, askProfileBody["approval_id"].(string))["result"].(map[string]any)["saved"] != true || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE id=$1 AND payout_instructions='wire to ask'`, siteID) != 1 {
		t.Fatal("approved profile did not save")
	}
	reportPayout := insertPaidPayout(t, db, siteID, makerID, "pending", "live")
	askReport, askReportBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payments/"+reportPayout+"/report", map[string]any{"sent": true, "reference": "ask-pay"})
	if askReport.Code != http.StatusAccepted || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND status='pending'`, reportPayout) != 1 {
		t.Fatalf("ask report: %d %s", askReport.Code, askReport.Body.String())
	}
	if approveStep(t, r, askReportBody["approval_id"].(string))["result"].(map[string]any)["status"] != "payment_reported" || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE id=$1 AND status='paid' AND provider_reference='ask-pay'`, reportPayout) != 1 {
		t.Fatal("approved report did not pay")
	}

	putStepModes(t, r, key, map[string]any{"payout": "auto"})
	autoPaid := insertPaidPayout(t, db, siteID, makerID, "paid", "live")
	autoConfirm, autoConfirmBody := postJSON(t, r, http.MethodPost, "/v1/payouts/"+autoPaid+"/confirm-received", nil, headers)
	if autoConfirm.Code != http.StatusOK || autoConfirmBody["status"] != "received" {
		t.Fatalf("auto confirm: %d %s", autoConfirm.Code, autoConfirm.Body.String())
	}
	autoPending := insertPaidPayout(t, db, siteID, makerID, "pending", "live")
	autoMark, autoMarkBody := postJSON(t, r, http.MethodPost, "/v1/payout-batches/"+autoPending+"/mark-paid", map[string]any{"provider_reference": "auto-ref"}, admin)
	if autoMark.Code != http.StatusOK || autoMarkBody["status"] != "paid" {
		t.Fatalf("auto mark paid: %d %s", autoMark.Code, autoMark.Body.String())
	}
	autoProfile, autoProfileBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payout-profile", map[string]any{"instructions": "wire to auto"})
	if autoProfile.Code != http.StatusOK || autoProfileBody["saved"] != true {
		t.Fatalf("auto profile: %d %s", autoProfile.Code, autoProfile.Body.String())
	}
	autoReportPayout := insertPaidPayout(t, db, siteID, makerID, "pending", "live")
	autoReport, autoReportBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/owner/api/payments/"+autoReportPayout+"/report", map[string]any{"sent": true, "reference": "auto-pay"})
	if autoReport.Code != http.StatusOK || autoReportBody["status"] != "payment_reported" {
		t.Fatalf("auto report: %d %s", autoReport.Code, autoReport.Body.String())
	}
}

func insertClearedLedger(t *testing.T, db *database.DB, siteID, offerID, event string) {
	t.Helper()
	var conversionID string
	err := db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id, provider, provider_event_id, kind, amount, currency, commission_amount, occurred_at, payment_mode)
VALUES($1,'stripe',$2,'sale',200,'USD',40,NOW(),'live') RETURNING id`, offerID, event).Scan(&conversionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id, conversion_id, kind, amount, currency, status) VALUES($1,$2,'commission',40,'USD','cleared')`, siteID, conversionID); err != nil {
		t.Fatal(err)
	}
}

func TestPayoutBatchAskSkipsOffAndAutoPays(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, autoSite, _ := signupPublisher(t, r, "batch-auto@example.com", "batch-auto.example")
	_, askSite, askKey := signupPublisher(t, r, "batch-ask@example.com", "batch-ask.example")
	_, offSite, offKey := signupPublisher(t, r, "batch-off@example.com", "batch-off.example")
	_, offerID := seedListedOffer(t, db, "batch-maker-key", "batch-maker@example.com", "CodeLens")
	insertClearedLedger(t, db, autoSite, offerID, "batch-auto")
	insertClearedLedger(t, db, askSite, offerID, "batch-ask")
	insertClearedLedger(t, db, offSite, offerID, "batch-off")
	putStepModes(t, r, askKey, map[string]any{"payout": "ask"})
	putStepModes(t, r, offKey, map[string]any{"payout": "off"})

	ran, body := postJSON(t, r, http.MethodPost, "/v1/payout-batches/run", map[string]any{"min_payout": 10}, map[string]string{"X-Admin-Key": "test-admin"})
	payouts, _ := body["payouts"].([]any)
	if ran.Code != http.StatusOK || len(payouts) != 1 || payouts[0].(map[string]any)["site_id"] != autoSite {
		t.Fatalf("batch: %d %s", ran.Code, ran.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_approvals WHERE site_id=$1 AND step='payout' AND status='pending'`, askSite) != 1 {
		t.Fatal("ask site was not queued")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM ledger_entries WHERE site_id=$1 AND payout_id IS NULL`, offSite) != 1 || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE site_id=$1`, offSite) != 0 {
		t.Fatal("off site was paid or stamped")
	}
	var approvalID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text FROM agent_step_approvals WHERE site_id=$1 AND status='pending'`, askSite).Scan(&approvalID); err != nil {
		t.Fatal(err)
	}
	approved := approveStep(t, r, approvalID)
	if approved["result"].(map[string]any)["created"] != true || countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE site_id=$1`, askSite) != 1 {
		t.Fatalf("approved batch: %v", approved)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM ledger_entries WHERE site_id=$1 AND payout_id IS NULL`, offSite) != 1 {
		t.Fatal("approving ask stamped the off site")
	}
}

func TestStepModeMigrationIsIdempotent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join("..", "migrations", "042_d062_step_modes.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	if strings.Contains(sqlText, "BEGIN") || strings.Contains(sqlText, "COMMIT") || !strings.Contains(sqlText, "CREATE TABLE IF NOT EXISTS agent_step_modes") {
		t.Fatal("migration is not an additive idempotent file")
	}
	if _, err = db.Pool.Exec(context.Background(), `INSERT INTO agent_step_modes(scope_key, site_signup) VALUES('site:00000000-0000-0000-0000-000000000000', 'auto')`); err == nil {
		t.Fatal("a scope row without an owner or site was accepted")
	}
	before := countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes`)
	if _, err = db.Pool.Exec(context.Background(), sqlText); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), sqlText); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes`) != before || countRows(t, db, `SELECT COUNT(*) FROM information_schema.tables WHERE table_name IN ('agent_step_modes','agent_step_approvals')`) != 2 {
		t.Fatal("reapplying the migration changed stored rows or tables")
	}
}
