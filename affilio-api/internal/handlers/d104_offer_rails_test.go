package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"affilio-api/internal/orp"
)

func TestD104ExecuteIsClaimOnlyWhenActive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, db, ownerOfParty(maker), "auto")

	offerRec, offerBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d104_offer", maker, standingOffer()), bearerHeader(makerToken))
	if offerRec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", offerRec.Code, offerRec.Body.String())
	}
	listed := offerBody["body"].(map[string]any)
	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_accept", publisher, map[string]any{
		"offer": listed["id"], "terms_hash": listed["terms_hash"],
	}), bearerHeader(pubToken))
	if acceptRec.Code != http.StatusOK || acceptBody["body"].(map[string]any)["state"] != "awaiting" {
		t.Fatalf("accept %d %s", acceptRec.Code, acceptRec.Body.String())
	}
	waiting := acceptBody["body"].(map[string]any)["waiting"].(map[string]any)
	reasons := stringList(waiting["reasons"])
	if !hasAll(reasons, "step_ask", "claims_unchecked") {
		t.Fatalf("waiting %v", waiting)
	}
	dealID := acceptBody["deal"].(string)
	exec := orpEnvelope("execute", "m_d104_exec_wait", publisher, map[string]any{
		"obligation": "place",
		"proof_url":  orp.ProofURL("http://127.0.0.1:8080", dealID),
	})
	exec["deal"] = dealID
	execRec, execBody := postJSON(t, r, http.MethodPost, "/orp", exec, bearerHeader(pubToken))
	if execRec.Code != http.StatusConflict || execBody["body"].(map[string]any)["code"] != "state_conflict" {
		t.Fatalf("awaiting execute %d %s", execRec.Code, execRec.Body.String())
	}
	if strings.Contains(execRec.Body.String(), "verified") {
		t.Fatalf("verified %s", execRec.Body.String())
	}
	var state string
	if err := db.Pool.QueryRow(context.Background(), `SELECT obligations->0->>'state' FROM orp_deals WHERE id=$1`, dealID).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("stored obligation %s %v", state, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->>'type'='receipt'`) != 0 || countRows(t, db, `SELECT COUNT(*) FROM placement_proposals`) != 0 {
		t.Fatal("execute wrote a receipt or a placement")
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='active' WHERE id=$1`, dealID); err != nil {
		t.Fatal(err)
	}
	claim := orpEnvelope("execute", "m_d104_exec_live", publisher, map[string]any{"obligation": "place"})
	claim["deal"] = dealID
	claimRec, claimBody := postJSON(t, r, http.MethodPost, "/orp", claim, bearerHeader(pubToken))
	if claimRec.Code != http.StatusOK || claimBody["body"].(map[string]any)["state"] != "active" {
		t.Fatalf("claim %d %s", claimRec.Code, claimRec.Body.String())
	}
	obligations := claimBody["body"].(map[string]any)["obligations"].([]any)
	if obligations[0].(map[string]any)["state"] != "claimed" {
		t.Fatalf("claimed %v", obligations)
	}
	if strings.Contains(claimRec.Body.String(), "verified") || countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->>'type'='receipt'`) != 0 {
		t.Fatal("active execute verified the placement")
	}

	payTerms := testTerms()
	payTerms["obligations"] = append(payTerms["obligations"].([]any), map[string]any{
		"id": "commission", "action": "pay", "by": "offerer", "per": "deal",
		"amount": map[string]any{"amount_minor": 100, "currency": "USD"},
		"rail":   "manual", "proof": []any{"provider"},
	})
	payRec, payBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d104_payofr", maker, map[string]any{"terms": payTerms}), bearerHeader(makerToken))
	if payRec.Code != http.StatusOK {
		t.Fatalf("pay offer %d %s", payRec.Code, payRec.Body.String())
	}
	payListed := payBody["body"].(map[string]any)
	payAccept, payAcceptBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_payacc", publisher, map[string]any{
		"offer": payListed["id"], "terms_hash": payListed["terms_hash"],
	}), bearerHeader(pubToken))
	if payAccept.Code != http.StatusOK {
		t.Fatalf("pay accept %d %s", payAccept.Code, payAccept.Body.String())
	}
	payDeal := payAcceptBody["deal"].(string)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state='active' WHERE id=$1`, payDeal); err != nil {
		t.Fatal(err)
	}
	payExec := orpEnvelope("execute", "m_d104_payex", maker, map[string]any{"obligation": "commission"})
	payExec["deal"] = payDeal
	payExecRec, payExecBody := postJSON(t, r, http.MethodPost, "/orp", payExec, bearerHeader(makerToken))
	if payExecRec.Code != http.StatusForbidden || payExecBody["body"].(map[string]any)["message"] != "This testing host does not settle money." {
		t.Fatalf("pay execute %d %s", payExecRec.Code, payExecRec.Body.String())
	}
}

