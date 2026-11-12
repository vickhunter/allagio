package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/database"
)

func trustHost(t *testing.T) (entry *AgentEntry, r *gin.Engine) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	entry = &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	return entry, mountORP(entry, &ORPHost{Entry: entry})
}

func directedDeal(t *testing.T, r *gin.Engine, makerToken, maker, publisher string) (dealID, termsHash string) {
	t.Helper()
	return directedDealID(t, r, makerToken, maker, publisher, "offer01")
}

func directedDealID(t *testing.T, r *gin.Engine, makerToken, maker, publisher, suffix string) (dealID, termsHash string) {
	t.Helper()
	offer := orpEnvelope("offer", "m_trust_"+suffix, maker, standingOffer())
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	inner, _ := body["body"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "proposed" {
		t.Fatalf("directed offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ = body["deal"].(string)
	termsHash, _ = inner["terms_hash"].(string)
	return dealID, termsHash
}

func activateDeal(t *testing.T, db *database.DB, r *gin.Engine, pubToken, publisher, dealID, termsHash string) {
	t.Helper()
	activateDealID(t, r, pubToken, publisher, dealID, termsHash, "accept1")
}

func activateDealID(t *testing.T, r *gin.Engine, pubToken, publisher, dealID, termsHash, suffix string) {
	t.Helper()
	accept := orpEnvelope("accept", "m_trust_"+suffix, publisher, map[string]any{"terms_hash": termsHash})
	accept["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", accept, bearerHeader(pubToken))
	if rec.Code != http.StatusOK || body["type"] != "deal" {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	actRec, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(pubToken))
	if actRec.Code != http.StatusOK || actBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("activate %d %s", actRec.Code, actRec.Body.String())
	}
}

func dealState(t *testing.T, db *database.DB, dealID string) string {
	t.Helper()
	var state string
	if err := db.Pool.QueryRow(context.Background(), `SELECT state FROM orp_deals WHERE id=$1`, dealID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestORPDecline(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	thirdToken, third := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)

	stranger := orpEnvelope("decline", "m_trust_dec00", third, map[string]any{"reason": "other"})
	stranger["deal"] = dealID
	strangerRec, _ := postJSON(t, r, http.MethodPost, "/orp", stranger, bearerHeader(thirdToken))
	if strangerRec.Code != http.StatusNotFound {
		t.Fatalf("stranger decline %d %s", strangerRec.Code, strangerRec.Body.String())
	}

	decline := orpEnvelope("decline", "m_trust_dec01", publisher, map[string]any{"reason": "not_relevant"})
	decline["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", decline, bearerHeader(pubToken))
	if rec.Code != http.StatusOK || body["type"] != "deal" || body["body"].(map[string]any)["state"] != "declined" {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	if dealState(t, entry.DB, dealID) != "declined" {
		t.Fatal("decline not stored")
	}

	accept := orpEnvelope("accept", "m_trust_acc01", publisher, map[string]any{"terms_hash": termsHash})
	accept["deal"] = dealID
	acceptRec, _ := postJSON(t, r, http.MethodPost, "/orp", accept, bearerHeader(pubToken))
	if acceptRec.Code != http.StatusConflict {
		t.Fatalf("accept after decline %d %s", acceptRec.Code, acceptRec.Body.String())
	}
}

func TestORPRevoke(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")

	// scope=deal on an active deal
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)
	revoke := orpEnvelope("revoke", "m_trust_rev01", maker, map[string]any{"scope": "deal", "reason": "owner_request"})
	revoke["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", revoke, bearerHeader(makerToken))
	if rec.Code != http.StatusOK || body["body"].(map[string]any)["state"] != "revoked" {
		t.Fatalf("revoke %d %s", rec.Code, rec.Body.String())
	}
	if dealState(t, entry.DB, dealID) != "revoked" {
		t.Fatal("revoke not stored")
	}
	exec := orpEnvelope("execute", "m_trust_ex001", publisher, map[string]any{"obligation": "place", "proof_url": "https://example.com/x"})
	exec["deal"] = dealID
	execRec, _ := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(pubToken))
	if execRec.Code != http.StatusConflict {
		t.Fatalf("execute on revoked %d %s", execRec.Code, execRec.Body.String())
	}

	// scope=obligation on a second active deal
	deal2, hash2 := directedDealID(t, r, makerToken, maker, publisher, "offer02")
	activateDealID(t, r, pubToken, publisher, deal2, hash2, "accept2")
	revObl := orpEnvelope("revoke", "m_trust_rev02", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "brand_safety"})
	revObl["deal"] = deal2
	rec2, body2 := postJSON(t, r, http.MethodPost, "/orp", revObl, bearerHeader(pubToken))
	if rec2.Code != http.StatusOK || body2["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("obligation revoke %d %s", rec2.Code, rec2.Body.String())
	}
	var obState string
	if err := entry.DB.Pool.QueryRow(context.Background(), `SELECT obligations->0->>'state' FROM orp_deals WHERE id=$1`, deal2).Scan(&obState); err != nil || obState != "revoked" {
		t.Fatalf("obligation %s %v", obState, err)
	}
	revMissing := orpEnvelope("revoke", "m_trust_rev03", publisher, map[string]any{"scope": "obligation", "obligation": "nope", "reason": "other"})
	revMissing["deal"] = deal2
	missRec, _ := postJSON(t, r, http.MethodPost, "/orp", revMissing, bearerHeader(pubToken))
	if missRec.Code != http.StatusNotFound {
		t.Fatalf("missing obligation %d %s", missRec.Code, missRec.Body.String())
	}

	// scope=offer unlists a standing offer
	offerRec, offerBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_trust_stand1", maker, standingOffer()), bearerHeader(makerToken))
	if offerRec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", offerRec.Code, offerRec.Body.String())
	}
	offerID, _ := offerBody["body"].(map[string]any)["id"].(string)
	revOffer := orpEnvelope("revoke", "m_trust_rev04", maker, map[string]any{"scope": "offer", "offer": offerID, "reason": "expired"})
	revOfferRec, revOfferBody := postJSON(t, r, http.MethodPost, "/orp", revOffer, bearerHeader(makerToken))
	if revOfferRec.Code != http.StatusOK || revOfferBody["body"].(map[string]any)["listed"] != false {
		t.Fatalf("offer revoke %d %s", revOfferRec.Code, revOfferRec.Body.String())
	}
	var listed bool
	if err := entry.DB.Pool.QueryRow(context.Background(), `SELECT listed FROM orp_offers WHERE id=$1`, offerID).Scan(&listed); err != nil || listed {
		t.Fatalf("offer listed %v %v", listed, err)
	}
	foreign := orpEnvelope("revoke", "m_trust_rev05", publisher, map[string]any{"scope": "offer", "offer": offerID, "reason": "other"})
	foreignRec, _ := postJSON(t, r, http.MethodPost, "/orp", foreign, bearerHeader(pubToken))
	if foreignRec.Code != http.StatusNotFound {
		t.Fatalf("foreign offer revoke %d %s", foreignRec.Code, foreignRec.Body.String())
	}
}

