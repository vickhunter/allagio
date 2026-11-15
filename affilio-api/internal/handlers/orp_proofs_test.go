package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

// proofHost mounts a host whose proof page fetch is the stub.
func proofHost(t *testing.T, fetch proofFetchFunc) (entry *AgentEntry, r *gin.Engine) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	entry = &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry, FetchProof: fetch}
	return entry, mountORP(entry, host)
}

func staticFetch(body string, status int, ok bool) proofFetchFunc {
	return func(ctx context.Context, rawURL string) (string, int, bool) {
		return body, status, ok
	}
}

// offerTerms posts a directed offer for terms and activates it.
func offerTerms(t *testing.T, r *gin.Engine, makerToken, maker, publisher, pubToken, suffix string, terms map[string]any) string {
	t.Helper()
	offer := orpEnvelope("offer", "m_proof_"+suffix, maker, map[string]any{"terms": terms})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)
	activateDealID(t, r, pubToken, publisher, dealID, termsHash, "acc_"+suffix)
	return dealID
}

// obligationRow reads one obligation row off the stored deal.
func obligationRow(t *testing.T, db *database.DB, dealID, id string) map[string]any {
	t.Helper()
	var raw []byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT obligations FROM orp_deals WHERE id=$1`, dealID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row["id"] == id {
			return row
		}
	}
	t.Fatalf("no obligation %s in %s", id, raw)
	return nil
}

// executeProof posts an execute carrying proof_url and returns the response.
func executeProof(t *testing.T, r *gin.Engine, token, party, dealID, obligation, proofURL, suffix string) (*int, map[string]any) {
	t.Helper()
	exec := orpEnvelope("execute", "m_proof_"+suffix, party, map[string]any{"obligation": obligation, "proof_url": proofURL})
	exec["deal"] = dealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(token))
	code := rec.Code
	return &code, body
}

// inboxReceipts returns the receipt messages a party's inbox holds.
func inboxReceipts(t *testing.T, r *gin.Engine, token string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, item := range inboxMessages(t, r, token) {
		msg, _ := item.(map[string]any)
		if msg["type"] == "receipt" {
			out = append(out, msg)
		}
	}
	return out
}

func lastReceipt(t *testing.T, r *gin.Engine, token string) map[string]any {
	t.Helper()
	receipts := inboxReceipts(t, r, token)
	if len(receipts) == 0 {
		t.Fatal("no receipt in inbox")
	}
	receipt := receipts[len(receipts)-1]
	var value any
	raw, _ := json.Marshal(receipt)
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if _, problem, ok := orp.ParseEnvelope(value); !ok {
		t.Fatalf("receipt fails the envelope: %s", problem.Message)
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/receipt.schema.json"), receipt); err != nil {
		t.Fatalf("receipt fails the schema: %v", err)
	}
	return receipt
}

func directTerms() map[string]any {
	return map[string]any{
		"title":      "zq proof link swap",
		"kind":       "link_exchange",
		"disclosure": map[string]any{"label": "Sponsored link", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id":          "place_a",
				"action":      "place",
				"by":          "acceptor",
				"content":     map[string]any{"direct": true},
				"destination": "https://maker.example/tool",
				"proof":       []any{"proof_url"},
			},
			map[string]any{
				"id":          "place_b",
				"action":      "place",
				"by":          "offerer",
				"content":     map[string]any{"direct": true},
				"destination": "https://pub.example/post",
				"proof":       []any{"proof_url"},
			},
		},
	}
}

func TestORPProofURLDirect(t *testing.T) {
	pages := map[string]string{
		"https://pub.example/post": `<html><body><p>Sponsored link</p><a href="https://maker.example/tool" rel="sponsored">the tool</a></body></html>`,
		"https://maker.example/x":  `<html><body><p>Sponsored link</p><a href="https://pub.example/post" rel="nofollow">the post</a></body></html>`,
	}
	entry, r := proofHost(t, func(ctx context.Context, rawURL string) (string, int, bool) {
		page, found := pages[rawURL]
		return page, 200, found
	})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")
	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "dir1", directTerms())

	code, _ := executeProof(t, r, pubToken, publisher, dealID, "place_a", "https://pub.example/post", "ex_a1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	row := obligationRow(t, entry.DB, dealID, "place_a")
	if row["state"] != "verified" || row["method"] != "proof_url" {
		t.Fatalf("place_a %v", row)
	}

	code, _ = executeProof(t, r, makerToken, maker, dealID, "place_b", "https://maker.example/x", "ex_b1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	if row := obligationRow(t, entry.DB, dealID, "place_b"); row["state"] != "verified" {
		t.Fatalf("place_b %v", row)
	}

	receipt := lastReceipt(t, r, makerToken)
	body, _ := receipt["body"].(map[string]any)
	if body["result"] != "verified" || body["method"] != "proof_url" {
		t.Fatalf("receipt %v", body)
	}
	found, _ := body["found"].(map[string]any)
	if found["destination"] != true || found["rel"] != true || found["disclosure"] != true {
		t.Fatalf("found %v", found)
	}
}

func TestORPProofURLDirectMissing(t *testing.T) {
	pages := map[string]string{
		// Anchor without sponsored/nofollow rel.
		"https://pub.example/norel": `<html><body>Sponsored link <a href="https://maker.example/tool">tool</a></body></html>`,
		// Right rel, no disclosure text.
		"https://pub.example/nodisc": `<html><body><a href="https://maker.example/tool" rel="sponsored">tool</a></body></html>`,
	}
	entry, r := proofHost(t, func(ctx context.Context, rawURL string) (string, int, bool) {
		page, found := pages[rawURL]
		return page, 200, found
	})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")

	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "dir2", directTerms())
	code, _ := executeProof(t, r, pubToken, publisher, dealID, "place_a", "https://pub.example/norel", "ex_c1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	if row := obligationRow(t, entry.DB, dealID, "place_a"); row["state"] != "failed" {
		t.Fatalf("place_a %v", row)
	}
	receipt := lastReceipt(t, r, pubToken)
	if body, _ := receipt["body"].(map[string]any); body["reason"] != "rel_missing" {
		t.Fatalf("reason %v", body)
	}

	// A second deal covers disclosure_missing.
	deal2 := offerTerms(t, r, makerToken, maker, publisher, pubToken, "dir3", directTerms())
	code, _ = executeProof(t, r, pubToken, publisher, deal2, "place_a", "https://pub.example/nodisc", "ex_c2")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	if row := obligationRow(t, entry.DB, deal2, "place_a"); row["state"] != "failed" {
		t.Fatalf("place_a %v", row)
	}
	receipt = lastReceipt(t, r, pubToken)
	if body, _ := receipt["body"].(map[string]any); body["reason"] != "disclosure_missing" {
		t.Fatalf("reason %v", body)
	}
}

func TestORPPlatformCheckRepost(t *testing.T) {
	terms := map[string]any{
		"title":      "zq proof repost",
		"kind":       "repost",
		"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
		"obligations": []any{map[string]any{
			"id":     "share",
			"action": "repost",
			"by":     "acceptor",
			"target": "https://x.com/maker/status/1111111111111111111",
			"proof":  []any{"platform_check", "proof_url"},
		}},
	}
	embed := `{"html":"<blockquote><a href=\"https://x.com/maker/status/1111111111111111111\">reposted</a></blockquote>"}`
	var page string
	entry, r := proofHost(t, func(ctx context.Context, rawURL string) (string, int, bool) {
		if strings.HasPrefix(rawURL, "https://publish.twitter.com/oembed") {
			return page, 200, true
		}
		return "", 0, false
	})
	page = embed
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")

	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "rep1", terms)
	code, _ := executeProof(t, r, pubToken, publisher, dealID, "share", "https://x.com/pub/status/2222222222222222222", "ex_r1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	if row := obligationRow(t, entry.DB, dealID, "share"); row["state"] != "verified" || row["method"] != "platform_check" {
		t.Fatalf("share %v", row)
	}
	receipt := lastReceipt(t, r, pubToken)
	if body, _ := receipt["body"].(map[string]any); body["method"] != "platform_check" {
		t.Fatalf("method %v", body)
	}

	// A repost whose embed points at a different status fails.
	page = `{"html":"<blockquote><a href=\"https://x.com/other/status/9999999999999999999\">nope</a></blockquote>"}`
	deal2 := offerTerms(t, r, makerToken, maker, publisher, pubToken, "rep2", terms)
	code, _ = executeProof(t, r, pubToken, publisher, deal2, "share", "https://x.com/pub/status/3333333333333333333", "ex_r2")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	if row := obligationRow(t, entry.DB, deal2, "share"); row["state"] != "failed" {
		t.Fatalf("share %v", row)
	}
	receipt = lastReceipt(t, r, pubToken)
	if body, _ := receipt["body"].(map[string]any); body["reason"] != "not_found" {
		t.Fatalf("reason %v", body)
	}
}

func TestORPProofURLComment(t *testing.T) {
	terms := map[string]any{
		"title":      "zq proof comment",
		"kind":       "comment",
		"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
		"obligations": []any{map[string]any{
			"id":      "say",
			"action":  "comment",
			"by":      "acceptor",
			"target":  "https://blog.example/post",
			"content": map[string]any{"copy": "Great notes on shipping fast"},
			"proof":   []any{"proof_url"},
		}},
	}
	entry, r := proofHost(t, staticFetch(
		`<html><body><div class="comment">great   notes on SHIPPING fast!!</div></body></html>`, 200, true))
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")

	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "com1", terms)
	code, _ := executeProof(t, r, pubToken, publisher, dealID, "say", "https://blog.example/post#comments", "ex_cm1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	row := obligationRow(t, entry.DB, dealID, "say")
	if row["state"] != "verified" {
		t.Fatalf("say %v", row)
	}
	receipt := lastReceipt(t, r, pubToken)
	if found, _ := receipt["body"].(map[string]any)["found"].(map[string]any); found["content"] != true {
		t.Fatalf("found %v", found)
	}
}

func TestORPProofUnreachable(t *testing.T) {
	terms := testTerms() // place, proof [proof_url], content link
	entry, r := proofHost(t, staticFetch("", 0, false))
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")

	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "unr1", terms)
	code, _ := executeProof(t, r, pubToken, publisher, dealID, "place", "https://gone.example/post", "ex_u1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	row := obligationRow(t, entry.DB, dealID, "place")
	if row["state"] != "claimed" {
		t.Fatalf("place %v", row)
	}
	receipt := lastReceipt(t, r, pubToken)
	body, _ := receipt["body"].(map[string]any)
	if body["result"] != "failed" || body["reason"] != "unreachable" {
		t.Fatalf("receipt %v", body)
	}
}

func attestTerms() map[string]any {
	return map[string]any{
		"title":      "zq proof attested",
		"kind":       "sponsored",
		"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id":      "place",
				"action":  "place",
				"by":      "acceptor",
				"content": map[string]any{"link": true},
				"proof":   []any{"attestation"},
			},
			map[string]any{
				"id":     "pay",
				"action": "pay",
				"by":     "offerer",
				"per":    "placement",
				"after":  "place",
				"amount": map[string]any{"amount_minor": 1500, "currency": "EUR"},
			},
		},
	}
}

func attestation(party, id, obligation, result, method string) map[string]any {
	msg := orpEnvelope("receipt", id, party, map[string]any{
		"obligation": obligation,
		"result":     result,
		"method":     method,
		"at":         time.Now().UTC().Format(time.RFC3339),
	})
	return msg
}

func TestORPAttestation(t *testing.T) {
	entry, r := proofHost(t, staticFetch("", 0, false))
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")
	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "att1", attestTerms())

	// P2: the obligor cannot attest its own obligation.
	self := attestation(publisher, "m_rcpt_self1", "place", "verified", "attestation")
	self["deal"] = dealID
	selfRec, _ := postJSON(t, r, http.MethodPost, "/orp", self, bearerHeader(pubToken))
	if selfRec.Code != http.StatusForbidden {
		t.Fatalf("self attestation %d %s", selfRec.Code, selfRec.Body.String())
	}

	// A party receipt with a host method is refused.
	hostMethod := attestation(maker, "m_rcpt_hm1", "place", "verified", "link")
	hostMethod["deal"] = dealID
	hmRec, _ := postJSON(t, r, http.MethodPost, "/orp", hostMethod, bearerHeader(makerToken))
	if hmRec.Code != http.StatusForbidden {
		t.Fatalf("host method %d %s", hmRec.Code, hmRec.Body.String())
	}

	// The counterparty's attestation verifies the obligation and mints the
	// pay that names the attester as payer.
	ok := attestation(maker, "m_rcpt_att1", "place", "verified", "attestation")
	ok["deal"] = dealID
	okRec, okBody := postJSON(t, r, http.MethodPost, "/orp", ok, bearerHeader(makerToken))
	if okRec.Code != http.StatusOK {
		t.Fatalf("attestation %d %s", okRec.Code, okRec.Body.String())
	}
	if okBody["type"] != "deal" {
		t.Fatalf("reply %s", okRec.Body.String())
	}
	row := obligationRow(t, entry.DB, dealID, "place")
	if row["state"] != "verified" || row["method"] != "attestation" || row["attester"] != maker {
		t.Fatalf("place %v", row)
	}

	// The payout exists because the attester is the payer.
	inboxMessages(t, r, pubToken) // the inbox read runs the payout sweep
	var count int
	if err := entry.DB.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM orp_payouts WHERE deal_id=$1`, dealID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("payouts %d", count)
	}

	// P7: an attestation-verified obligation never counts for reputation.
	query := orpEnvelope("reputation", "m_proof_rep1", maker, map[string]any{"subject": publisher})
	rec, repBody := postJSON(t, r, http.MethodPost, "/orp", query, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("reputation %d %s", rec.Code, rec.Body.String())
	}
	comp, _ := repBody["body"].(map[string]any)["components"].(map[string]any)
	if verified, _ := comp["verified"].(float64); verified != 0 {
		t.Fatalf("attestation counted in reputation %v", comp)
	}
}