func TestD104ListAskWaitsUntilApproved(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	r := mountORP(entry, &ORPHost{Entry: entry})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)

	offerRec, offerBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d104_ask", maker, standingOffer()), bearerHeader(makerToken))
	if offerRec.Code != http.StatusOK || offerBody["type"] != "offer" {
		t.Fatalf("ask offer %d %s", offerRec.Code, offerRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/offer.schema.json"), offerBody); err != nil {
		t.Fatalf("schema %v\n%s", err, offerRec.Body.String())
	}
	body := offerBody["body"].(map[string]any)
	offerID, _ := body["id"].(string)
	waiting, _ := body["waiting"].(map[string]any)
	if offerID == "" || waiting["party"] != maker || waiting["step"] != "list" || !hasAll(stringList(waiting["reasons"]), "step_ask") {
		t.Fatalf("waiting %s", offerRec.Body.String())
	}
	if _, ok := body["bound"]; ok || strings.Contains(offerRec.Body.String(), "mandate") {
		t.Fatalf("maker bound %s", offerRec.Body.String())
	}
	var listed bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT listed FROM orp_offers WHERE id=$1`, offerID).Scan(&listed); err != nil || listed {
		t.Fatalf("listed %v %v", listed, err)
	}
	foundRec, foundBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d104_find", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if foundRec.Code != http.StatusOK || foundBody["body"].(map[string]any)["empty_reason"] != "no_match" || strings.Contains(foundRec.Body.String(), offerID) {
		t.Fatalf("discover %d %s", foundRec.Code, foundRec.Body.String())
	}
	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_askacc", publisher, map[string]any{
		"offer": offerID, "terms_hash": body["terms_hash"],
	}), bearerHeader(pubToken))
	if acceptRec.Code != http.StatusNotFound || acceptBody["body"].(map[string]any)["code"] != "not_found" {
		t.Fatalf("accept waiting %d %s", acceptRec.Code, acceptRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != 0 {
		t.Fatal("waiting offer opened a deal")
	}

	setPlacementPublish(t, db, ownerOfParty(maker), "auto")
	autoRec, autoBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d104_auto", maker, standingOffer()), bearerHeader(makerToken))
	if autoRec.Code != http.StatusOK {
		t.Fatalf("auto offer %d %s", autoRec.Code, autoRec.Body.String())
	}
	autoListed := autoBody["body"].(map[string]any)
	if _, ok := autoListed["waiting"]; ok {
		t.Fatalf("auto waiting %s", autoRec.Body.String())
	}
	var autoFlag bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT listed FROM orp_offers WHERE id=$1`, autoListed["id"]).Scan(&autoFlag); err != nil || !autoFlag {
		t.Fatalf("auto listed %v %v", autoFlag, err)
	}
	seeRec, _ := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_d104_see", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if seeRec.Code != http.StatusOK || !strings.Contains(seeRec.Body.String(), autoListed["id"].(string)) {
		t.Fatalf("auto discover %d %s", seeRec.Code, seeRec.Body.String())
	}
	dealRec, dealBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_autobind", publisher, map[string]any{
		"offer": autoListed["id"], "terms_hash": autoListed["terms_hash"],
	}), bearerHeader(pubToken))
	if dealRec.Code != http.StatusOK {
		t.Fatalf("auto accept %d %s", dealRec.Code, dealRec.Body.String())
	}
	bound := dealBody["body"].(map[string]any)["bound"].([]any)
	first := bound[0].(map[string]any)
	if first["party"] != maker || first["authority"].(map[string]any)["type"] != "mandate" {
		t.Fatalf("mandate %v", bound)
	}
}

func TestD104WrongTermsHashIsMismatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	r := mountORP(entry, &ORPHost{Entry: entry})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, db, ownerOfParty(maker), "auto")
	offerRec, offerBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_d104_hashofr", maker, standingOffer()), bearerHeader(makerToken))
	if offerRec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", offerRec.Code, offerRec.Body.String())
	}
	offerID := offerBody["body"].(map[string]any)["id"].(string)
	for i, hash := range []string{"sha256-" + strings.Repeat("b", 43), "sha256:deadbeef", "not-a-hash"} {
		rec, body := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_bad_"+string(rune('a'+i)), publisher, map[string]any{
			"offer": offerID, "terms_hash": hash,
		}), bearerHeader(pubToken))
		code, _ := body["body"].(map[string]any)["code"].(string)
		if rec.Code != http.StatusConflict || code != "terms_mismatch" {
			t.Fatalf("hash %q %d %s", hash, rec.Code, rec.Body.String())
		}
	}
	missing, missingBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("accept", "m_d104_nohash", publisher, map[string]any{
		"offer": offerID,
	}), bearerHeader(pubToken))
	missingCode, _ := missingBody["body"].(map[string]any)["code"].(string)
	missingMsg, _ := missingBody["body"].(map[string]any)["message"].(string)
	if missing.Code != http.StatusBadRequest || missingCode != "invalid_message" || !strings.Contains(missingMsg, "sha256-<b64url43>") {
		t.Fatalf("missing %d %s", missing.Code, missing.Body.String())
	}
}

func TestD104SurfaceAndPlatformShareValues(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	r := mountORP(entry, &ORPHost{Entry: entry})
	token, _ := joinParty(t, r)
	draft := map[string]any{"text": "A short draft about writing tools for founders."}
	for _, value := range orp.DraftSurfaces {
		suggest := map[string]any{"text": draft["text"], "platform": value}
		rec, body := postJSON(t, r, http.MethodPost, "/orp/suggest", suggest, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("suggest %s %d %s", value, rec.Code, rec.Body.String())
		}
		_ = body
		monetizeSurface, _ := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"text": draft["text"], "surface": value}, bearerHeader(token))
		if monetizeSurface.Code != http.StatusOK {
			t.Fatalf("surface %s %d %s", value, monetizeSurface.Code, monetizeSurface.Body.String())
		}
		monetizePlatform, _ := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"text": draft["text"], "platform": value}, bearerHeader(token))
		if monetizePlatform.Code != http.StatusOK {
			t.Fatalf("platform %s %d %s", value, monetizePlatform.Code, monetizePlatform.Body.String())
		}
	}
	badSuggest, badSuggestBody := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": draft["text"], "platform": "tiktok"}, nil)
	suggestErr := badSuggestBody["body"].(map[string]any)
	if badSuggest.Code != http.StatusBadRequest || suggestErr["code"] != "invalid_message" || suggestErr["field"] != "/platform" || !listsSurfaces(suggestErr["message"].(string)) {
		t.Fatalf("suggest tiktok %d %s", badSuggest.Code, badSuggest.Body.String())
	}
	badSurface, badSurfaceBody := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"text": draft["text"], "surface": "tiktok"}, bearerHeader(token))
	if badSurface.Code != http.StatusBadRequest || badSurfaceBody["code"] != "invalid_message" || badSurfaceBody["field"] != "/surface" || !listsSurfaces(badSurfaceBody["error"].(string)) || !sameSurfaces(badSurfaceBody["allowed"]) {
		t.Fatalf("monetize surface %d %s", badSurface.Code, badSurface.Body.String())
	}
	badPlatform, badPlatformBody := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"text": draft["text"], "platform": "youtube"}, bearerHeader(token))
	if badPlatform.Code != http.StatusBadRequest || badPlatformBody["code"] != "invalid_message" || badPlatformBody["field"] != "/platform" || !listsSurfaces(badPlatformBody["error"].(string)) || !sameSurfaces(badPlatformBody["allowed"]) {
		t.Fatalf("monetize platform %d %s", badPlatform.Code, badPlatform.Body.String())
	}
	var surface string
	if err := db.Pool.QueryRow(context.Background(), `SELECT surface FROM monetization_suggestions WHERE surface='web' LIMIT 1`).Scan(&surface); err != nil || surface != "web" {
		t.Fatalf("stored web %q %v", surface, err)
	}
}