func TestORPDispute(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)

	early := orpEnvelope("dispute", "m_trust_dsp00", maker, map[string]any{
		"action": "open", "obligation": "place", "claim": "not_done", "statement": "zq early dispute",
	})
	early["deal"] = dealID
	earlyRec, _ := postJSON(t, r, http.MethodPost, "/orp", early, bearerHeader(makerToken))
	if earlyRec.Code != http.StatusConflict {
		t.Fatalf("dispute on proposed %d %s", earlyRec.Code, earlyRec.Body.String())
	}

	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)
	resolve := orpEnvelope("dispute", "m_trust_dsp01", maker, map[string]any{"action": "resolve", "dispute": "d1"})
	resolve["deal"] = dealID
	resolveRec, _ := postJSON(t, r, http.MethodPost, "/orp", resolve, bearerHeader(makerToken))
	if resolveRec.Code != http.StatusForbidden {
		t.Fatalf("resolve %d %s", resolveRec.Code, resolveRec.Body.String())
	}

	open := orpEnvelope("dispute", "m_trust_dsp02", maker, map[string]any{
		"action": "open", "obligation": "place", "claim": "not_done",
		"statement": "zq placement never appeared",
	})
	open["deal"] = dealID
	openRec, openBody := postJSON(t, r, http.MethodPost, "/orp", open, bearerHeader(makerToken))
	if openRec.Code != http.StatusOK || openBody["body"].(map[string]any)["state"] != "disputed" {
		t.Fatalf("dispute open %d %s", openRec.Code, openRec.Body.String())
	}
	if dealState(t, entry.DB, dealID) != "disputed" {
		t.Fatal("disputed not stored")
	}

	respond := orpEnvelope("dispute", "m_trust_dsp03", publisher, map[string]any{"action": "respond", "dispute": "d1", "statement": "zq it is live"})
	respond["deal"] = dealID
	respRec, _ := postJSON(t, r, http.MethodPost, "/orp", respond, bearerHeader(pubToken))
	if respRec.Code != http.StatusOK {
		t.Fatalf("respond %d %s", respRec.Code, respRec.Body.String())
	}

	withdraw := orpEnvelope("dispute", "m_trust_dsp04", maker, map[string]any{"action": "withdraw", "dispute": "d1"})
	withdraw["deal"] = dealID
	wdRec, wdBody := postJSON(t, r, http.MethodPost, "/orp", withdraw, bearerHeader(makerToken))
	if wdRec.Code != http.StatusOK || wdBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("withdraw %d %s", wdRec.Code, wdRec.Body.String())
	}
	if dealState(t, entry.DB, dealID) != "active" {
		t.Fatal("withdraw did not restore active")
	}
}

