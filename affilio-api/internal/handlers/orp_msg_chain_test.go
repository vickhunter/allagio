package handlers

import (
	"net/http"
	"reflect"
	"sync"
	"testing"

	"affilio-api/internal/orp"
)

func chainHash(env map[string]any) string {
	return orp.ContentHash(env)
}

func chainCounter(t *testing.T, r http.Handler, token, from, dealID, suffix, prev string) (int, map[string]any, map[string]any) {
	t.Helper()
	msg := orpEnvelope("counter", "m_chain_"+suffix, from, map[string]any{"terms": counterTerms()})
	msg["deal"] = dealID
	if prev != "" {
		msg["prev"] = prev
	}
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(token))
	return rec.Code, body, msg
}

func TestORPChainSequenceVerifies(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_seq")

	code, body, first := chainCounter(t, r, pubToken, publisher, dealID, "c01", "")
	if code != http.StatusOK {
		t.Fatalf("first counter %d %s", code, body)
	}
	var second map[string]any
	code, body, second = chainCounter(t, r, pubToken, publisher, dealID, "c02", chainHash(first))
	if code != http.StatusOK {
		t.Fatalf("second counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, dealID, "c03", chainHash(second))
	if code != http.StatusOK {
		t.Fatalf("third counter %d %s", code, body)
	}
}

func TestORPChainWrongPrev(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_wrong")

	code, body, _ := chainCounter(t, r, pubToken, publisher, dealID, "w01", "")
	if code != http.StatusOK {
		t.Fatalf("first counter %d %s", code, body)
	}
	other := orpEnvelope("counter", "m_chain_unrelated", publisher, map[string]any{"terms": counterTerms()})
	code, body, _ = chainCounter(t, r, pubToken, publisher, dealID, "w02", chainHash(other))
	if code != http.StatusConflict || body["body"].(map[string]any)["code"] != "chain_broken" {
		t.Fatalf("wrong prev %d %s", code, body)
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/error.schema.json"), body); err != nil {
		t.Fatalf("error doc fails the schema: %v", err)
	}
}

func TestORPChainOmittedPrevAfterUse(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_omit")

	code, body, first := chainCounter(t, r, pubToken, publisher, dealID, "o01", "")
	if code != http.StatusOK {
		t.Fatalf("first counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, dealID, "o02", chainHash(first))
	if code != http.StatusOK {
		t.Fatalf("second counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, dealID, "o03", "")
	if code != http.StatusConflict || body["body"].(map[string]any)["code"] != "chain_broken" {
		t.Fatalf("omitted prev %d %s", code, body)
	}
}

func TestORPChainPrevOnFirstMessage(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_first")

	some := orpEnvelope("counter", "m_chain_someone", publisher, map[string]any{"terms": counterTerms()})
	code, body, _ := chainCounter(t, r, pubToken, publisher, dealID, "f01", chainHash(some))
	if code != http.StatusConflict || body["body"].(map[string]any)["code"] != "chain_broken" {
		t.Fatalf("prev on first message %d %s", code, body)
	}
}

func TestORPChainNeverChainedSenderUnaffected(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_free")

	code, body, first := chainCounter(t, r, pubToken, publisher, dealID, "n01", "")
	if code != http.StatusOK {
		t.Fatalf("first counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, dealID, "n02", chainHash(first))
	if code != http.StatusOK {
		t.Fatalf("second counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, makerToken, maker, dealID, "n03", "")
	if code != http.StatusOK {
		t.Fatalf("unchained counterparty %d %s", code, body)
	}
}

func TestORPChainReplayReturnsStoredReply(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_replay")

	code, body, first := chainCounter(t, r, pubToken, publisher, dealID, "r01", "")
	if code != http.StatusOK {
		t.Fatalf("first counter %d %s", code, body)
	}
	msg := orpEnvelope("counter", "m_chain_r02", publisher, map[string]any{"terms": counterTerms()})
	msg["deal"] = dealID
	msg["prev"] = chainHash(first)
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("chained counter %d %s", rec.Code, body)
	}
	again, againBody := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	if again.Code != http.StatusOK || !reflect.DeepEqual(againBody, body) {
		t.Fatalf("replay %d %s want stored reply", again.Code, again.Body.String())
	}
}

func TestORPChainIndependentDeals(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	deal1, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_a")
	deal2, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_b")

	code, body, first := chainCounter(t, r, pubToken, publisher, deal1, "d01", "")
	if code != http.StatusOK {
		t.Fatalf("deal1 counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, deal1, "d02", chainHash(first))
	if code != http.StatusOK {
		t.Fatalf("deal1 chained counter %d %s", code, body)
	}
	code, body, _ = chainCounter(t, r, pubToken, publisher, deal2, "d03", "")
	if code != http.StatusOK {
		t.Fatalf("deal2 fresh counter %d %s", code, body)
	}
}

func TestORPChainFirstMessageRace(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	dealID, _ := directedDealID(t, r, makerToken, maker, publisher, "chain_race")

	// Two first messages in flight at once: a plain counter and one that
	// chains onto it. Without the ON CONFLICT insert the losing transaction
	// surfaced a unique-violation 500; the loser must be re-evaluated
	// against the winner's stored hash instead.
	msgA := orpEnvelope("counter", "m_chain_race_a", publisher, map[string]any{"terms": counterTerms()})
	msgA["deal"] = dealID
	msgB := orpEnvelope("counter", "m_chain_race_b", publisher, map[string]any{"terms": counterTerms()})
	msgB["deal"] = dealID
	msgB["prev"] = chainHash(msgA)

	start := make(chan struct{})
	var wg sync.WaitGroup
	var codeA, codeB int
	var bodyB map[string]any
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		rec, _ := postJSON(t, r, http.MethodPost, "/orp", msgA, bearerHeader(pubToken))
		codeA = rec.Code
	}()
	go func() {
		defer wg.Done()
		<-start
		rec, body := postJSON(t, r, http.MethodPost, "/orp", msgB, bearerHeader(pubToken))
		codeB = rec.Code
		bodyB = body
	}()
	close(start)
	wg.Wait()

	if codeA >= 500 || codeB >= 500 {
		t.Fatalf("race surfaced a server error: plain=%d chained=%d", codeA, codeB)
	}
	if codeA != http.StatusOK {
		t.Fatalf("plain first message failed: %d", codeA)
	}
	switch codeB {
	case http.StatusOK:
		// The plain message committed first and the chained prev matched it.
	case http.StatusConflict:
		inner, _ := bodyB["body"].(map[string]any)
		if inner["code"] != "chain_broken" {
			t.Fatalf("chained loser got %s, want chain_broken", bodyB)
		}
	default:
		t.Fatalf("chained message got unexpected status %d", codeB)
	}
}