func TestD104FitReasonsUseDraftTerms(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	r := mountORP(entry, &ORPHost{Entry: entry})
	token, _ := joinParty(t, r)
	insertSuggestProgram(t, db, "Ahrefs Affiliate Program", "ahrefs-affiliate-program", "seo", []string{"zqfitdraft"})

	none, noneBody := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": "qxnomatchleaf qxnomatchleaf"}, nil)
	if none.Code != http.StatusOK || noneBody["empty_reason"] != "no_match" || strings.Contains(none.Body.String(), "The draft mentions") {
		t.Fatalf("none %d %s", none.Code, none.Body.String())
	}
	hit, hitBody := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": "zqfitdraft zqfitdraft"}, nil)
	if hit.Code != http.StatusOK {
		t.Fatalf("hit %d %s", hit.Code, hit.Body.String())
	}
	reason := fitReasonOf(t, hitBody, "Ahrefs Affiliate Program")
	if !strings.Contains(reason, "zqfitdraft") || strings.Contains(reason, "Ahrefs") || reason == orp.NeutralFitReason {
		t.Fatalf("suggest reason %q\n%s", reason, hit.Body.String())
	}

	monetizeNone, monetizeNoneBody := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"text": "qxnomatchleaf qxnomatchleaf", "surface": "web"}, bearerHeader(token))
	if monetizeNone.Code != http.StatusOK || len(suggestOffers(t, monetizeNoneBody)) != 0 || strings.Contains(monetizeNone.Body.String(), "The draft mentions") {
		t.Fatalf("monetize none %d %s", monetizeNone.Code, monetizeNone.Body.String())
	}
	monetizeHit, monetizeHitBody := postJSON(t, r, http.MethodPost, MonetizationSuggestPath, map[string]any{"draft": "zqfitdraft zqfitdraft", "platform": "blog"}, bearerHeader(token))
	if monetizeHit.Code != http.StatusOK {
		t.Fatalf("monetize hit %d %s", monetizeHit.Code, monetizeHit.Body.String())
	}
	var why string
	for _, offer := range suggestOffers(t, monetizeHitBody) {
		if offer["title"] == "Ahrefs Affiliate Program" {
			why, _ = offer["why_match"].(string)
		}
	}
	if !strings.Contains(why, "zqfitdraft") || strings.Contains(why, "Ahrefs") {
		t.Fatalf("why %q\n%s", why, monetizeHit.Body.String())
	}
}

func listsSurfaces(message string) bool {
	for _, value := range orp.DraftSurfaces {
		if !strings.Contains(message, value) {
			return false
		}
	}
	return true
}

func sameSurfaces(raw any) bool {
	items, _ := raw.([]any)
	if len(items) != len(orp.DraftSurfaces) {
		return false
	}
	have := map[string]bool{}
	for _, item := range items {
		text, _ := item.(string)
		have[text] = true
	}
	for _, value := range orp.DraftSurfaces {
		if !have[value] {
			return false
		}
	}
	return true
}

func fitReasonOf(t *testing.T, body map[string]any, title string) string {
	t.Helper()
	items, _ := body["items"].([]any)
	for _, item := range items {
		row, _ := item.(map[string]any)
		offer, _ := row["offer"].(map[string]any)
		terms, _ := offer["terms"].(map[string]any)
		if terms["title"] != title {
			continue
		}
		fit, _ := row["fit"].(map[string]any)
		reasons, _ := fit["reasons"].([]any)
		if len(reasons) == 0 {
			t.Fatalf("no reasons %v", row)
		}
		text, _ := reasons[0].(string)
		return text
	}
	t.Fatalf("missing %s in %v", title, body)
	return ""
}