func TestORPReputation(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	query := orpEnvelope("reputation", "m_trust_rep01", maker, map[string]any{"subject": publisher})
	rec, body := postJSON(t, r, http.MethodPost, "/orp", query, bearerHeader(makerToken))
	if rec.Code != http.StatusOK || body["type"] != "reputation" {
		t.Fatalf("reputation %d %s", rec.Code, rec.Body.String())
	}
	inner := body["body"].(map[string]any)
	if inner["subject"] != publisher || inner["algorithm"] != "orp-rep-1" {
		t.Fatalf("attestation %s", rec.Body.String())
	}
	if score, present := inner["score"]; !present || score != nil {
		t.Fatalf("score %v should be null under evidence weight 3", score)
	}
	if inner["valid_until"].(string) <= inner["computed_at"].(string) {
		t.Fatalf("validity %s", rec.Body.String())
	}
	if !strings.HasPrefix(body["from"].(string), "orp:") || body["to"] != maker || body["re"] != "m_trust_rep01" {
		t.Fatalf("envelope %s", rec.Body.String())
	}
}

func orpProblem(body map[string]any) (code, message string) {
	inner, _ := body["body"].(map[string]any)
	code, _ = inner["code"].(string)
	message, _ = inner["message"].(string)
	return code, message
}

func openDispute(t *testing.T, r *gin.Engine, token, party, dealID, id string) {
	t.Helper()
	open := orpEnvelope("dispute", id, party, map[string]any{
		"action": "open", "obligation": "place", "claim": "not_done",
		"statement": "zq placement never appeared",
	})
	open["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", open, bearerHeader(token))
	if rec.Code != http.StatusOK || body["body"].(map[string]any)["state"] != "disputed" {
		t.Fatalf("open %s %d %s", id, rec.Code, rec.Body.String())
	}
}

