package handlers

import (
	"context"
	"net/http"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

func counterTerms() map[string]any {
	terms := testTerms()
	terms["title"] = "countered"
	return terms
}

func counterDeal(t *testing.T, r http.Handler, makerToken, maker, publisher string) (dealID, termsHash string) {
	t.Helper()
	offer := orpEnvelope("offer", "m_ctr_offer", maker, map[string]any{"terms": testTerms()})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	inner, _ := body["body"].(map[string]any)
	if rec.Code != http.StatusOK || inner["state"] != "proposed" {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ = body["deal"].(string)
	termsHash, _ = inner["terms_hash"].(string)
	return dealID, termsHash
}

func sendCounter(t *testing.T, r http.Handler, token, from, dealID, suffix string, terms map[string]any) (int, map[string]any) {
	t.Helper()
	msg := orpEnvelope("counter", "m_ctr_"+suffix, from, map[string]any{"terms": terms})
	msg["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(token))
	return rec.Code, body
}

func acceptDealWith(t *testing.T, r http.Handler, token, from, dealID, termsHash, suffix string) (int, map[string]any) {
	t.Helper()
	msg := orpEnvelope("accept", "m_acc_"+suffix, from, map[string]any{"terms_hash": termsHash})
	msg["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(token))
	return rec.Code, body
}

func dealRow(t *testing.T, db *database.DB, dealID string) (state, termsHash string, waiting map[string]any) {
	t.Helper()
	var raw []byte
	err := db.Pool.QueryRow(context.Background(), `SELECT state, terms_hash, COALESCE(waiting,'null') FROM orp_deals WHERE id=$1`, dealID).Scan(&state, &termsHash, &raw)
	if err != nil {
		t.Fatal(err)
	}
	waiting, _ = decodeObject(raw)
	return state, termsHash, waiting
}

func TestORPCounterReplacesTerms(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, oldHash := counterDeal(t, r, makerToken, maker, publisher)

	newTerms := counterTerms()
	newHash := mustTermsHash(t, newTerms)
	code, body := sendCounter(t, r, pubToken, publisher, dealID, "c01", newTerms)
	inner, _ := body["body"].(map[string]any)
	if code != http.StatusOK || inner["state"] != "proposed" {
		t.Fatalf("counter %d %s", code, body)
	}
	state, storedHash, waiting := dealRow(t, entry.DB, dealID)
	if state != "proposed" || storedHash != newHash {
		t.Fatalf("deal row state=%s hash=%s", state, storedHash)
	}
	if waiting["party"] != maker {
		t.Fatalf("waiting did not flip to offerer %v", waiting["party"])
	}

	// A stale accept of the old terms is refused.
	code, _ = acceptDealWith(t, r, makerToken, maker, dealID, oldHash, "stale")
	if code != http.StatusConflict {
		t.Fatalf("stale accept %d", code)
	}

	// The offerer accepts the countered terms; the deal awaits activation.
	code, body = acceptDealWith(t, r, makerToken, maker, dealID, newHash, "ok")
	inner, _ = body["body"].(map[string]any)
	if code != http.StatusOK || inner["state"] != "awaiting" {
		t.Fatalf("accept countered terms %d %s", code, body)
	}
}

func TestORPCounterWrongState(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID, termsHash := counterDeal(t, r, makerToken, maker, publisher)
	activateDeal(t, entry.DB, r, pubToken, publisher, dealID, termsHash)

	code, _ := sendCounter(t, r, pubToken, publisher, dealID, "late", counterTerms())
	if code != http.StatusConflict {
		t.Fatalf("counter on awaiting/active %d", code)
	}
}

func mustTermsHash(t *testing.T, terms map[string]any) string {
	t.Helper()
	return orp.TermsHash(terms)
}
