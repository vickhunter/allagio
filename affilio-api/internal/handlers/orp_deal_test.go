package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	setPlacementPublish(t, db, ownerOfParty(maker), "auto")
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
	if execRec.Code != http.StatusConflict || execBody["body"].(map[string]any)["code"] != "state_conflict" {
		t.Fatalf("execute %d %s", execRec.Code, execRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/error.schema.json"), execBody); err != nil {
		t.Fatalf("execute schema: %v\n%s", err, execRec.Body.String())
	}
	var obligationState string
	if err := db.Pool.QueryRow(context.Background(), `SELECT obligations->0->>'state' FROM orp_deals WHERE id=$1`, dealID).Scan(&obligationState); err != nil || obligationState != "pending" {
		t.Fatalf("obligation %s %v", obligationState, err)
	}
	if strings.Contains(execRec.Body.String(), `"verified"`) {
		t.Fatalf("execute verified a non-active deal %s", execRec.Body.String())
	}

	inboxRec, inboxBody := postJSON(t, r, http.MethodGet, "/orp/inbox", nil, bearerHeader(pubToken))
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("inbox %d %s", inboxRec.Code, inboxRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "http/inbox.schema.json"), inboxBody); err != nil {
		t.Fatalf("inbox schema: %v\n%s", err, inboxRec.Body.String())
	}
	if inboxHasReceipt(t, inboxBody["messages"].([]any)) {
		t.Fatalf("inbox receipt %s", inboxRec.Body.String())
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
	if payExecRec.Code != http.StatusConflict || payExecBody["body"].(map[string]any)["code"] != "state_conflict" {
		t.Fatalf("pay execute %d %s", payExecRec.Code, payExecRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM ledger_entries`) != 0 || countRows(t, db, `SELECT COUNT(*) FROM payouts`) != 0 {
		t.Fatal("ledger or payout written")
	}

	directed := orpEnvelope("offer", "m_d092c_direct", maker, standingOffer())
	directed["to"] = publisher
	dirRec, dirBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(makerToken))
	dirDeal, _ := dirBody["body"].(map[string]any)
	dirID, _ := dirBody["deal"].(string)
	if dirRec.Code != http.StatusOK || dirBody["type"] != "deal" || dirDeal["state"] != "proposed" || dirID == "" {
		t.Fatalf("directed %d %s", dirRec.Code, dirRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), dirBody); err != nil {
		t.Fatalf("directed schema: %v\n%s", err, dirRec.Body.String())
	}
	var directedState, directedBound string
	if err := db.Pool.QueryRow(context.Background(), `SELECT state, COALESCE(bound::text, '') FROM orp_deals WHERE id=$1`, dirID).Scan(&directedState, &directedBound); err != nil || directedState != "proposed" || strings.Contains(directedBound, publisher) {
		t.Fatalf("directed stored %s %s %v", directedState, directedBound, err)
	}
	earlyAct, earlyActBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dirID+"/activate", nil, bearerHeader(makerToken))
	earlyActInner, _ := earlyActBody["body"].(map[string]any)
	if earlyAct.Code != http.StatusConflict || earlyActInner["code"] != "state_conflict" || earlyActInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("activate before accept %d %s", earlyAct.Code, earlyAct.Body.String())
	}
	acceptorEarly, acceptorEarlyBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dirID+"/activate", nil, bearerHeader(pubToken))
	acceptorEarlyInner, _ := acceptorEarlyBody["body"].(map[string]any)
	if acceptorEarly.Code != http.StatusConflict || acceptorEarlyInner["code"] != "state_conflict" || acceptorEarlyInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("acceptor activate before accept %d %s", acceptorEarly.Code, acceptorEarly.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='awaiting' WHERE id=$1`, dirID); err != nil {
		t.Fatal(err)
	}
	forced, forcedBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dirID+"/activate", nil, bearerHeader(pubToken))
	forcedInner, _ := forcedBody["body"].(map[string]any)
	if forced.Code != http.StatusConflict || forcedInner["code"] != "state_conflict" || forcedInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("acceptor activate without accept message %d %s", forced.Code, forced.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE party=$1 AND status=200 AND reply->>'type'='deal' AND reply->>'deal'=$2`, publisher, dirID) != 0 {
		t.Fatal("accept stored before the counterparty accept")
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(bound::text, '') FROM orp_deals WHERE id=$1`, dirID).Scan(&directedBound); err != nil || strings.Contains(directedBound, publisher) {
		t.Fatalf("forced activate bound the acceptor %s %v", directedBound, err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='proposed' WHERE id=$1`, dirID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT state, COALESCE(bound::text, '') FROM orp_deals WHERE id=$1`, dirID).Scan(&directedState, &directedBound); err != nil || directedState != "proposed" || strings.Contains(directedBound, publisher) {
		t.Fatalf("activate before accept stored %s %s %v", directedState, directedBound, err)
	}
	offererAccept := orpEnvelope("accept", "m_d092c_diroff", maker, map[string]any{"terms_hash": dirDeal["terms_hash"]})
	offererAccept["deal"] = dirID
	offererAccRec, offererAccBody := postJSON(t, r, http.MethodPost, "/orp", offererAccept, bearerHeader(makerToken))
	offererAccInner, _ := offererAccBody["body"].(map[string]any)
	if offererAccRec.Code != http.StatusConflict || offererAccInner["code"] != "state_conflict" || offererAccInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("offerer accept %d %s", offererAccRec.Code, offererAccRec.Body.String())
	}
	counterAccept := orpEnvelope("accept", "m_d092c_diracc", publisher, map[string]any{"terms_hash": dirDeal["terms_hash"]})
	counterAccept["deal"] = dirID
	counterRec, counterBody := postJSON(t, r, http.MethodPost, "/orp", counterAccept, bearerHeader(pubToken))
	if counterRec.Code != http.StatusOK || counterBody["type"] != "deal" || counterBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("counterparty accept %d %s", counterRec.Code, counterRec.Body.String())
	}
	counterWaiting, _ := counterBody["body"].(map[string]any)["waiting"].(map[string]any)
	if !hasAll(stringList(counterWaiting["reasons"]), "claims_unchecked") {
		t.Fatalf("0.3 accept reasons %v", counterWaiting["reasons"])
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT state FROM orp_deals WHERE id=$1`, dirID).Scan(&directedState); err != nil || directedState != "awaiting" {
		t.Fatalf("accept stored %s %v", directedState, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE party=$1 AND status=200 AND reply->>'type'='deal' AND reply->>'deal'=$2`, publisher, dirID) != 1 {
		t.Fatal("counterparty accept was not stored")
	}
	dirAct, dirActBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dirID+"/activate", nil, bearerHeader(makerToken))
	if dirAct.Code != http.StatusOK || dirActBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("activate after accept %d %s", dirAct.Code, dirAct.Body.String())
	}
	if _, waiting := dirActBody["body"].(map[string]any)["waiting"]; waiting {
		t.Fatalf("directed activate still waiting %s", dirAct.Body.String())
	}
	dirBound, _ := dirActBody["body"].(map[string]any)["bound"].([]any)
	dirParties := map[string]bool{}
	for _, item := range dirBound {
		row, _ := item.(map[string]any)
		party, _ := row["party"].(string)
		dirParties[party] = true
	}
	if len(dirBound) != 2 || !dirParties[maker] || !dirParties[publisher] {
		t.Fatalf("directed bound %s", dirAct.Body.String())
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT state FROM orp_deals WHERE id=$1`, dirID).Scan(&directedState); err != nil || directedState != "active" {
		t.Fatalf("activate stored %s %v", directedState, err)
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

func TestORPLLMsWalkthrough(t *testing.T) {
	for _, phrase := range []string{
		`{"id":"place","by":"acceptor","action":"place","destination":"https://example.com/notes","proof":["proof_url"]}`,
		"obligation is the id from step 3",
		"The acceptor is the second party",
		"HTTP 403 forbidden",
		"HTTP 404 not_found",
		"No such offer for this sender.",
		"placement_publish auto",
		"platform is one of x, blog, newsletter, email, web",
		"Allowed values: x, blog, newsletter, email, web.",
		"409 state_conflict",
		"The deal is in the wrong state.",
		"Execute does not read proof_url",
		"obligation state becomes claimed",
		"The deal stays active",
		`{"obligation":"place"}`,
		"id is 8 to 128 characters of letters, digits, underscore, dot, colon, or hyphen",
		"A 6-character id is HTTP 400 invalid_message on /id",
		"The message is not valid ORP 0.3.",
		"A hello body requires versions",
		"versions is an array of 1 to 8 unique strings",
		"A three-part value such as 0.3.0 is HTTP 400 invalid_message",
		`The array must include "0.3"`,
		"HTTP 400 unsupported_version",
		"This host speaks 0.3.",
		"Optional hello body keys are capabilities, party, and limits",
		`The host reply versions is exactly ["0.3"]`,
		"https://offer-rails.onrender.com/orp/schemas/envelope.schema.json",
		"https://offer-rails.onrender.com/orp/schemas/messages/hello.schema.json",
		"https://offer-rails.onrender.com/orp/spec",
		"POST /orp/deals/<deal>/activate",
		"hello then discover then a directed offer (to) then accept by the second party then activate then execute then proof",
		"body.terms_hash is optional",
		"Omit it and the host computes the hash",
		"An empty or malformed hash is a generic HTTP 400 invalid_message",
		"Only a well-formed hash that does not match the terms is HTTP 409 terms_mismatch",
		"A missing or empty terms_hash is HTTP 400 invalid_message",
		"terms_hash must be sha256-<b64url43>.",
		"Any other string that does not match that hash is HTTP 409 terms_mismatch",
		"terms_hash does not match the terms.",
	} {
		if !strings.Contains(orpTestDealSection, phrase) {
			t.Fatalf("section missing %q", phrase)
		}
	}
	spec, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "protocol", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	appendixAt := strings.Index(string(spec), "## Appendix C:")
	if appendixAt < 0 {
		t.Fatal("SPEC has no Appendix C")
	}
	appendix := string(spec)[appendixAt:]
	formatAt := strings.Index(appendix, "Message format over")
	if formatAt < 0 {
		t.Fatal("Appendix C has no message format row")
	}
	formatEnd := strings.Index(appendix[formatAt:], "\n|")
	if formatEnd < 0 {
		t.Fatal("Appendix C message format row does not end")
	}
	formatRow := appendix[formatAt : formatAt+formatEnd]
	if strings.Contains(formatRow, "terms_hash") {
		t.Fatal("message format row states terms_hash for every POST /orp message")
	}
	for _, phrase := range []string{
		"On `accept`, a missing or empty `terms_hash` is HTTP 400 `invalid_message` and the message names `sha256-<b64url43>`",
		"Any other string that does not match the terms is HTTP 409 `terms_mismatch`",
		"terms_hash does not match the terms.",
		"On `offer`, `terms_hash` is optional and the host computes it",
		"An empty or malformed hash is a generic HTTP 400 `invalid_message`",
		"Only a well-formed hash that does not match the terms is HTTP 409 `terms_mismatch`",
	} {
		if !strings.Contains(appendix, phrase) {
			t.Fatalf("Appendix C missing %q", phrase)
		}
	}
	if strings.Contains(orpTestDealSection, "PR #83 lands") || strings.Contains(orpTestDealSection, "The deal stays awaiting") || strings.Contains(orpTestDealSection, "not claimed as live") {
		t.Fatal("walkthrough still describes the pre-D-104 host")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
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

	llmsRec, _ := postJSON(t, r, http.MethodGet, "/llms.txt", nil, nil)
	if llmsRec.Code != http.StatusOK || !strings.Contains(llmsRec.Body.String(), "## ORP 0.3 test deal") || !strings.Contains(llmsFullDocument(), orpTestDealSection) {
		t.Fatalf("llms %d", llmsRec.Code)
	}
	discDocRec, discDoc := postJSON(t, r, http.MethodGet, "/.well-known/orp.json", nil, nil)
	desc, _ := discDoc["description"].(string)
	if discDocRec.Code != http.StatusOK || discDoc["docs"] != origin+orp.DocsPath || !strings.Contains(desc, orp.ProofPageTemplate) || !strings.Contains(desc, "https://offer-rails.onrender.com/orp/schemas/envelope.schema.json") || !strings.Contains(desc, "https://offer-rails.onrender.com/orp/spec") || len(desc) > 500 {
		t.Fatalf("discovery %d %s", discDocRec.Code, discDocRec.Body.String())
	}

	offererRec, offererBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{}, nil)
	acceptorRec, acceptorBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{}, nil)
	if offererRec.Code != http.StatusCreated || acceptorRec.Code != http.StatusCreated {
		t.Fatalf("join %d %d", offererRec.Code, acceptorRec.Code)
	}
	offererToken, _ := offererBody["token"].(string)
	offerer, _ := offererBody["party"].(string)
	acceptorToken, _ := acceptorBody["token"].(string)
	acceptor, _ := acceptorBody["party"].(string)
	if offerer == "" || acceptor == "" || offerer == acceptor || offererBody["steps"].(map[string]any)["list"] != "ask" || acceptorBody["steps"].(map[string]any)["list"] != "ask" {
		t.Fatalf("parties %s %s", offererRec.Body.String(), acceptorRec.Body.String())
	}

	shortRec, shortBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "short1", offerer, map[string]any{
		"want": "offers", "query": "zqwalkthrough", "limit": 5,
	}), bearerHeader(offererToken))
	shortInner, _ := shortBody["body"].(map[string]any)
	if shortRec.Code != http.StatusBadRequest || shortInner["code"] != "invalid_message" || shortInner["field"] != "/id" || shortInner["message"] != "The message is not valid ORP 0.3." {
		t.Fatalf("short id %d %s", shortRec.Code, shortRec.Body.String())
	}
	longID := strings.Repeat("b", 129)
	longRec, longBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", longID, offerer, map[string]any{
		"want": "offers", "query": "zqwalkthrough", "limit": 5,
	}), bearerHeader(offererToken))
	longInner, _ := longBody["body"].(map[string]any)
	if longRec.Code != http.StatusBadRequest || longInner["code"] != "invalid_message" || longInner["field"] != "/id" {
		t.Fatalf("long id %d %s", longRec.Code, longRec.Body.String())
	}

	helloRec, helloBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("hello", "m_hello1", offerer, map[string]any{
		"versions": []any{"0.4", "0.3"},
	}), bearerHeader(offererToken))
	if helloRec.Code != http.StatusOK || helloBody["type"] != "hello" {
		t.Fatalf("hello %d %s", helloRec.Code, helloRec.Body.String())
	}
	helloVersions, _ := helloBody["body"].(map[string]any)["versions"].([]any)
	if len(helloVersions) != 1 || helloVersions[0] != "0.3" {
		t.Fatalf("hello versions %s", helloRec.Body.String())
	}
	for _, item := range []struct {
		id    string
		body  map[string]any
		code  string
		field string
	}{
		{id: "m_d105_patch", body: map[string]any{"versions": []any{"0.3.0"}}, code: "invalid_message", field: "/body/versions/0"},
		{id: "m_d105_oldv", body: map[string]any{"versions": []any{"0.2"}}, code: "unsupported_version", field: ""},
		{id: "m_d105_dupv", body: map[string]any{"versions": []any{"0.3", "0.3"}}, code: "invalid_message", field: "/body/versions/1"},
		{id: "m_d105_nine", body: map[string]any{"versions": []any{"0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9", "1.0", "1.1"}}, code: "invalid_message", field: "/body/versions"},
		{id: "m_d105_xkey", body: map[string]any{"versions": []any{"0.3"}, "extra": true}, code: "invalid_message", field: "/body/extra"},
	} {
		badRec, badBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("hello", item.id, offerer, item.body), bearerHeader(offererToken))
		badInner, _ := badBody["body"].(map[string]any)
		field, _ := badInner["field"].(string)
		if badRec.Code != http.StatusBadRequest || badInner["code"] != item.code || field != item.field {
			t.Fatalf("hello %s %d %s", item.id, badRec.Code, badRec.Body.String())
		}
		if item.code == "unsupported_version" && badInner["message"] != "This host speaks 0.3." {
			t.Fatalf("hello version message %s", badRec.Body.String())
		}
	}

	blogRec, _ := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{
		"text": "Walkthrough notes about a public page.", "platform": "blog",
	}, nil)
	if blogRec.Code != http.StatusOK {
		t.Fatalf("blog platform %d %s", blogRec.Code, blogRec.Body.String())
	}
	youtubeRec, youtubeBody := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{
		"text": "Walkthrough notes about a public page.", "platform": "youtube",
	}, nil)
	youtubeInner, _ := youtubeBody["body"].(map[string]any)
	if youtubeRec.Code != http.StatusBadRequest || youtubeInner["code"] != "invalid_message" || youtubeInner["message"] != "Allowed values: x, blog, newsletter, email, web." {
		t.Fatalf("youtube platform %d %s", youtubeRec.Code, youtubeRec.Body.String())
	}

	findRec, findBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d105_find01", offerer, map[string]any{
		"want": "offers", "query": "zqwalkthrough", "limit": 5,
	}), bearerHeader(offererToken))
	if findRec.Code != http.StatusOK || findBody["type"] != "results" {
		t.Fatalf("discover %d %s", findRec.Code, findRec.Body.String())
	}

	terms := map[string]any{
		"title":      "zqwalkthrough notes",
		"kind":       "sponsored",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id":          "place",
			"by":          "acceptor",
			"action":      "place",
			"destination": "https://example.com/notes",
			"proof":       []any{"proof_url"},
		}},
	}
	waitRec, waitBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d105_wait", offerer, map[string]any{"terms": terms}), bearerHeader(offererToken))
	if waitRec.Code != http.StatusOK || waitBody["type"] != "offer" {
		t.Fatalf("waiting offer %d %s", waitRec.Code, waitRec.Body.String())
	}
	waitingOffer := waitBody["body"].(map[string]any)
	waiting, _ := waitingOffer["waiting"].(map[string]any)
	waitID, _ := waitingOffer["id"].(string)
	waitHash, _ := waitingOffer["terms_hash"].(string)
	if waitID == "" || waiting["step"] != "list" || !hasAll(stringList(waiting["reasons"]), "step_ask", "claims_unchecked") {
		t.Fatalf("waiting %s", waitRec.Body.String())
	}
	hiddenRec, hiddenBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d105_find02", acceptor, map[string]any{
		"want": "offers", "query": "zqwalkthrough", "limit": 5,
	}), bearerHeader(acceptorToken))
	if hiddenRec.Code != http.StatusOK || strings.Contains(hiddenRec.Body.String(), waitID) || hiddenBody["body"].(map[string]any)["empty_reason"] != "no_match" {
		t.Fatalf("hidden discover %d %s", hiddenRec.Code, hiddenRec.Body.String())
	}
	missRec, missBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d105_miss", acceptor, map[string]any{
		"offer": waitID, "terms_hash": waitHash,
	}), bearerHeader(acceptorToken))
	missInner, _ := missBody["body"].(map[string]any)
	if missRec.Code != http.StatusNotFound || missInner["code"] != "not_found" || missInner["message"] != "No such offer for this sender." {
		t.Fatalf("accept waiting %d %s", missRec.Code, missRec.Body.String())
	}

	modeRec, modeBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"placement_publish": "auto"}, bearerHeader(offererToken))
	if modeRec.Code != http.StatusForbidden || modeBody["error"] != "a person approves this step" {
		t.Fatalf("join bearer publish %d %s", modeRec.Code, modeRec.Body.String())
	}
	directed := orpEnvelope("offer", "m_d105_direct", offerer, map[string]any{"terms": terms})
	directed["to"] = acceptor
	dirRec, dirBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(offererToken))
	dirDeal, _ := dirBody["body"].(map[string]any)
	dealID, _ := dirBody["deal"].(string)
	termsHash, _ := dirDeal["terms_hash"].(string)
	if dirRec.Code != http.StatusOK || dirBody["type"] != "deal" || dirDeal["state"] != "proposed" || dealID == "" || !strings.HasPrefix(termsHash, "sha256-") {
		t.Fatalf("directed %d %s", dirRec.Code, dirRec.Body.String())
	}
	for _, item := range []struct {
		name, typ, id, token string
		body                 map[string]any
		status               int
		code, message, field string
		deal                 bool
	}{
		{"empty offer hash", "offer", "m_d105_emptyh", offererToken, map[string]any{"terms": terms, "terms_hash": ""}, http.StatusBadRequest, "invalid_message", "The message is not valid ORP 0.3.", "/body/terms_hash", false},
		{"malformed offer hash", "offer", "m_d105_malhsh", offererToken, map[string]any{"terms": terms, "terms_hash": "not-a-hash"}, http.StatusBadRequest, "invalid_message", "The message is not valid ORP 0.3.", "/body/terms_hash", false},
		{"offer mismatch", "offer", "m_d105_mismh", offererToken, map[string]any{"terms": terms, "terms_hash": "sha256-" + strings.Repeat("b", 43)}, http.StatusConflict, "terms_mismatch", "terms_hash does not match the terms.", "", false},
		{"missing accept hash", "accept", "m_d105_nohash", acceptorToken, map[string]any{}, http.StatusBadRequest, "invalid_message", "terms_hash must be sha256-<b64url43>.", "/body/terms_hash", true},
		{"empty accept hash", "accept", "m_d105_emptya", acceptorToken, map[string]any{"terms_hash": ""}, http.StatusBadRequest, "invalid_message", "terms_hash must be sha256-<b64url43>.", "/body/terms_hash", true},
		{"accept other string", "accept", "m_d105_othrh", acceptorToken, map[string]any{"terms_hash": "not-a-hash"}, http.StatusConflict, "terms_mismatch", "terms_hash does not match the terms.", "", true},
		{"accept well-formed mismatch", "accept", "m_d105_wellh", acceptorToken, map[string]any{"terms_hash": "sha256-" + strings.Repeat("c", 43)}, http.StatusConflict, "terms_mismatch", "terms_hash does not match the terms.", "", true},
	} {
		msg := orpEnvelope(item.typ, item.id, map[string]string{"offer": offerer, "accept": acceptor}[item.typ], item.body)
		if item.deal {
			msg["deal"] = dealID
		}
		hashRec, hashBody := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(item.token))
		hashInner, _ := hashBody["body"].(map[string]any)
		field, _ := hashInner["field"].(string)
		if hashRec.Code != item.status || hashInner["code"] != item.code || hashInner["message"] != item.message || field != item.field {
			t.Fatalf("%s %d %s", item.name, hashRec.Code, hashRec.Body.String())
		}
	}
	beforeAccept, beforeBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptorToken))
	beforeInner, _ := beforeBody["body"].(map[string]any)
	if beforeAccept.Code != http.StatusConflict || beforeInner["code"] != "state_conflict" || beforeInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("acceptor activate before accept %d %s", beforeAccept.Code, beforeAccept.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE party=$1 AND status=200 AND reply->>'type'='deal' AND reply->>'deal'=$2`, acceptor, dealID) != 0 {
		t.Fatal("accept stored before the counterparty accept")
	}

	acceptMsg := orpEnvelope("accept", "m_d105_accept", acceptor, map[string]any{"terms_hash": termsHash})
	acceptMsg["deal"] = dealID
	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", acceptMsg, bearerHeader(acceptorToken))
	if acceptRec.Code != http.StatusOK || acceptBody["type"] != "deal" || acceptBody["deal"] != dealID {
		t.Fatalf("accept %d %s", acceptRec.Code, acceptRec.Body.String())
	}
	if acceptBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("deal %s", acceptRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE party=$1 AND status=200 AND reply->>'type'='deal' AND reply->>'deal'=$2`, acceptor, dealID) != 1 {
		t.Fatal("counterparty accept was not stored")
	}

	early := orpEnvelope("execute", "m_d105_early", acceptor, map[string]any{"obligation": "place"})
	early["deal"] = dealID
	earlyRec, earlyBody := postJSON(t, r, http.MethodPost, "/orp", early, bearerHeader(acceptorToken))
	earlyInner, _ := earlyBody["body"].(map[string]any)
	if earlyRec.Code != http.StatusConflict || earlyInner["code"] != "state_conflict" || earlyInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("awaiting execute %d %s", earlyRec.Code, earlyRec.Body.String())
	}
	var obligationState string
	if err := db.Pool.QueryRow(context.Background(), `SELECT obligations->0->>'state' FROM orp_deals WHERE id=$1`, dealID).Scan(&obligationState); err != nil || obligationState != "pending" {
		t.Fatalf("pending %s %v", obligationState, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->>'type'='receipt'`) != 0 {
		t.Fatal("awaiting execute wrote a receipt")
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='proposed' WHERE id=$1`, dealID); err != nil {
		t.Fatal(err)
	}
	wrongRec, wrongBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptorToken))
	wrongInner, _ := wrongBody["body"].(map[string]any)
	if wrongRec.Code != http.StatusConflict || wrongInner["code"] != "state_conflict" || wrongInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("activate proposed %d %s", wrongRec.Code, wrongRec.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='awaiting' WHERE id=$1`, dealID); err != nil {
		t.Fatal(err)
	}
	anonRec, anonBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, nil)
	anonInner, _ := anonBody["body"].(map[string]any)
	if anonRec.Code != http.StatusUnauthorized || anonInner["code"] != "unauthorized" || anonInner["message"] != "No credential or a bad credential." {
		t.Fatalf("activate anon %d %s", anonRec.Code, anonRec.Body.String())
	}
	strangerToken, _ := joinParty(t, r)
	strangerRec, strangerBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(strangerToken))
	strangerInner, _ := strangerBody["body"].(map[string]any)
	if strangerRec.Code != http.StatusNotFound || strangerInner["code"] != "not_found" || strangerInner["message"] != "No such deal for this sender." {
		t.Fatalf("activate stranger %d %s", strangerRec.Code, strangerRec.Body.String())
	}
	actRec, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptorToken))
	if actRec.Code != http.StatusOK || !strings.Contains(actRec.Header().Get("Content-Type"), "application/json") || actBody["type"] != "deal" {
		t.Fatalf("activate %d %s", actRec.Code, actRec.Body.String())
	}
	actDeal := actBody["body"].(map[string]any)
	if actDeal["state"] != "active" {
		t.Fatalf("activate state %s", actRec.Body.String())
	}
	if _, waiting := actDeal["waiting"]; waiting {
		t.Fatalf("activate still waiting %s", actRec.Body.String())
	}
	bound, _ := actDeal["bound"].([]any)
	parties := map[string]bool{}
	for _, item := range bound {
		row, _ := item.(map[string]any)
		party, _ := row["party"].(string)
		parties[party] = true
		authority, _ := row["authority"].(map[string]any)
		if authority["type"] != "mandate" {
			t.Fatalf("authority %v", row)
		}
	}
	if len(bound) != 2 || !parties[offerer] || !parties[acceptor] {
		t.Fatalf("bound %s", actRec.Body.String())
	}
	if _, ok := actDeal["tracking"]; ok {
		t.Fatalf("tracking minted %s", actRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), actBody); err != nil {
		t.Fatalf("activate schema: %v\n%s", err, actRec.Body.String())
	}
	if _, present := actDeal["approved_by"]; present || strings.Contains(actRec.Body.String(), "approved_by") {
		t.Fatalf("0.3 snapshot includes approved_by %s", actRec.Body.String())
	}
	read03, read03Body := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(acceptorToken))
	if read03.Code != http.StatusOK || strings.Contains(read03.Body.String(), "approved_by") {
		t.Fatalf("0.3 deal read %d %s", read03.Code, read03.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), read03Body); err != nil {
		t.Fatalf("0.3 read schema: %v\n%s", err, read03.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->'body'->>'state'='active'`) != 2 {
		t.Fatal("activate inbox")
	}
	againRec, againBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptorToken))
	againInner, _ := againBody["body"].(map[string]any)
	if againRec.Code != http.StatusConflict || againInner["code"] != "state_conflict" || againInner["message"] != "The deal is in the wrong state." {
		t.Fatalf("activate again %d %s", againRec.Code, againRec.Body.String())
	}
	makerExec := orpEnvelope("execute", "m_d105_execmk", offerer, map[string]any{"obligation": "place"})
	makerExec["deal"] = dealID
	makerExecRec, makerExecBody := postJSON(t, r, http.MethodPost, "/orp", makerExec, bearerHeader(offererToken))
	if makerExecRec.Code != http.StatusForbidden || makerExecBody["body"].(map[string]any)["code"] != "forbidden" {
		t.Fatalf("offerer execute %d %s", makerExecRec.Code, makerExecRec.Body.String())
	}

	exec := orpEnvelope("execute", "m_d105_execok", acceptor, map[string]any{"obligation": "place"})
	exec["deal"] = dealID
	execRec, execBody := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(acceptorToken))
	if execRec.Code != http.StatusOK || execBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("claim %d %s", execRec.Code, execRec.Body.String())
	}
	obligations := execBody["body"].(map[string]any)["obligations"].([]any)
	if obligations[0].(map[string]any)["id"] != "place" || obligations[0].(map[string]any)["state"] != "claimed" {
		t.Fatalf("obligation %v", obligations)
	}
	if strings.Contains(execRec.Body.String(), "verified") || countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->>'type'='receipt'`) != 0 {
		t.Fatal("active execute wrote a receipt")
	}

	proofRec, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	if proofRec.Code != http.StatusOK || !strings.Contains(proofRec.Header().Get("Content-Type"), "text/html") || !strings.Contains(proofRec.Body.String(), "Affiliate link") || !strings.Contains(proofRec.Body.String(), "The deal is active.") {
		t.Fatalf("proof %d %s", proofRec.Code, proofRec.Body.String())
	}
}

