package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

func TestORPTestDealProof(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	programID := insertProgram(t, db, "Zqcatalog Notes", "Zqcatalog", "zqcatalog-notes")
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry, SuggestLimiter: NewIPRateLimiter(30, time.Minute)}
	r := mountORP(entry, host)
	origin := "http://127.0.0.1:8080"

	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	beforeMoney := dealCounts(t, db)

	missRec, missBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d092c_find00", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if missRec.Code != http.StatusOK || missBody["body"].(map[string]any)["empty_reason"] != "no_match" {
		t.Fatalf("discover before offer %d %s", missRec.Code, missRec.Body.String())
	}

	bareRec, _ := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d092c_nobear", maker, standingOffer()), nil)
	if bareRec.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer %d %s", bareRec.Code, bareRec.Body.String())
	}
	keyRec, _ := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d092c_apikey", maker, standingOffer()), map[string]string{"X-API-Key": "site-key"})
	if keyRec.Code != http.StatusUnauthorized {
		t.Fatalf("api key %d %s", keyRec.Code, keyRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_offers`) != 0 {
		t.Fatal("offer stored without a bearer")
	}

	offerMsg := orpEnvelope("offer", "m_d092c_offer1", maker, standingOffer())
	offerRec, offerBody := postJSON(t, r, http.MethodPost, "/orp", offerMsg, bearerHeader(makerToken))
	if offerRec.Code != http.StatusOK || offerBody["type"] != "offer" {
		t.Fatalf("offer %d %s", offerRec.Code, offerRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/offer.schema.json"), offerBody); err != nil {
		t.Fatalf("offer schema: %v\n%s", err, offerRec.Body.String())
	}
	listed := offerBody["body"].(map[string]any)
	offerID, _ := listed["id"].(string)
	termsHash, _ := listed["terms_hash"].(string)
	if offerID == "" || termsHash != orp.TermsHash(testTerms()) {
		t.Fatalf("listed %s", offerRec.Body.String())
	}
	if !strings.HasSuffix(offerBody["ts"].(string), "Z") {
		t.Fatalf("ts %v", offerBody["ts"])
	}

	foundRec, foundBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d092c_find01", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if foundRec.Code != http.StatusOK {
		t.Fatalf("discover %d %s", foundRec.Code, foundRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/results.schema.json"), foundBody); err != nil {
		t.Fatalf("results schema: %v\n%s", err, foundRec.Body.String())
	}
	items := foundBody["body"].(map[string]any)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["pool"] != "test" {
		t.Fatalf("items %s", foundRec.Body.String())
	}
	if items[0].(map[string]any)["checks"].(map[string]any)["claims"] != "not_checked" {
		t.Fatalf("checks %s", foundRec.Body.String())
	}

	selfRec, selfBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d092c_self01", maker, map[string]any{
		"offer": offerID, "terms_hash": termsHash,
	}), bearerHeader(makerToken))
	if selfRec.Code != http.StatusForbidden || selfBody["body"].(map[string]any)["code"] != "forbidden" {
		t.Fatalf("self accept %d %s", selfRec.Code, selfRec.Body.String())
	}

	emptyInbox, emptyBody := postJSON(t, r, http.MethodGet, "/orp/inbox", nil, bearerHeader(pubToken))
	if emptyInbox.Code != http.StatusOK || len(emptyBody["messages"].([]any)) != 0 || emptyBody["next"] != "0" {
		t.Fatalf("empty inbox %d %s", emptyInbox.Code, emptyInbox.Body.String())
	}

	acceptMsg := orpEnvelope("accept", "m_d092c_accept", publisher, map[string]any{
		"offer": offerID, "terms_hash": termsHash,
	})
	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", acceptMsg, bearerHeader(pubToken))
	if acceptRec.Code != http.StatusOK || acceptBody["type"] != "deal" {
		t.Fatalf("accept %d %s", acceptRec.Code, acceptRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), acceptBody); err != nil {
		t.Fatalf("deal schema: %v\n%s", err, acceptRec.Body.String())
	}
	deal := acceptBody["body"].(map[string]any)
	dealID, _ := acceptBody["deal"].(string)
	if deal["state"] != "awaiting" || dealID == "" {
		t.Fatalf("deal %s", acceptRec.Body.String())
	}
	if _, ok := deal["tracking"]; ok {
		t.Fatalf("tracking minted %s", acceptRec.Body.String())
	}
	waiting := deal["waiting"].(map[string]any)
	reasons := stringList(waiting["reasons"])
	if !hasAll(reasons, "step_ask", "claims_unchecked") || !strings.Contains(waiting["url"].(string), "/orp/approve/") {
		t.Fatalf("waiting %v", waiting)
	}
	if !strings.HasSuffix(deal["updated"].(string), "Z") {
		t.Fatalf("updated %v", deal["updated"])
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != 1 {
		t.Fatal("deal count")
	}

	proofRec, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	proofHTML := proofRec.Body.String()
	if proofRec.Code != http.StatusOK || !strings.Contains(proofHTML, "Affiliate link") || strings.Contains(proofHTML, maker) || strings.Contains(proofHTML, publisher) {
		t.Fatalf("proof %d %s", proofRec.Code, proofHTML)
	}
	esc := waiting["escalation"].(string)
	approveRec, _ := postJSON(t, r, http.MethodGet, "/orp/approve/"+esc, nil, nil)
	approveHTML := approveRec.Body.String()
	if approveRec.Code != http.StatusOK || !strings.Contains(approveHTML, "zqtestmention") || !strings.Contains(approveHTML, "does not approve") {
		t.Fatalf("approve %d %s", approveRec.Code, approveHTML)
	}

	exec := orpEnvelope("execute", "m_d092c_exec01", publisher, map[string]any{
		"obligation": "place",
		"proof_url":  orp.ProofURL(origin, dealID),
	})
	exec["deal"] = dealID
	execRec, execBody := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(pubToken))
	if execRec.Code != http.StatusOK || execBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("execute %d %s", execRec.Code, execRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), execBody); err != nil {
		t.Fatalf("execute schema: %v\n%s", err, execRec.Body.String())
	}
	obligations := execBody["body"].(map[string]any)["obligations"].([]any)
	if obligations[0].(map[string]any)["state"] != "verified" {
		t.Fatalf("obligation %v", obligations)
	}

	inboxRec, inboxBody := postJSON(t, r, http.MethodGet, "/orp/inbox", nil, bearerHeader(pubToken))
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("inbox %d %s", inboxRec.Code, inboxRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "http/inbox.schema.json"), inboxBody); err != nil {
		t.Fatalf("inbox schema: %v\n%s", err, inboxRec.Body.String())
	}
	if !inboxHasReceipt(t, inboxBody["messages"].([]any)) {
		t.Fatalf("inbox %s", inboxRec.Body.String())
	}

	makerDealRec, makerDeal := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(makerToken))
	if makerDealRec.Code != http.StatusOK {
		t.Fatalf("maker deal %d %s", makerDealRec.Code, makerDealRec.Body.String())
	}
	makerWaiting := makerDeal["body"].(map[string]any)["waiting"].(map[string]any)
	if _, ok := makerWaiting["url"]; ok || makerWaiting["reasons"] != nil || makerWaiting["party"] != publisher {
		t.Fatalf("counterparty waiting %v", makerWaiting)
	}

	againRec, againBody := postJSON(t, r, http.MethodPost, "/orp", acceptMsg, bearerHeader(pubToken))
	if againRec.Code != http.StatusOK || againBody["deal"] != dealID || countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != 1 {
		t.Fatalf("replay %d %s", againRec.Code, againRec.Body.String())
	}
	changed := orpEnvelope("accept", "m_d092c_accept", publisher, map[string]any{
		"offer": offerID, "terms_hash": termsHash, "note": "different",
	})
	diffRec, diffBody := postJSON(t, r, http.MethodPost, "/orp", changed, bearerHeader(pubToken))
	if diffRec.Code != http.StatusConflict || diffBody["body"].(map[string]any)["code"] != "replay" {
		t.Fatalf("replay conflict %d %s", diffRec.Code, diffRec.Body.String())
	}

	receiptRec, receiptBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("receipt", "m_d092c_rcpt01", publisher, map[string]any{}), bearerHeader(pubToken))
	if receiptRec.Code != http.StatusForbidden || receiptBody["body"].(map[string]any)["message"] != "Only the host issues a receipt." {
		t.Fatalf("receipt %d %s", receiptRec.Code, receiptRec.Body.String())
	}
	payRec, payBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("payout", "m_d092c_paymsg", publisher, map[string]any{}), bearerHeader(pubToken))
	if payRec.Code != http.StatusForbidden || payBody["body"].(map[string]any)["code"] != "forbidden" {
		t.Fatalf("payout %d %s", payRec.Code, payRec.Body.String())
	}

	if again := dealCounts(t, db); again != beforeMoney {
		t.Fatalf("money rows changed: before %s after %s", beforeMoney, again)
	}
	var termsText string
	if err := db.Pool.QueryRow(context.Background(), `SELECT terms::text FROM orp_deals WHERE id=$1`, dealID).Scan(&termsText); err != nil || strings.Contains(termsText, "@") {
		t.Fatalf("terms pii %v %s", err, termsText)
	}
	var updated time.Time
	var kind string
	if err := db.Pool.QueryRow(context.Background(), `SELECT updated_at FROM orp_deals WHERE id=$1`, dealID).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT data_type FROM information_schema.columns WHERE table_name='orp_deals' AND column_name='updated_at'`).Scan(&kind); err != nil || kind != "timestamp with time zone" {
		t.Fatalf("updated_at type %s %v", kind, err)
	}
	parsed, err := time.Parse(time.RFC3339, deal["updated"].(string))
	if err != nil || !updated.UTC().Truncate(time.Second).Equal(parsed.UTC()) {
		t.Fatalf("updated roundtrip db %s json %s err %v", updated.UTC(), deal["updated"], err)
	}

	suggestRec, suggestBody := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": "zqcatalog zqcatalog"}, nil)
	if suggestRec.Code != http.StatusOK {
		t.Fatalf("suggest %d %s", suggestRec.Code, suggestRec.Body.String())
	}
	pitched := suggestBody["items"].([]any)[0].(map[string]any)["offer"].(map[string]any)
	if pitched["id"] != programID {
		t.Fatalf("catalog id %v", pitched["id"])
	}
	catRec, catBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d092c_catalg", publisher, map[string]any{
		"offer": pitched["id"], "terms_hash": pitched["terms_hash"],
	}), bearerHeader(pubToken))
	if catRec.Code != http.StatusOK || catBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("catalog accept %d %s", catRec.Code, catRec.Body.String())
	}
	if _, ok := catBody["body"].(map[string]any)["tracking"]; ok {
		t.Fatal("catalog tracking")
	}
	payTerms := testTerms()
	payTerms["obligations"] = append(payTerms["obligations"].([]any), map[string]any{
		"id": "commission", "action": "pay", "by": "offerer", "per": "deal",
		"amount": map[string]any{"amount_minor": 100, "currency": "USD"},
		"rail":   "manual", "proof": []any{"provider"},
	})
	payOfferRec, payOfferBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d092c_payofr", maker, map[string]any{"terms": payTerms}), bearerHeader(makerToken))
	if payOfferRec.Code != http.StatusOK {
		t.Fatalf("pay offer %d %s", payOfferRec.Code, payOfferRec.Body.String())
	}
	payListed := payOfferBody["body"].(map[string]any)
	payAcceptRec, payAcceptBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d092c_payacc", publisher, map[string]any{
		"offer": payListed["id"], "terms_hash": payListed["terms_hash"],
	}), bearerHeader(pubToken))
	if payAcceptRec.Code != http.StatusOK {
		t.Fatalf("pay accept %d %s", payAcceptRec.Code, payAcceptRec.Body.String())
	}
	payExec := orpEnvelope("execute", "m_d092c_payex1", maker, map[string]any{"obligation": "commission"})
	payExec["deal"] = payAcceptBody["deal"]
	payExecRec, payExecBody := postJSON(t, r, http.MethodPost, "/orp", payExec, bearerHeader(makerToken))
	if payExecRec.Code != http.StatusForbidden || payExecBody["body"].(map[string]any)["message"] != "This testing host does not settle money." {
		t.Fatalf("pay execute %d %s", payExecRec.Code, payExecRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM ledger_entries`) != 0 || countRows(t, db, `SELECT COUNT(*) FROM payouts`) != 0 {
		t.Fatal("ledger or payout written")
	}

	directed := orpEnvelope("offer", "m_d092c_direct", maker, standingOffer())
	directed["to"] = publisher
	dirRec, dirBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(makerToken))
	if dirRec.Code != http.StatusOK || dirBody["type"] != "deal" || dirBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("directed %d %s", dirRec.Code, dirRec.Body.String())
	}

	setPlacementPublish(t, db, ownerOfParty(publisher), "auto")
	autoOfferRec, autoOffer := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d092c_autoof", maker, standingOffer()), bearerHeader(makerToken))
	if autoOfferRec.Code != http.StatusOK {
		t.Fatalf("auto offer %d %s", autoOfferRec.Code, autoOfferRec.Body.String())
	}
	autoID := autoOffer["body"].(map[string]any)["id"].(string)
	autoHash := autoOffer["body"].(map[string]any)["terms_hash"].(string)
	autoRec, autoBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d092c_autoac", publisher, map[string]any{
		"offer": autoID, "terms_hash": autoHash,
	}), bearerHeader(pubToken))
	if autoRec.Code != http.StatusOK || autoBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("auto accept %d %s", autoRec.Code, autoRec.Body.String())
	}
	autoReasons := stringList(autoBody["body"].(map[string]any)["waiting"].(map[string]any)["reasons"])
	if len(autoReasons) != 1 || autoReasons[0] != "claims_unchecked" {
		t.Fatalf("auto reasons %v", autoReasons)
	}

	setPlacementPublish(t, db, ownerOfParty(publisher), "off")
	offOfferRec, offOffer := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d092c_offofr", maker, standingOffer()), bearerHeader(makerToken))
	if offOfferRec.Code != http.StatusOK {
		t.Fatalf("off offer %d %s", offOfferRec.Code, offOfferRec.Body.String())
	}
	dealsBeforeOff := countRows(t, db, `SELECT COUNT(*) FROM orp_deals`)
	offRec, offBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d092c_offacc", publisher, map[string]any{
		"offer":      offOffer["body"].(map[string]any)["id"],
		"terms_hash": offOffer["body"].(map[string]any)["terms_hash"],
	}), bearerHeader(pubToken))
	if offRec.Code != http.StatusForbidden || offBody["body"].(map[string]any)["code"] != "step_off" || offBody["body"].(map[string]any)["step"] != "accept" {
		t.Fatalf("off accept %d %s", offRec.Code, offRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/error.schema.json"), offBody); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != dealsBeforeOff {
		t.Fatal("off accept stored a deal")
	}

	host.Now = func() time.Time { return time.Now().Add(time.Hour) }
	staleReplay, staleBody := postJSON(t, r, http.MethodPost, "/orp", offerMsg, bearerHeader(makerToken))
	if staleReplay.Code != http.StatusOK || staleBody["id"] != offerBody["id"] {
		t.Fatalf("stored replay %d %s", staleReplay.Code, staleReplay.Body.String())
	}
}

func TestORPMessageRateLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey, TokenLimiter: NewIPRateLimiter(8, time.Minute)}
	host := &ORPHost{Entry: entry, MessageLimiter: NewIPRateLimiter(1, time.Minute)}
	r := mountORP(entry, host)
	token, party := joinParty(t, r)
	first, _ := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("hello", "m_d092c_hello1", party, map[string]any{"versions": []any{"0.3"}}), bearerHeader(token))
	if first.Code != http.StatusOK {
		t.Fatalf("hello %d %s", first.Code, first.Body.String())
	}
	second, body := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("hello", "m_d092c_hello2", party, map[string]any{"versions": []any{"0.3"}}), bearerHeader(token))
	if second.Code != http.StatusTooManyRequests || body["body"].(map[string]any)["code"] != "rate_limited" {
		t.Fatalf("limit %d %s", second.Code, second.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE party=$1`, party) != 1 {
		t.Fatal("rate limit was stored")
	}
}