func TestORPAttestationNotPayer(t *testing.T) {
	// The same obligation attested by the publisher-side payer counts only
	// for that payer: a pay obligation the offerer owes stays unminted.
	terms := attestTerms()
	terms["obligations"] = []any{
		map[string]any{
			"id":      "place",
			"action":  "place",
			"by":      "offerer",
			"content": map[string]any{"link": true},
			"proof":   []any{"attestation"},
		},
		map[string]any{
			"id":     "pay",
			"action": "pay",
			"by":     "offerer",
			"per":    "placement",
			"after":  "place",
			"amount": map[string]any{"amount_minor": 1500, "currency": "EUR"},
		},
	}
	entry, r := proofHost(t, staticFetch("", 0, false))
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")
	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "att2", terms)

	ok := attestation(publisher, "m_rcpt_att2", "place", "verified", "attestation")
	ok["deal"] = dealID
	okRec, _ := postJSON(t, r, http.MethodPost, "/orp", ok, bearerHeader(pubToken))
	if okRec.Code != http.StatusOK {
		t.Fatalf("attestation %d %s", okRec.Code, okRec.Body.String())
	}
	inboxMessages(t, r, pubToken)
	var count int
	if err := entry.DB.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM orp_payouts WHERE deal_id=$1`, dealID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("payout minted for the wrong payer %d", count)
	}
}

func keepTerms() map[string]any {
	return map[string]any{
		"title":      "zq proof kept link",
		"kind":       "affiliate",
		"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id":      "place",
				"action":  "place",
				"by":      "acceptor",
				"content": map[string]any{"link": true},
				"keep":    "P2D",
				"proof":   []any{"proof_url"},
			},
			map[string]any{
				"id":     "pay",
				"action": "pay",
				"by":     "offerer",
				"per":    "placement",
				"after":  "place",
				"amount": map[string]any{"amount_minor": 1500, "currency": "EUR"},
			},
		},
	}
}

func TestORPKeepRecheck(t *testing.T) {
	var page string
	fetch := func(ctx context.Context, rawURL string) (string, int, bool) {
		return page, 200, true
	}
	entry, r := proofHost(t, fetch)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	setPlacementPublish(t, entry.DB, ownerOfParty(publisher), "auto")
	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "keep1", keepTerms())

	// The page carries the tracked link at execute time.
	dealRec, dealDoc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(pubToken))
	if dealRec.Code != http.StatusOK {
		t.Fatalf("deal %d %s", dealRec.Code, dealRec.Body.String())
	}
	rows, _ := dealDoc["body"].(map[string]any)["tracking"].([]any)
	link, _ := rows[0].(map[string]any)["link"].(string)
	token := link[strings.LastIndex(link, "/go/"):]
	page = `<html><body>Sponsored <a href="` + token + `">the offer</a> Sponsored</body></html>`

	before := time.Now()
	code, _ := executeProof(t, r, pubToken, publisher, dealID, "place", "https://pub.example/post", "ex_k1")
	if *code != http.StatusOK {
		t.Fatalf("execute %d", *code)
	}
	row := obligationRow(t, entry.DB, dealID, "place")
	if row["state"] != "verified" || row["next_check"] == "" || row["keep_until"] == "" {
		t.Fatalf("place %v", row)
	}

	// The dependent payout stays held while keep rechecks are pending.
	inboxMessages(t, r, pubToken)
	var state string
	if err := entry.DB.Pool.QueryRow(context.Background(),
		`SELECT state FROM orp_payouts WHERE deal_id=$1`, dealID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "held" {
		t.Fatalf("payout %s", state)
	}

	// A day later the page is gone: the recheck fails the obligation, opens
	// a removed_early dispute, and reverses the held payout.
	page = `<html><body>Sponsored nothing here</body></html>`
	stats, err := SweepORPOutcomes(context.Background(), entry.DB, entry.BaseURL, before.Add(25*time.Hour), fetch)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rechecked != 1 {
		t.Fatalf("rechecked %d", stats.Rechecked)
	}
	row = obligationRow(t, entry.DB, dealID, "place")
	if row["state"] != "failed" {
		t.Fatalf("place %v", row)
	}
	if got := dealState(t, entry.DB, dealID); got != "disputed" {
		t.Fatalf("deal %s", got)
	}
	if err := entry.DB.Pool.QueryRow(context.Background(),
		`SELECT state FROM orp_payouts WHERE deal_id=$1`, dealID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "reversed" {
		t.Fatalf("payout %s", state)
	}

	var disputes, receipts int
	for _, item := range inboxMessages(t, r, pubToken) {
		msg, _ := item.(map[string]any)
		body, _ := msg["body"].(map[string]any)
		switch msg["type"] {
		case "dispute":
			if body["claim"] == "removed_early" {
				disputes++
			}
		case "receipt":
			if body["result"] == "failed" && body["reason"] == "removed" {
				receipts++
			}
		}
	}
	if disputes != 1 || receipts < 1 {
		t.Fatalf("disputes %d receipts %d", disputes, receipts)
	}
}

func TestORPTermsDirectClickPay(t *testing.T) {
	// T11: a direct place has no tracking link, so a per-click pay naming it
	// in after is rejected at offer validation.
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")

	terms := map[string]any{
		"title":      "zq t11",
		"kind":       "link_exchange",
		"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id":          "place",
				"action":      "place",
				"by":          "acceptor",
				"content":     map[string]any{"direct": true},
				"destination": "https://maker.example/tool",
				"proof":       []any{"proof_url"},
			},
			map[string]any{
				"id":     "pay",
				"action": "pay",
				"by":     "offerer",
				"per":    "click",
				"after":  "place",
				"amount": map[string]any{"amount_minor": 15, "currency": "EUR"},
			},
		},
	}
	offer := orpEnvelope("offer", "m_proof_t11", maker, map[string]any{"terms": terms})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusBadRequest || orpCode(body) != "invalid_message" {
		t.Fatalf("t11 offer %d %s", rec.Code, rec.Body.String())
	}

	// The same terms through a counter is refused too.
	goodID, _ := directedDeal(t, r, makerToken, maker, publisher)
	counter := orpEnvelope("counter", "m_proof_t11c", publisher, map[string]any{"terms": terms})
	counter["deal"] = goodID
	rec, body = postJSON(t, r, http.MethodPost, "/orp", counter, bearerHeader(pubToken))
	if rec.Code != http.StatusBadRequest || orpCode(body) != "invalid_message" {
		t.Fatalf("t11 counter %d %s", rec.Code, rec.Body.String())
	}
}