func withdrawDispute(r http.Handler, token, party, dealID, id string) (int, map[string]any) {
	msg := orpEnvelope("dispute", id, party, map[string]any{"action": "withdraw", "dispute": "d1"})
	msg["deal"] = dealID
	raw, _ := json.Marshal(msg)
	req := httptest.NewRequest(http.MethodPost, "/orp", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// TestORPDisputeWithdrawOpenerOnly pins SPEC §15.2: only the opener may withdraw.
// A respondent, a non-party, a spoofed from, and a history that does not name
// one opener are refused. The deal stays disputed.
func TestORPDisputeWithdrawOpenerOnly(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	thirdToken, third := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")

	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	earlyCode, earlyBody := withdrawDispute(r, pubToken, publisher, dealID, "m_trust_wd_early")
	if earlyCode != http.StatusConflict || orpCode(earlyBody) != "state_conflict" {
		t.Fatalf("withdraw before open %d %v", earlyCode, earlyBody)
	}
	if dealState(t, entry.DB, dealID) != "active" {
		t.Fatal("early withdraw changed the deal")
	}

	openDispute(t, r, makerToken, maker, dealID, "m_trust_wd_open")

	strangerCode, _ := withdrawDispute(r, thirdToken, third, dealID, "m_trust_wd_stranger")
	if strangerCode != http.StatusNotFound {
		t.Fatalf("non-party withdraw %d", strangerCode)
	}
	spoofCode, spoofBody := withdrawDispute(r, thirdToken, publisher, dealID, "m_trust_wd_spoof")
	if spoofCode != http.StatusForbidden || orpCode(spoofBody) != "forbidden" {
		t.Fatalf("spoofed withdraw %d %v", spoofCode, spoofBody)
	}
	respondentCode, respondentBody := withdrawDispute(r, pubToken, publisher, dealID, "m_trust_wd_respondent")
	if respondentCode != http.StatusForbidden || orpCode(respondentBody) != "forbidden" {
		t.Fatalf("respondent withdraw %d %v", respondentCode, respondentBody)
	}
	if dealState(t, entry.DB, dealID) != "disputed" {
		t.Fatal("refused withdraw changed the deal")
	}

	openerCode, openerBody := withdrawDispute(r, makerToken, maker, dealID, "m_trust_wd_opener")
	if openerCode != http.StatusOK || openerBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("opener withdraw %d %v", openerCode, openerBody)
	}
	if dealState(t, entry.DB, dealID) != "active" {
		t.Fatal("opener withdraw did not restore active")
	}

	// Older rows keep the opener only in the stored dispute message.
	// Two open messages, or none, do not name one opener: fail closed.
	ambiguous, ambiguousHash := directedDealID(t, r, makerToken, maker, publisher, "ambig")
	activateDealID(t, r, pubToken, publisher, ambiguous, ambiguousHash, "ambigacc")
	openDispute(t, r, makerToken, maker, ambiguous, "m_trust_ambig_open")
	plantDisputeOpen(t, entry.DB, ambiguous, publisher, "m_trust_ambig_other")
	ambigCode, ambigBody := withdrawDispute(r, makerToken, maker, ambiguous, "m_trust_ambig_wd")
	otherCode, otherBody := withdrawDispute(r, pubToken, publisher, ambiguous, "m_trust_ambig_wd2")
	if ambigCode != http.StatusConflict || otherCode != http.StatusConflict || orpMessage(ambigBody) == "Only the opener may withdraw." || orpMessage(otherBody) == "Only the opener may withdraw." {
		t.Fatalf("ambiguous withdraw maker %d %v publisher %d %v", ambigCode, ambigBody, otherCode, otherBody)
	}
	if dealState(t, entry.DB, ambiguous) != "disputed" {
		t.Fatal("ambiguous withdraw changed the deal")
	}

	missing, missingHash := directedDealID(t, r, makerToken, maker, publisher, "miss")
	activateDealID(t, r, pubToken, publisher, missing, missingHash, "missacc")
	openDispute(t, r, makerToken, maker, missing, "m_trust_miss_open")
	if _, err := entry.DB.Pool.Exec(context.Background(), `
DELETE FROM orp_inbox
WHERE COALESCE(message->>'deal', message->>'Deal')=$1
  AND COALESCE(message->>'type', message->>'Type')='dispute'`, missing); err != nil {
		t.Fatal(err)
	}
	missCode, missBody := withdrawDispute(r, makerToken, maker, missing, "m_trust_miss_wd")
	if missCode != http.StatusConflict || orpMessage(missBody) != "The host cannot tell who opened this dispute." {
		t.Fatalf("missing opener history %d %v", missCode, missBody)
	}
	if dealState(t, entry.DB, missing) != "disputed" {
		t.Fatal("unresolved opener withdrew the dispute")
	}
}

func orpCode(body map[string]any) string {
	code, _ := orpProblem(body)
	return code
}

func orpMessage(body map[string]any) string {
	_, message := orpProblem(body)
	return message
}

// plantDisputeOpen inserts a second stored open, the shape older inbox rows use.
func plantDisputeOpen(t *testing.T, db *database.DB, dealID, from, id string) {
	t.Helper()
	var raw []byte
	var owner string
	err := db.Pool.QueryRow(context.Background(), `
SELECT message, owner_id::text
FROM orp_inbox
WHERE COALESCE(message->>'deal', message->>'Deal')=$1
  AND COALESCE(message->>'type', message->>'Type')='dispute'
ORDER BY id
LIMIT 1`, dealID).Scan(&raw, &owner)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err = json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["From"]; ok {
		obj["From"] = from
		obj["ID"] = id
	} else {
		obj["from"] = from
		obj["id"] = id
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `INSERT INTO orp_inbox(owner_id, message) VALUES($1,$2)`, owner, encoded); err != nil {
		t.Fatal(err)
	}
}

func plantDisputeAction(t *testing.T, db *database.DB, dealID, from, id, action string) {
	t.Helper()
	plantDisputeOpen(t, db, dealID, from, id)
	var raw []byte
	var rowID int64
	err := db.Pool.QueryRow(context.Background(), `
SELECT id, message FROM orp_inbox
WHERE COALESCE(message->>'deal', message->>'Deal')=$1
  AND COALESCE(message->>'id', message->>'ID')=$2
ORDER BY id DESC LIMIT 1`, dealID, id).Scan(&rowID, &raw)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err = json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Body", "body"} {
		body, _ := obj[key].(map[string]any)
		if body == nil {
			continue
		}
		body["action"] = action
		obj[key] = body
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE orp_inbox SET message=$2 WHERE id=$1`, rowID, encoded); err != nil {
		t.Fatal(err)
	}
}

// TestORPObligationRevokeObligor pins SPEC §15.1. Only the obligor may revoke
// an obligation, only while the deal is active, and only while that obligation
// is still pending or claimed. Verified work stays verified. A write that loses
// the race does not apply.
func TestORPObligationRevokeObligor(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	thirdToken, third := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")

	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	wrong := orpEnvelope("revoke", "m_trust_ob_wrong", maker, map[string]any{"scope": "obligation", "obligation": "place", "reason": "owner_request"})
	wrong["deal"] = dealID
	wrongRec, wrongBody := postJSON(t, r, http.MethodPost, "/orp", wrong, bearerHeader(makerToken))
	if wrongRec.Code != http.StatusForbidden || orpCode(wrongBody) != "forbidden" {
		t.Fatalf("wrong obligor %d %s", wrongRec.Code, wrongRec.Body.String())
	}
	if obligationState(t, entry.DB, dealID, "place") != "pending" {
		t.Fatal("wrong obligor revoked the obligation")
	}

	stranger := orpEnvelope("revoke", "m_trust_ob_stranger", third, map[string]any{"scope": "obligation", "obligation": "place", "reason": "other"})
	stranger["deal"] = dealID
	strangerRec, _ := postJSON(t, r, http.MethodPost, "/orp", stranger, bearerHeader(thirdToken))
	if strangerRec.Code != http.StatusNotFound {
		t.Fatalf("non-party revoke %d %s", strangerRec.Code, strangerRec.Body.String())
	}
	spoof := orpEnvelope("revoke", "m_trust_ob_spoof", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "other"})
	spoof["deal"] = dealID
	spoofRec, spoofBody := postJSON(t, r, http.MethodPost, "/orp", spoof, bearerHeader(thirdToken))
	if spoofRec.Code != http.StatusForbidden || orpCode(spoofBody) != "forbidden" {
		t.Fatalf("spoofed revoke %d %s", spoofRec.Code, spoofRec.Body.String())
	}
	if obligationState(t, entry.DB, dealID, "place") != "pending" || dealState(t, entry.DB, dealID) != "active" {
		t.Fatal("refused revoke changed the deal")
	}

	declinedID, _ := directedDealID(t, r, makerToken, maker, publisher, "decl")
	decline := orpEnvelope("decline", "m_trust_ob_dec", publisher, map[string]any{"reason": "not_relevant"})
	decline["deal"] = declinedID
	declineRec, _ := postJSON(t, r, http.MethodPost, "/orp", decline, bearerHeader(pubToken))
	if declineRec.Code != http.StatusOK {
		t.Fatalf("decline %d %s", declineRec.Code, declineRec.Body.String())
	}
	declRevoke := orpEnvelope("revoke", "m_trust_ob_decl", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "other"})
	declRevoke["deal"] = declinedID
	declRec, declBody := postJSON(t, r, http.MethodPost, "/orp", declRevoke, bearerHeader(pubToken))
	if declRec.Code != http.StatusConflict || orpCode(declBody) != "state_conflict" {
		t.Fatalf("declined revoke %d %s", declRec.Code, declRec.Body.String())
	}
	if dealState(t, entry.DB, declinedID) != "declined" || obligationState(t, entry.DB, declinedID, "place") != "pending" {
		t.Fatal("declined deal obligation changed")
	}

	verifiedID, verifiedHash := directedDealID(t, r, makerToken, maker, publisher, "ver")
	activateDealID(t, r, pubToken, publisher, verifiedID, verifiedHash, "veracc")
	if _, err := entry.DB.Pool.Exec(context.Background(), `
UPDATE orp_deals
SET obligations = jsonb_set(obligations, '{0,state}', '"verified"')
WHERE id=$1`, verifiedID); err != nil {
		t.Fatal(err)
	}
	verRevoke := orpEnvelope("revoke", "m_trust_ob_ver", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "breach"})
	verRevoke["deal"] = verifiedID
	verRec, verBody := postJSON(t, r, http.MethodPost, "/orp", verRevoke, bearerHeader(pubToken))
	if verRec.Code != http.StatusConflict || orpCode(verBody) != "state_conflict" {
		t.Fatalf("verified revoke %d %s", verRec.Code, verRec.Body.String())
	}
	if obligationState(t, entry.DB, verifiedID, "place") != "verified" {
		t.Fatal("verified obligation was revoked")
	}

	claimedID, claimedHash := directedDealID(t, r, makerToken, maker, publisher, "claim")
	activateDealID(t, r, pubToken, publisher, claimedID, claimedHash, "claimacc")
	exec := orpEnvelope("execute", "m_trust_ob_exec", publisher, map[string]any{"obligation": "place"})
	exec["deal"] = claimedID
	execRec, _ := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(pubToken))
	if execRec.Code != http.StatusOK || obligationState(t, entry.DB, claimedID, "place") != "claimed" {
		t.Fatalf("claim %d %s", execRec.Code, execRec.Body.String())
	}
	claimRevoke := orpEnvelope("revoke", "m_trust_ob_claim", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "brand_safety"})
	claimRevoke["deal"] = claimedID
	claimRec, _ := postJSON(t, r, http.MethodPost, "/orp", claimRevoke, bearerHeader(pubToken))
	if claimRec.Code != http.StatusOK || obligationState(t, entry.DB, claimedID, "place") != "revoked" {
		t.Fatalf("claimed revoke %d %s state %s", claimRec.Code, claimRec.Body.String(), obligationState(t, entry.DB, claimedID, "place"))
	}
	if dealState(t, entry.DB, claimedID) != "active" {
		t.Fatal("obligation revoke changed the deal state")
	}

	raceID, raceHash := directedDealID(t, r, makerToken, maker, publisher, "race")
	activateDealID(t, r, pubToken, publisher, raceID, raceHash, "raceacc")
	entry.DB.ORPBeforeEffect = func(ctx context.Context) error {
		_, err := entry.DB.Pool.Exec(ctx, `
UPDATE orp_deals
SET obligations = jsonb_set(obligations, '{0,state}', '"verified"')
WHERE id=$1`, raceID)
		return err
	}
	raceRevoke := orpEnvelope("revoke", "m_trust_ob_race", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "other"})
	raceRevoke["deal"] = raceID
	raceRec, _ := postJSON(t, r, http.MethodPost, "/orp", raceRevoke, bearerHeader(pubToken))
	entry.DB.ORPBeforeEffect = nil
	if raceRec.Code != http.StatusConflict {
		t.Fatalf("stale revoke %d %s", raceRec.Code, raceRec.Body.String())
	}
	if obligationState(t, entry.DB, raceID, "place") != "verified" {
		t.Fatal("stale revoke overwrote a verified obligation")
	}

	concID, concHash := directedDealID(t, r, makerToken, maker, publisher, "conc")
	activateDealID(t, r, pubToken, publisher, concID, concHash, "concacc")
	var wg sync.WaitGroup
	codes := make([]int, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			codes[i], _ = postRevoke(r, pubToken, publisher, concID, "m_trust_ob_conc_"+string(rune('a'+i)))
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 1 || conflict != 1 || obligationState(t, entry.DB, concID, "place") != "revoked" {
		t.Fatalf("concurrent revoke codes %v state %s", codes, obligationState(t, entry.DB, concID, "place"))
	}
}

func TestORPDisputeIDCollisionAndLateRespond(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")

	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)
	openDispute(t, r, makerToken, maker, dealID, "m_trust_sameid")
	respond := orpEnvelope("dispute", "m_trust_sameid", publisher, map[string]any{
		"action": "respond", "dispute": "d1", "statement": "zq same id from the other party",
	})
	respond["deal"] = dealID
	respRec, _ := postJSON(t, r, http.MethodPost, "/orp", respond, bearerHeader(pubToken))
	if respRec.Code != http.StatusOK {
		t.Fatalf("same-id respond %d %s", respRec.Code, respRec.Body.String())
	}
	code, body := withdrawDispute(r, makerToken, maker, dealID, "m_trust_sameid_wd")
	if code != http.StatusOK || body["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("withdraw after id collision %d %v", code, body)
	}

	lateID, lateHash := directedDealID(t, r, makerToken, maker, publisher, "late")
	activateDealID(t, r, pubToken, publisher, lateID, lateHash, "lateacc")
	openDispute(t, r, makerToken, maker, lateID, "m_trust_late_open")
	if wd, _ := withdrawDispute(r, makerToken, maker, lateID, "m_trust_late_wd"); wd != http.StatusOK {
		t.Fatalf("first withdraw %d", wd)
	}
	plantDisputeAction(t, entry.DB, lateID, publisher, "m_trust_late_respond", "respond")
	openDispute(t, r, makerToken, maker, lateID, "m_trust_late_open2")
	again, againBody := withdrawDispute(r, makerToken, maker, lateID, "m_trust_late_wd2")
	if again != http.StatusOK || dealState(t, entry.DB, lateID) != "active" {
		t.Fatalf("withdraw after late respond %d %v", again, againBody)
	}
}

func TestORPDisputeOpenerWithoutPeerOwner(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)
	parties := map[string]any{"seats": []any{
		map[string]any{"id": maker, "role": "offerer", "required": true},
		map[string]any{"id": publisher, "role": "acceptor", "required": true},
	}}
	rawParties, err := json.Marshal(parties)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.DB.Pool.Exec(context.Background(), `
UPDATE orp_deals SET offerer_owner = NULL, parties = $2 WHERE id = $1`, dealID, rawParties); err != nil {
		t.Fatal(err)
	}
	openDispute(t, r, pubToken, publisher, dealID, "m_trust_nolocal_open")
	var stored int
	if err = entry.DB.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM orp_inbox
WHERE COALESCE(message->>'deal', message->>'Deal')=$1
  AND COALESCE(message->>'type', message->>'Type')='dispute'
  AND COALESCE(message->'body'->>'action', message->'Body'->>'action')='open'`, dealID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored < 1 {
		t.Fatal("ownerless peer stored no opener record")
	}
	code, body := withdrawDispute(r, pubToken, publisher, dealID, "m_trust_nolocal_wd")
	if code != http.StatusOK || dealState(t, entry.DB, dealID) != "active" {
		t.Fatalf("opener withdraw without peer owner %d %v", code, body)
	}

	roleID, roleHash := directedDealID(t, r, makerToken, maker, publisher, "role")
	activateDealID(t, r, pubToken, publisher, roleID, roleHash, "roleacc")
	if _, err = entry.DB.Pool.Exec(context.Background(), `
UPDATE orp_deals SET offerer_owner = NULL, parties = $2 WHERE id = $1`, roleID, rawParties); err != nil {
		t.Fatal(err)
	}
	makerRevoke := orpEnvelope("revoke", "m_trust_role_maker", maker, map[string]any{"scope": "obligation", "obligation": "place", "reason": "breach"})
	makerRevoke["deal"] = roleID
	makerRec, makerBody := postJSON(t, r, http.MethodPost, "/orp", makerRevoke, bearerHeader(makerToken))
	if makerRec.Code != http.StatusForbidden || orpCode(makerBody) != "forbidden" || obligationState(t, entry.DB, roleID, "place") != "pending" {
		t.Fatalf("counterparty revoke on many-party deal %d %s", makerRec.Code, makerRec.Body.String())
	}
	pubRevoke := orpEnvelope("revoke", "m_trust_role_pub", publisher, map[string]any{"scope": "obligation", "obligation": "place", "reason": "owner_request"})
	pubRevoke["deal"] = roleID
	pubRec, _ := postJSON(t, r, http.MethodPost, "/orp", pubRevoke, bearerHeader(pubToken))
	if pubRec.Code != http.StatusOK || obligationState(t, entry.DB, roleID, "place") != "revoked" {
		t.Fatalf("obligor revoke on many-party deal %d %s", pubRec.Code, pubRec.Body.String())
	}
}

func TestORPExecuteKeepsTerminalObligations(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := directedDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)
	if _, err := entry.DB.Pool.Exec(context.Background(), `
UPDATE orp_deals
SET obligations = '[{"id":"place","state":"pending"},{"id":"other","state":"pending"}]'::jsonb
WHERE id=$1`, dealID); err != nil {
		t.Fatal(err)
	}
	entry.DB.ORPBeforeEffect = func(ctx context.Context) error {
		_, err := entry.DB.Pool.Exec(ctx, `
UPDATE orp_deals
SET obligations = jsonb_set(obligations, '{1,state}', '"verified"'),
    updated_at = updated_at + interval '1 millisecond'
WHERE id=$1`, dealID)
		return err
	}
	exec := orpEnvelope("execute", "m_trust_exec_stale", publisher, map[string]any{"obligation": "place"})
	exec["deal"] = dealID
	rec, _ := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(pubToken))
	entry.DB.ORPBeforeEffect = nil
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale execute %d %s", rec.Code, rec.Body.String())
	}
	if obligationState(t, entry.DB, dealID, "place") != "pending" || obligationState(t, entry.DB, dealID, "other") != "verified" {
		t.Fatalf("stale execute rewrote obligations place=%s other=%s", obligationState(t, entry.DB, dealID, "place"), obligationState(t, entry.DB, dealID, "other"))
	}

	revokedID, revokedHash := directedDealID(t, r, makerToken, maker, publisher, "exrev")
	activateDealID(t, r, pubToken, publisher, revokedID, revokedHash, "exrevacc")
	entry.DB.ORPBeforeEffect = func(ctx context.Context) error {
		_, err := entry.DB.Pool.Exec(ctx, `
UPDATE orp_deals
SET obligations = jsonb_set(obligations, '{0,state}', '"revoked"'),
    updated_at = updated_at + interval '1 millisecond'
WHERE id=$1`, revokedID)
		return err
	}
	execRev := orpEnvelope("execute", "m_trust_exec_rev", publisher, map[string]any{"obligation": "place"})
	execRev["deal"] = revokedID
	revRec, _ := postJSON(t, r, http.MethodPost, "/orp", execRev, bearerHeader(pubToken))
	entry.DB.ORPBeforeEffect = nil
	if revRec.Code != http.StatusConflict || obligationState(t, entry.DB, revokedID, "place") != "revoked" {
		t.Fatalf("execute over revoked %d state %s", revRec.Code, obligationState(t, entry.DB, revokedID, "place"))
	}
}

func postRevoke(r http.Handler, token, party, dealID, id string) (int, map[string]any) {
	msg := orpEnvelope("revoke", id, party, map[string]any{"scope": "obligation", "obligation": "place", "reason": "owner_request"})
	msg["deal"] = dealID
	raw, _ := json.Marshal(msg)
	req := httptest.NewRequest(http.MethodPost, "/orp", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}