func joinParty(t *testing.T, r http.Handler) (token, party string) {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	token, _ = body["token"].(string)
	party, _ = body["party"].(string)
	if !strings.HasPrefix(token, "orat_") || party == "" {
		t.Fatalf("join body %s", rec.Body.String())
	}
	return token, party
}

func orpEnvelope(typ, id, from string, body map[string]any) map[string]any {
	return map[string]any{
		"orp":  "0.3",
		"type": typ,
		"id":   id,
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"from": from,
		"body": body,
	}
}

func standingOffer() map[string]any {
	return map[string]any{"terms": testTerms()}
}

func testTerms() map[string]any {
	return map[string]any{
		"title":      "zqtestmention notes",
		"kind":       "sponsored",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id":          "place",
			"action":      "place",
			"by":          "acceptor",
			"content":     map[string]any{"link": true},
			"destination": "https://example.com/notes",
			"proof":       []any{"proof_url"},
		}},
	}
}

func setPlacementPublish(t *testing.T, db *database.DB, ownerID, mode string) {
	t.Helper()
	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM agent_step_modes WHERE owner_id=$1 AND site_id IS NULL`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO agent_step_modes(scope_key, owner_id, placement_publish)
VALUES('owner:' || $1::text, $1::uuid, $2)`, ownerID, mode); err != nil {
		t.Fatal(err)
	}
}

func ownerOfParty(party string) string {
	return party[strings.LastIndex(party, "/")+1:]
}

func hasAll(got []string, want ...string) bool {
	have := map[string]bool{}
	for _, item := range got {
		have[item] = true
	}
	for _, item := range want {
		if !have[item] {
			return false
		}
	}
	return true
}

func inboxHasReceipt(t *testing.T, messages []any) bool {
	t.Helper()
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		if msg["type"] != "receipt" {
			continue
		}
		if err := orp.ValidateAgainst(schemaPath(t, "messages/receipt.schema.json"), msg); err != nil {
			t.Fatal(err)
		}
		body, _ := msg["body"].(map[string]any)
		if body["method"] == "proof_url" && body["result"] == "verified" {
			return true
		}
	}
	return false
}
