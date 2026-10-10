package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/entry"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func TestRestoreOrRedoWithoutUndoReturnsNotUndone(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	siteID := createTestSite(t, db, "undo-miss-site", "undo-miss.example", "Undo")
	_, offerID := seedListedOffer(t, db, "undo-miss-maker", "undo-miss-maker@example.com", "Undo Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://undo-miss.example/post", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	rec, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil, map[string]string{"X-API-Key": "undo-miss-site"})
	if rec.Code != http.StatusConflict || body["reason"] != "not_undone" || body["message"] != "The placement was not undone." || body["error"] != nil {
		t.Fatalf("restore without undo: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDigestItemsIncludeUIFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "digest-ui", "digest-ui@example.com")
	siteID := createTestSite(t, db, "digest-ui-site", "digest-ui.example", "Digest UI")
	makerID, offerID := seedListedOffer(t, db, "digest-ui-maker", "digest-ui-maker@example.com", "Digest Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	older := insertPlacement(t, db, siteID, offerID, "https://digest-ui.example/old", "Digest Product older.")
	newer := insertPlacement(t, db, siteID, offerID, "https://digest-ui.example/new", "Digest Product newer.")
	markPlacementLive(t, db, siteID, offerID, older, "digest-ui-go", "https://digest-ui.example/old")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), campaign_id=(SELECT campaign_id FROM placement_proposals WHERE id=$2), expires_at=NOW()+INTERVAL '2 days', restore_unconfirmed=false, matched_rule='hard_checks' WHERE id=$1`, newer, older); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET
post_title='The post', position='end', link_text='Buy the tool', label='Recommended',
approved_by_kind='policy:yolo', policy_version=4, plugin_wait_at=NOW()-INTERVAL '2 hours'
WHERE id=$1`, older); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO clicks(site_id, placement_id) VALUES ($1,$2),($1,$2)`, siteID, older); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO owner_events(kind, user_id, created_at) VALUES('digest_opened',$1,NOW()-INTERVAL '1 hour')`, owner.ID); err != nil {
		t.Fatal(err)
	}

	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	if body["went_live"] != float64(2) || body["went_live_since_last"] != float64(1) {
		t.Fatalf("counts: %s", rec.Body.String())
	}
	if body["change_url"] != "http://127.0.0.1:8080/owner" || body["site_url"] != "https://digest-ui.example/" {
		t.Fatalf("urls: %s", rec.Body.String())
	}
	modes, _ := body["available_modes"].([]any)
	if len(modes) != 3 || modes[0] != "manual" || modes[1] != "auto" || modes[2] != "yolo" {
		t.Fatalf("modes: %v", body["available_modes"])
	}
	paused, _ := body["paused"].(map[string]any)
	if paused["account"] != false || paused["site"] != false || paused["offer"] != false {
		t.Fatalf("paused: %v", paused)
	}
	var placement, listing map[string]any
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		switch item["id"] {
		case older:
			placement = item
		case offerID:
			listing = item
		}
	}
	if placement == nil || listing == nil {
		t.Fatalf("items: %s", rec.Body.String())
	}
	if placement["plugin_state"] != "placed" || placement["live_now"] != true || placement["post_title"] != "The post" || placement["position"] != "end" || placement["link_text"] != "Buy the tool" || placement["label"] != "Recommended" || placement["clicks"] != float64(2) || placement["approved_by"] != "policy:yolo" || placement["policy_rule_id"] != "hard_checks_passed" || placement["policy_rule_text"] != "Passed every hard check." || placement["rules_version"] != float64(4) {
		t.Fatalf("placement item: %v", placement)
	}
	if listing["kind"] != "offer" || listing["label"] != "Digest Product" || listing["live_now"] != true || listing["plugin_state"] != "" || listing["clicks"] != float64(0) || listing["rules_version"] != float64(0) || listing["undo_token"] == "" {
		t.Fatalf("listing item: %v", listing)
	}

	accountRec, account := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if accountRec.Code != http.StatusOK || account["went_live_since_last"] != float64(1) || account["change_url"] != body["change_url"] || account["site_url"] != body["site_url"] {
		t.Fatalf("account digest: %d %s", accountRec.Code, accountRec.Body.String())
	}

	killed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", nil)
	if killed.Code != http.StatusOK {
		t.Fatalf("kill: %d %s", killed.Code, killed.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	paused, _ = body["paused"].(map[string]any)
	if rec.Code != http.StatusOK || paused["site"] != true || paused["account"] != false || paused["offer"] != false || body["went_live"] != float64(0) {
		t.Fatalf("site pause: %d %s", rec.Code, rec.Body.String())
	}
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == older && item["live_now"] != false {
			t.Fatalf("live_now after pause: %v", item)
		}
	}
	pausedAll, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "all"})
	if pausedAll.Code != http.StatusOK {
		t.Fatalf("account pause: %d %s", pausedAll.Code, pausedAll.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	paused, _ = body["paused"].(map[string]any)
	if paused["account"] != true || paused["site"] != true || paused["offer"] != true {
		t.Fatalf("account pause state: %v", paused)
	}
}

func TestSetupProjectListingAndCommission(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "setup-money", "setup-money@example.com")
	siteID := createTestSite(t, db, "setup-money-site", "setup-money.example", "Setup Money")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='cursor' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=maker&install_path=cli", nil)
	if rec.Code != http.StatusOK || body["project_name"] != "" || body["listing_url"] != "" || body["commission"] != nil || body["listing_price"] != nil || body["change_url"] != "http://127.0.0.1:8080/owner" || body["site_url"] != "https://setup-money.example/" {
		t.Fatalf("empty setup: %d %s", rec.Code, rec.Body.String())
	}
	modes, _ := body["available_modes"].([]any)
	if len(modes) != 3 || modes[0] != "manual" || modes[2] != "yolo" {
		t.Fatalf("available modes: %v", modes)
	}
	base := map[string]any{
		"role": "maker", "install_path": "cli", "mode_shown": "yolo", "mode": "auto",
		"promote": "API toolkit", "project_name": "Prompt Forge", "listing_url": "https://promptforge.example/pricing",
	}
	percent := cloneMap(base)
	percent["commission"] = map[string]any{"type": "percent", "value": 30}
	percent["listing_price"] = map[string]any{"value": 49, "currency": "USD"}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", percent)
	commission, _ := body["commission"].(map[string]any)
	price, _ := body["listing_price"].(map[string]any)
	if rec.Code != http.StatusOK || body["project_name"] != "Prompt Forge" || body["listing_url"] != "https://promptforge.example/pricing" || commission["type"] != "percent" || commission["value"] != float64(30) || commission["currency"] != "" || price["value"] != float64(49) || price["currency"] != "USD" || body["site_url"] != "https://setup-money.example/" {
		t.Fatalf("percent save: %d %s", rec.Code, rec.Body.String())
	}
	fixedMissing := cloneMap(base)
	fixedMissing["commission"] = map[string]any{"type": "fixed", "value": 10}
	rec, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", fixedMissing)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("fixed without currency: %d %s", rec.Code, rec.Body.String())
	}
	mismatch := cloneMap(base)
	mismatch["commission"] = map[string]any{"type": "fixed", "value": 10, "currency": "USD"}
	mismatch["listing_price"] = map[string]any{"value": 49, "currency": "EUR"}
	rec, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", mismatch)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("currency mismatch: %d %s", rec.Code, rec.Body.String())
	}
	fixed := cloneMap(base)
	fixed["commission"] = map[string]any{"type": "fixed", "value": 10, "currency": "usd"}
	fixed["listing_price"] = map[string]any{"value": 49, "currency": "USD"}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", fixed)
	commission, _ = body["commission"].(map[string]any)
	if rec.Code != http.StatusOK || commission["type"] != "fixed" || commission["value"] != float64(10) || commission["currency"] != "USD" {
		t.Fatalf("fixed save: %d %s", rec.Code, rec.Body.String())
	}
	readiness := cloneMap(fixed)
	readiness["readiness_signals"] = []string{"payments"}
	rec, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", readiness)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("readiness accepted: %d %s", rec.Code, rec.Body.String())
	}
	types := cloneMap(fixed)
	types["publisher_types_to_avoid"] = []string{"coupon"}
	rec, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", types)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("publisher types accepted: %d %s", rec.Code, rec.Body.String())
	}
	api.Claims = entry.StubClaimChecker{}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=maker", nil)
	if rec.Code != http.StatusOK || body["mode"] != "manual" || body["mode_preselected"] != "yolo" || body["approval_reason"] != entry.ClaimCheckReason || body["approval_message"] != entry.ClaimCheckMessage {
		t.Fatalf("stub mode: %d %s", rec.Code, rec.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK || body["effective_mode"] != "manual" || body["approval_reason"] != entry.ClaimCheckReason || body["approval_message"] != entry.ClaimCheckMessage {
		t.Fatalf("stub digest: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMakerProjectKeyReadsPlacementStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, _ := testOwnerSession(t, h, "maker-read", "maker-read@example.com")
	siteID := createTestSite(t, db, "maker-read-site", "maker-read.example", "Maker Read")
	makerID, offerID := seedListedOffer(t, db, "maker-read-maker", "maker-read-maker@example.com", "Maker Product")
	_, otherOffer := seedListedOffer(t, db, "maker-read-other", "maker-read-other@example.com", "Other Product")
	var secondOffer string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'listed','percentage',30,'https://example.com/other' FROM offers WHERE id=$1 RETURNING id`, offerID).Scan(&secondOffer); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	own := insertPlacement(t, db, siteID, offerID, "https://maker-read.example/own", "Honest note about the product.")
	sibling := insertPlacement(t, db, siteID, secondOffer, "https://maker-read.example/sib", "Honest note about the product.")
	foreign := insertPlacement(t, db, siteID, otherOffer, "https://maker-read.example/foreign", "Honest note about the product.")
	offerKey := "or_maker_read_key"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,offer_id,maker_id,expires_at)
VALUES($1,'or_make',$2,'offer',$3,$4,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(offerKey), owner.ID, offerID, makerID); err != nil {
		t.Fatal(err)
	}
	otherKey := "or_maker_other_key"
	var otherMaker string
	if err := db.Pool.QueryRow(context.Background(), `SELECT maker_id::text FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, otherOffer).Scan(&otherMaker); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,offer_id,maker_id,expires_at)
VALUES($1,'or_othe',$2,'offer',$3,$4,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(otherKey), owner.ID, otherOffer, otherMaker); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-Maker-Key": offerKey}
	rec, body := postJSON(t, r, http.MethodGet, "/v1/placements/"+own, nil, headers)
	if rec.Code != http.StatusOK || body["id"] != own || body["status"] != "pending" || body["plugin_state"] != "pending" || body["fallback_url"] == "" {
		t.Fatalf("own status: %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+own+"/retry", nil, headers)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("maker retry: %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodGet, "/v1/placements/"+sibling, nil, headers)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sibling offer: %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodGet, "/v1/placements/"+foreign, nil, map[string]string{"X-Maker-Key": otherKey})
	if rec.Code != http.StatusOK {
		t.Fatalf("other maker own offer: %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodGet, "/v1/placements/"+own, nil, map[string]string{"X-Maker-Key": otherKey})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other maker: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUndoListingHasNoRedoInV1(t *testing.T) {
	found := false
	for _, action := range entry.ScreenActions {
		if action.Action != "Undo a listing (no redo in v1)" {
			continue
		}
		found = true
		if action.SessionOnly || action.Method != http.MethodPost || action.Path != "/v1/digest/undo" || action.Screen != "digest" {
			t.Fatalf("parity row: %+v", action)
		}
	}
	if !found {
		t.Fatal("missing undo-listing parity row")
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{}).Mount(r)
	for _, route := range r.Routes() {
		if route.Path == "/v1/offers/:id/restore" || route.Path == "/v1/listings/:id/redo" {
			t.Fatalf("listing redo is mounted: %s %s", route.Method, route.Path)
		}
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, engine := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "listing-undo", "listing-undo@example.com")
	makerID, offerID := seedListedOffer(t, db, "listing-undo-maker", "listing-undo@example.com", "Listing Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET updated_at=NOW() WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, engine, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	var token string
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == offerID {
			token, _ = item["undo_token"].(string)
		}
	}
	if token == "" {
		t.Fatalf("no listing token: %s", rec.Body.String())
	}
	rec, undone := serveOwner(t, engine, h, cookie, owner.CSRF, http.MethodPost, "/v1/digest/undo", map[string]any{"undo_token": token})
	if rec.Code != http.StatusOK || undone["kind"] != "offer" || undone["offer_id"] != offerID {
		t.Fatalf("undo listing: %d %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM offers WHERE id=$1`, offerID).Scan(&status); err != nil || status != "paused" {
		t.Fatalf("offer status %s %v", status, err)
	}
	redoReq := httptest.NewRequest(http.MethodPost, "/v1/offers/"+offerID+"/restore", nil)
	redoRec := httptest.NewRecorder()
	engine.ServeHTTP(redoRec, redoReq)
	if redoRec.Code != http.StatusNotFound {
		t.Fatalf("listing redo status %d", redoRec.Code)
	}
	_ = api
}

func TestHardChecksAreTheUISource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{}).Mount(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/policy/hard-checks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("hard checks: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	checks, _ := body["checks"].([]any)
	if len(checks) != len(entry.HardCheckCatalog) {
		t.Fatalf("checks: %s", rec.Body.String())
	}
	for i, check := range entry.HardCheckCatalog {
		item := checks[i].(map[string]any)
		if item["check_id"] != check.ID || item["check_text"] != check.Text || len(item) != 2 {
			t.Fatalf("check %d: %v", i, item)
		}
	}
	llms := llmsBody()
	modes, ok := agentDoc("modes.md")
	if !ok || !containsAll(llms, "GET /v1/policy/hard-checks", "is the single source the UI renders") || !containsAll(modes, "GET /v1/policy/hard-checks", "is the single source the UI renders") {
		t.Fatal("docs do not name the hard-check endpoint as the UI source")
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}