func TestORPConcurrentActivate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, db, ownerOfParty(maker), "auto")

	directed := orpEnvelope("offer", "m_d107_raceof", maker, standingOffer())
	directed["to"] = publisher
	dirRec, dirBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(makerToken))
	dirDeal, _ := dirBody["body"].(map[string]any)
	dealID, _ := dirBody["deal"].(string)
	if dirRec.Code != http.StatusOK || dirDeal["state"] != "proposed" || dealID == "" {
		t.Fatalf("directed %d %s", dirRec.Code, dirRec.Body.String())
	}
	accept := orpEnvelope("accept", "m_d107_raceac", publisher, map[string]any{"terms_hash": dirDeal["terms_hash"]})
	accept["deal"] = dealID
	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", accept, bearerHeader(pubToken))
	accepted, _ := acceptBody["body"].(map[string]any)
	if acceptRec.Code != http.StatusOK || accepted["state"] != "awaiting" {
		t.Fatalf("accept %d %s", acceptRec.Code, acceptRec.Body.String())
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, 2)
	bodies := make([]map[string]any, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec, body := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(pubToken))
			codes[i] = rec.Code
			bodies[i] = body
		}(i)
	}
	close(start)
	wg.Wait()

	ok, conflict := 0, 0
	for i, code := range codes {
		inner, _ := bodies[i]["body"].(map[string]any)
		switch code {
		case http.StatusOK:
			ok++
			if inner["state"] != "active" {
				t.Fatalf("winner %v", bodies[i])
			}
		case http.StatusConflict:
			conflict++
			if inner["code"] != "state_conflict" || inner["message"] != "The deal is in the wrong state." {
				t.Fatalf("loser %v", bodies[i])
			}
		default:
			t.Fatalf("activate %d %v", code, bodies[i])
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("codes %v", codes)
	}
	var state string
	if err := db.Pool.QueryRow(context.Background(), `SELECT state FROM orp_deals WHERE id=$1`, dealID).Scan(&state); err != nil || state != "active" {
		t.Fatalf("stored %s %v", state, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->'body'->>'state'='active'`) != 2 {
		t.Fatal("concurrent activate wrote more than one pair of snapshots")
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

func TestApprovedByKeepsEachParty(t *testing.T) {
	wire := approvedByWire(nil, []orp.Approval{
		{Party: "orp:hub.example/p_one", Step: "accept", By: "agent"},
		{Party: "orp:hub.example/p_two", Step: "accept", By: "agent"},
	})
	if len(wire) != 2 {
		t.Fatalf("collapsed accepts %v", wire)
	}
	rows, omit := approvalsFromQuery(nil, errors.New("db down"))
	if !omit || rows != nil {
		t.Fatalf("query failure rows %v omit %v", rows, omit)
	}
	okRows, okOmit := approvalsFromQuery([]database.DealApproval{{Step: "accept", Party: "orp:hub.example/p_one", By: "owner"}}, nil)
	if okOmit || len(okRows) != 1 {
		t.Fatalf("query success %v %v", okRows, okOmit)
	}

	terms, err := json.Marshal(testTerms())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	deal := database.ORPDeal{
		ID:        "deal_testunavailable",
		State:     "pending",
		Terms:     terms,
		TermsHash: "sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Parties:   json.RawMessage(`{"round":1,"seats":[{"id":"orp:hub.example/p_one","role":"maker","required":true},{"id":"orp:hub.example/p_two","role":"publisher","required":true}]}`),
		UpdatedAt: now,
	}
	deal.ApprovalsUnavailable = true
	snap, err := (&ORPHost{}).manySnapshot("orp:hub.example", "orp:hub.example/p_one", "", deal, now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := snap["body"].(map[string]any)
	if body["approvals_unavailable"] != true {
		t.Fatalf("0.4 marker %#v", body["approvals_unavailable"])
	}
	if _, ok := body["approved_by"]; ok {
		t.Fatal("failed read still sent approved_by")
	}
	plain, err := dealBody(deal, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain["approvals_unavailable"]; ok {
		t.Fatal("0.3 body includes approvals_unavailable")
	}
	if _, ok := plain["approved_by"]; ok {
		t.Fatal("0.3 body includes approved_by")
	}
}

func TestDealReadMarksApprovalFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	r := mountORP(entry, &ORPHost{Entry: entry})
	offererToken, offerer := joinParty(t, r)
	_, acceptor := joinParty(t, r)
	directed := orpEnvelope("offer", "m_d115_unread", offerer, standingOffer())
	directed["to"] = acceptor
	created, createdBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(offererToken))
	dealID, _ := createdBody["deal"].(string)
	if created.Code != http.StatusOK || dealID == "" {
		t.Fatalf("offer %d %s", created.Code, created.Body.String())
	}
	db.ApprovalsReadErr = errors.New("db down")
	plainRec, plainBody := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(offererToken))
	plainDeal, _ := plainBody["body"].(map[string]any)
	if plainRec.Code != http.StatusOK {
		t.Fatalf("0.3 read %d %s", plainRec.Code, plainRec.Body.String())
	}
	if _, ok := plainDeal["approvals_unavailable"]; ok {
		t.Fatal("0.3 body includes approvals_unavailable")
	}
	if _, ok := plainDeal["approved_by"]; ok {
		t.Fatal("0.3 body includes approved_by")
	}
	parties := `{"round":1,"seats":[{"id":"` + offerer + `","role":"maker","required":true},{"id":"` + acceptor + `","role":"publisher","required":true}],"approvals":[{"party":"` + offerer + `","step":"accept","approved_by":"agent"}]}`
	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET parties=$2::jsonb WHERE id=$1`, dealID, parties); err != nil {
		t.Fatal(err)
	}
	failed, failedBody := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(offererToken))
	failedDeal, _ := failedBody["body"].(map[string]any)
	if failed.Code != http.StatusOK || failedDeal["approvals_unavailable"] != true {
		t.Fatalf("0.4 read %d %s", failed.Code, failed.Body.String())
	}
	if _, ok := failedDeal["approved_by"]; ok {
		t.Fatalf("failed read sent approved_by %s", failed.Body.String())
	}
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
