package handlers

import (
	"context"
	"net/http"
	"strings"
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
