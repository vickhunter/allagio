package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/database"
)

func setStepMode(t *testing.T, db *database.DB, ownerID, column, mode string) {
	t.Helper()
	if column != "share" && column != "revoke" {
		t.Fatalf("step column %s", column)
	}
	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM agent_step_modes WHERE owner_id=$1 AND site_id IS NULL`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO agent_step_modes(scope_key, owner_id, `+column+`)
VALUES('owner:' || $1::text, $1::uuid, $2)`, ownerID, mode); err != nil {
		t.Fatal(err)
	}
}

func shareTerms() map[string]any {
	terms := testTerms()
	terms["obligations"] = []any{map[string]any{
		"id":     "share",
		"action": "share",
		"by":     "acceptor",
		"data":   map[string]any{"kind": "click_counts"},
	}}
	return terms
}

func shareDeal(t *testing.T, r http.Handler, makerToken, maker, publisher string) (dealID, termsHash string) {
	t.Helper()
	offer := orpEnvelope("offer", "m_share_offer", maker, map[string]any{"terms": shareTerms()})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	inner, _ := body["body"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "proposed" {
		t.Fatalf("share offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ = body["deal"].(string)
	termsHash, _ = inner["terms_hash"].(string)
	return dealID, termsHash
}

func executeShare(t *testing.T, r http.Handler, token, publisher, dealID, suffix string) (rec *httptest.ResponseRecorder, body map[string]any) {
	t.Helper()
	msg := orpEnvelope("execute", "m_share_"+suffix, publisher, map[string]any{"obligation": "share"})
	msg["deal"] = dealID
	return postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(token))
}

func obligationState(t *testing.T, db *database.DB, dealID, obligationID string) string {
	t.Helper()
	var state string
	err := db.Pool.QueryRow(context.Background(), `
SELECT elem->>'state'
FROM orp_deals, jsonb_array_elements(obligations) elem
WHERE orp_deals.id=$1 AND elem->>'id'=$2`, dealID, obligationID).Scan(&state)
	if err != nil {
		t.Fatalf("obligation %s: %v", obligationID, err)
	}
	return state
}

// A share claim under the owner's ask step holds the deal awaiting; the waiting
// party's agent approves it and the claim lands.
func TestORPShareAskHold(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setStepMode(t, entry.DB, ownerOfParty(publisher), "share", "ask")
	dealID, termsHash := shareDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	rec, body := executeShare(t, r, pubToken, publisher, dealID, "exec01")
	inner, _ := body["body"].(map[string]any)
	waiting, _ := inner["waiting"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "awaiting" || waiting["step"] != "share" {
		t.Fatalf("share claim should hold %d %s", rec.Code, rec.Body.String())
	}
	if obligationState(t, entry.DB, dealID, "share") == "claimed" {
		t.Fatal("share claim landed before approval")
	}

	actRec, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(pubToken))
	if actRec.Code != http.StatusOK || actBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("approve held share %d %s", actRec.Code, actRec.Body.String())
	}
	if got := obligationState(t, entry.DB, dealID, "share"); got != "claimed" {
		t.Fatalf("share obligation state %s", got)
	}
}

func TestORPShareOffRefused(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setStepMode(t, entry.DB, ownerOfParty(publisher), "share", "off")
	dealID, termsHash := shareDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	rec, body := executeShare(t, r, pubToken, publisher, dealID, "exec02")
	if rec.Code != http.StatusForbidden || body["body"].(map[string]any)["code"] != "step_off" {
		t.Fatalf("share off should refuse %d %s", rec.Code, rec.Body.String())
	}
	if got := dealState(t, entry.DB, dealID); got != "active" {
		t.Fatalf("deal state %s", got)
	}
}

func TestORPShareAutoClaims(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setStepMode(t, entry.DB, ownerOfParty(publisher), "share", "auto")
	dealID, termsHash := shareDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	rec, body := executeShare(t, r, pubToken, publisher, dealID, "exec03")
	if rec.Code != http.StatusOK || body["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("share auto claim %d %s", rec.Code, rec.Body.String())
	}
	if got := obligationState(t, entry.DB, dealID, "share"); got != "claimed" {
		t.Fatalf("share obligation state %s", got)
	}
}

// A revoke under the owner's ask step holds; approval lands the revocation.
func TestORPRevokeAskHold(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setStepMode(t, entry.DB, ownerOfParty(publisher), "revoke", "ask")
	dealID, termsHash := directedDealID(t, r, makerToken, maker, publisher, "rev_offer")
	activateDealID(t, r, pubToken, publisher, dealID, termsHash, "rev_accept")

	revoke := orpEnvelope("revoke", "m_share_rev01", publisher, map[string]any{"scope": "deal", "reason": "owner_request"})
	revoke["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", revoke, bearerHeader(pubToken))
	inner, _ := body["body"].(map[string]any)
	waiting, _ := inner["waiting"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "awaiting" || waiting["step"] != "revoke" {
		t.Fatalf("revoke should hold %d %s", rec.Code, rec.Body.String())
	}

	actRec, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(pubToken))
	if actRec.Code != http.StatusOK || actBody["body"].(map[string]any)["state"] != "revoked" {
		t.Fatalf("approve held revoke %d %s", actRec.Code, actRec.Body.String())
	}
	if got := dealState(t, entry.DB, dealID); got != "revoked" {
		t.Fatalf("deal state %s", got)
	}
}

func TestORPRevokeOffRefused(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setStepMode(t, entry.DB, ownerOfParty(publisher), "revoke", "off")
	dealID, termsHash := directedDealID(t, r, makerToken, maker, publisher, "rev_offer2")
	activateDealID(t, r, pubToken, publisher, dealID, termsHash, "rev_accept2")

	revoke := orpEnvelope("revoke", "m_share_rev02", publisher, map[string]any{"scope": "deal", "reason": "owner_request"})
	revoke["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", revoke, bearerHeader(pubToken))
	if rec.Code != http.StatusForbidden || body["body"].(map[string]any)["code"] != "step_off" {
		t.Fatalf("revoke off should refuse %d %s", rec.Code, rec.Body.String())
	}
	if got := dealState(t, entry.DB, dealID); got != "active" {
		t.Fatalf("deal state %s", got)
	}
}
