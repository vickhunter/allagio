package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func newStepModeRouter(db *database.DB, claims entry.ClaimChecker) (*HostedHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &HostedHandler{DB: db, BaseURL: "http://127.0.0.1:8080"}
	api := &AgentEntry{DB: db, BaseURL: h.BaseURL, Owner: h.RequireOwnerSession, Claims: claims}
	api.Mount(r)
	r.POST("/v1/sites/register", (&SitesHandler{DB: db}).Register)
	placements := &PlacementsHandler{DB: db, Claims: claims}
	payouts := &PayoutsHandler{DB: db}
	review := &OfferReviewHandler{DB: db}
	secured := r.Group("/v1", middleware.APIKeyAuth(db, nil))
	secured.POST("/revenue/opportunities", placements.Find)
	secured.POST("/placements", placements.Prepare)
	secured.POST("/marketplace/offers/:id/join", (&MarketplaceHandler{DB: db}).JoinOffer)
	secured.POST("/payouts/:id/confirm-received", payouts.ConfirmReceived)
	admin := r.Group("/v1", middleware.AdminKeyAuth("test-admin", nil))
	admin.GET("/offer-review/queue", review.Queue)
	admin.POST("/offer-review/steps/:id/approve", review.ApproveStep)
	admin.POST("/payout-batches/run", payouts.RunBatch)
	admin.POST("/payout-batches/:id/mark-paid", payouts.MarkPaid)
	owner := r.Group("/owner", h.RequireOwnerSession)
	owner.GET("", h.Dashboard)
	owner.POST("/accounts", h.OwnerAccount)
	owner.POST("/placements/:id/approve", h.ApprovePlacement)
	owner.POST("/payouts/:id/confirm-received", h.requirePublisher, h.ConfirmReceived)
	h.mountWorkspace(r.Group(""), owner)
	return h, r
}

func signupPublisher(t *testing.T, r http.Handler, email, domain string) (ownerID, siteID, apiKey string) {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Step Publisher", "email": email, "domain": domain,
	}, nil)
	if rec.Code != http.StatusCreated || body["api_key"] == "" || body["site_id"] == "" {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}
	return body["owner_id"].(string), body["site_id"].(string), body["api_key"].(string)
}

func putStepModes(t *testing.T, r http.Handler, apiKey string, modes map[string]any) map[string]any {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPut, "/v1/step-modes", modes, map[string]string{"X-API-Key": apiKey})
	if rec.Code != http.StatusOK {
		t.Fatalf("put modes: %d %s", rec.Code, rec.Body.String())
	}
	return body
}

func modeMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	modes, ok := body["modes"].(map[string]any)
	if !ok {
		t.Fatalf("modes: %v", body)
	}
	return modes
}

func verifySite(t *testing.T, db *database.DB, siteID, host string) {
	t.Helper()
	tag, err := db.Pool.Exec(context.Background(), `UPDATE sites SET verified_at=NOW(), verification_host=$2 WHERE id=$1`, siteID, host)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("verify site: %v rows %d", err, tag.RowsAffected())
	}
}

func countRows(t *testing.T, db *database.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func stepArticle(product string) string {
	return product + " devtools reviews every pull request. " + product + " finds bugs before merge. This devtools note is about " + product + "."
}

const stepCopy = "We use CodeLens to catch bugs before merge."

func preparePlacement(t *testing.T, r http.Handler, apiKey, offerID, surface string, postID int) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": surface, "context": stepArticle("CodeLens"), "copy": stepCopy, "post_id": postID,
	}, map[string]string{"X-API-Key": apiKey})
}

func placementBody(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	placement, ok := body["placement"].(map[string]any)
	if !ok {
		t.Fatalf("placement body: %v", body)
	}
	return placement
}

func approveStep(t *testing.T, r http.Handler, id string) map[string]any {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/v1/offer-review/steps/"+id+"/approve", map[string]any{}, map[string]string{"X-Admin-Key": "test-admin"})
	if rec.Code != http.StatusOK || body["status"] != "approved" {
		t.Fatalf("approve step: %d %s", rec.Code, rec.Body.String())
	}
	return body
}

func TestStepModesDefaultToAuto(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, r, "modes-default@example.com", "modes-default.example")
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes`) != 0 {
		t.Fatal("a missing setting was stored")
	}
	rec, body := postJSON(t, r, http.MethodGet, "/v1/step-modes", nil, map[string]string{"X-API-Key": key})
	modes := modeMap(t, body)
	if rec.Code != http.StatusOK || body["site_id"] != siteID || modes["site_signup"] != "auto" || modes["offer_match"] != "auto" || modes["placement_publish"] != "auto" || modes["link_change"] != "auto" || modes["payout"] != "auto" {
		t.Fatalf("defaults: %d %s", rec.Code, rec.Body.String())
	}
	if body["placement_publish_effective"] != "ask" || body["placement_publish_reason"] != "domain_unverified" {
		t.Fatalf("unverified publish: %s", rec.Body.String())
	}
	saved := putStepModes(t, r, key, map[string]any{"placement_publish": "ask"})
	modes = modeMap(t, saved)
	if modes["placement_publish"] != "ask" || modes["offer_match"] != "auto" || modes["payout"] != "auto" || countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes`) != 1 {
		t.Fatalf("partial save: %s", saved)
	}
	bad, badBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"placement_publish": "yolo"}, map[string]string{"X-API-Key": key})
	if bad.Code != http.StatusBadRequest || badBody["error"] != "mode must be auto, ask, or off" {
		t.Fatalf("bad mode: %d %s", bad.Code, bad.Body.String())
	}
}

func TestSiteSignupAskAndOff(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, _, key := signupPublisher(t, r, "signup-gate@example.com", "signup-gate.example")

	auto, autoBody := postJSON(t, r, http.MethodPost, "/v1/sites/register", map[string]any{"domain": "signup-auto.example", "name": "Auto"}, map[string]string{"X-API-Key": key})
	if auto.Code != http.StatusOK || autoBody["site_id"] == "" {
		t.Fatalf("auto register: %d %s", auto.Code, auto.Body.String())
	}
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Second", "email": "signup-gate@example.com", "domain": "signup-second.example",
	}, nil)
	if again.Code != http.StatusCreated || againBody["site_id"] == "" {
		t.Fatalf("auto repeat signup: %d %s", again.Code, again.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"site_signup": "off"})
	off, offBody := postJSON(t, r, http.MethodPost, "/v1/sites/register", map[string]any{"domain": "signup-off.example", "name": "Off"}, map[string]string{"X-API-Key": key})
	if off.Code != http.StatusForbidden || offBody["reason"] != "step_off" || offBody["step"] != "site_signup" || offBody["error"] != offBody["message"] || !strings.Contains(offBody["message"].(string), "Site signup is off") {
		t.Fatalf("off register: %d %s", off.Code, off.Body.String())
	}
	repeat, repeatBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Blocked", "email": "signup-gate@example.com", "domain": "signup-blocked.example",
	}, nil)
	if repeat.Code != http.StatusForbidden || repeatBody["reason"] != "step_off" || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='signup-off.example'`) != 0 {
		t.Fatalf("off signup: %d %s", repeat.Code, repeat.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"site_signup": "ask"})
	asked, askedBody := postJSON(t, r, http.MethodPost, "/v1/sites/register", map[string]any{"domain": "signup-ask.example", "name": "Ask"}, map[string]string{"X-API-Key": key})
	if asked.Code != http.StatusAccepted || askedBody["reason"] != "step_ask" || askedBody["approval_id"] == "" || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='signup-ask.example'`) != 0 {
		t.Fatalf("ask register: %d %s", asked.Code, asked.Body.String())
	}
	queue, queueBody := postJSON(t, r, http.MethodGet, "/v1/offer-review/queue", nil, map[string]string{"X-Admin-Key": "test-admin"})
	waiting, _ := queueBody["step_approvals"].([]any)
	if queue.Code != http.StatusOK || len(waiting) != 1 {
		t.Fatalf("queue: %d %s", queue.Code, queue.Body.String())
	}
	approved := approveStep(t, r, askedBody["approval_id"].(string))
	result, _ := approved["result"].(map[string]any)
	if result["site_id"] == "" || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='signup-ask.example'`) != 1 {
		t.Fatalf("approved signup: %v", approved)
	}
	account, accountBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Waiting", "email": "signup-gate@example.com", "domain": "signup-waiting.example",
	}, nil)
	if account.Code != http.StatusAccepted || accountBody["step"] != "site_signup" || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='signup-waiting.example'`) != 0 {
		t.Fatalf("ask account: %d %s", account.Code, account.Body.String())
	}
	accountApproved := approveStep(t, r, accountBody["approval_id"].(string))
	if accountApproved["result"].(map[string]any)["site_id"] == "" || countRows(t, db, `SELECT COUNT(*) FROM sites WHERE domain='signup-waiting.example'`) != 1 {
		t.Fatalf("approved account: %v", accountApproved)
	}
}

func TestOfferMatchAskAndOff(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, _, key := signupPublisher(t, r, "match-gate@example.com", "match-gate.example")
	_, offerID := seedListedOffer(t, db, "match-maker-key", "match-maker@example.com", "CodeLens")
	payload := map[string]any{"context": stepArticle("CodeLens")}

	matched, matchedBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", payload, map[string]string{"X-API-Key": key})
	items, _ := matchedBody["opportunities"].([]any)
	if matched.Code != http.StatusOK || matchedBody["status"] != "matched" || len(items) == 0 || items[0].(map[string]any)["offer_id"] != offerID {
		t.Fatalf("auto match: %d %s", matched.Code, matched.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"offer_match": "ask"})
	asked, askedBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", payload, map[string]string{"X-API-Key": key})
	if asked.Code != http.StatusAccepted || askedBody["reason"] != "step_ask" || askedBody["opportunities"] != nil {
		t.Fatalf("ask match: %d %s", asked.Code, asked.Body.String())
	}
	approved := approveStep(t, r, askedBody["approval_id"].(string))
	found, _ := approved["result"].(map[string]any)["opportunities"].([]any)
	if len(found) == 0 || found[0].(map[string]any)["offer_id"] != offerID {
		t.Fatalf("approved match: %v", approved)
	}

	putStepModes(t, r, key, map[string]any{"offer_match": "off"})
	off, offBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", payload, map[string]string{"X-API-Key": key})
	if off.Code != http.StatusForbidden || offBody["reason"] != "step_off" || offBody["step"] != "offer_match" || !strings.Contains(offBody["message"].(string), "Offer match is off") {
		t.Fatalf("off match: %d %s", off.Code, off.Body.String())
	}
}

func TestMCPStepModesReturnOffReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, api := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, _, key := signupPublisher(t, api, "mcp-modes@example.com", "mcp-modes.example")
	_, _ = seedListedOffer(t, db, "mcp-maker-key", "mcp-maker@example.com", "CodeLens")
	server := httptest.NewServer(api)
	defer server.Close()
	gin.SetMode(gin.TestMode)
	mcpRouter := gin.New()
	mcpRouter.POST("/v1/mcp", (&MCPHandler{LoopbackURL: server.URL}).Receive)

	call := func(id int, name string, args map[string]any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(string(raw)))
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		mcpRouter.ServeHTTP(rec, req)
		var body map[string]any
		if err = json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("mcp %s: %d %s", name, rec.Code, rec.Body.String())
		}
		return body["result"].(map[string]any)
	}

	set := call(1, "set_step_modes", map[string]any{"offer_match": "off"})
	if set["isError"] != false || !strings.Contains(set["content"].([]any)[0].(map[string]any)["text"].(string), `"offer_match": "off"`) {
		t.Fatalf("set: %v", set)
	}
	got := call(2, "get_step_modes", map[string]any{})
	if got["isError"] != false || !strings.Contains(got["content"].([]any)[0].(map[string]any)["text"].(string), `"offer_match": "off"`) {
		t.Fatalf("get: %v", got)
	}
	found := call(3, "find_opportunities", map[string]any{"context": stepArticle("CodeLens")})
	text := found["content"].([]any)[0].(map[string]any)["text"].(string)
	if found["isError"] != true || !strings.Contains(text, "step_off:") || !strings.Contains(text, "Offer match is off") {
		t.Fatalf("find: %v", found)
	}
}

func TestPlacementPublishModesDomainAndClaims(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, live := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, live, "publish-gate@example.com", "publish-gate.example")
	_, offerID := seedListedOffer(t, db, "publish-maker-key", "publish-maker@example.com", "CodeLens")
	surface := "https://publish-gate.example/posts/review"

	putStepModes(t, live, key, map[string]any{"placement_publish": "off"})
	offRec, offBody := preparePlacement(t, live, key, offerID, surface, 1)
	if offRec.Code != http.StatusForbidden || offBody["reason"] != "step_off" || countRows(t, db, `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1`, siteID) != 0 {
		t.Fatalf("off publish: %d %s", offRec.Code, offRec.Body.String())
	}

	putStepModes(t, live, key, map[string]any{"placement_publish": "auto"})
	unverified, unverifiedBody := preparePlacement(t, live, key, offerID, surface, 2)
	placement := placementBody(t, unverifiedBody)
	if unverified.Code != http.StatusCreated || unverifiedBody["status"] != "awaiting_approval" || unverifiedBody["reason"] != "domain_unverified" || placement["status"] != "pending" || !strings.Contains(unverifiedBody["next_action"].(string), "operator") {
		t.Fatalf("unverified: %d %s", unverified.Code, unverified.Body.String())
	}

	verifySite(t, db, siteID, "publish-gate.example")
	putStepModes(t, live, key, map[string]any{"placement_publish": "ask"})
	asked, askedBody := preparePlacement(t, live, key, offerID, "https://publish-gate.example/posts/asked", 3)
	askedPlacement := placementBody(t, askedBody)
	if asked.Code != http.StatusCreated || askedBody["reason"] != "step_ask" || askedPlacement["status"] != "pending" || askedPlacement["approved_by"] != nil && askedPlacement["approved_by"] != "" {
		t.Fatalf("ask publish: %d %s", asked.Code, asked.Body.String())
	}

	_, stub := newStepModeRouter(db, entry.StubClaimChecker{})
	putStepModes(t, stub, key, map[string]any{"placement_publish": "auto"})
	held, heldBody := preparePlacement(t, stub, key, offerID, "https://publish-gate.example/posts/claims", 4)
	if held.Code != http.StatusCreated || heldBody["reason"] != "claim_check_unavailable" || heldBody["message"] != "Automatic approval is off until claim checks are live." || placementBody(t, heldBody)["status"] != "pending" {
		t.Fatalf("claims hold: %d %s", held.Code, held.Body.String())
	}

	published, publishedBody := preparePlacement(t, live, key, offerID, "https://publish-gate.example/posts/live", 5)
	livePlacement := placementBody(t, publishedBody)
	if published.Code != http.StatusCreated || publishedBody["status"] != "published" || livePlacement["status"] != "committed" || livePlacement["approved_by"] != "policy:auto" || livePlacement["matched_rule"] != "hard_checks" {
		t.Fatalf("auto publish: %d %s", published.Code, published.Body.String())
	}
}

func TestAskToAutoPublishesOnlyTheNextPlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, r, "release-next@example.com", "release-next.example")
	verifySite(t, db, siteID, "release-next.example")
	_, offerID := seedListedOffer(t, db, "release-maker-key", "release-maker@example.com", "CodeLens")
	putStepModes(t, r, key, map[string]any{"placement_publish": "ask", "offer_match": "auto"})

	matched, matchedBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", map[string]any{"context": stepArticle("CodeLens")}, map[string]string{"X-API-Key": key})
	if matched.Code != http.StatusOK || matchedBody["status"] != "matched" {
		t.Fatalf("match while publish asks: %d %s", matched.Code, matched.Body.String())
	}
	waiting, waitingBody := preparePlacement(t, r, key, offerID, "https://release-next.example/posts/waiting", 7)
	waitingID := placementBody(t, waitingBody)["id"].(string)
	if waiting.Code != http.StatusCreated || waitingBody["status"] != "awaiting_approval" || waitingBody["reason"] != "step_ask" {
		t.Fatalf("waiting publish: %d %s", waiting.Code, waiting.Body.String())
	}

	putStepModes(t, r, key, map[string]any{"placement_publish": "auto"})
	var still string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, waitingID).Scan(&still); err != nil || still != "pending" {
		t.Fatalf("queued placement status %s %v", still, err)
	}
	next, nextBody := preparePlacement(t, r, key, offerID, "https://release-next.example/posts/next", 8)
	nextPlacement := placementBody(t, nextBody)
	if next.Code != http.StatusCreated || nextBody["status"] != "published" || nextPlacement["status"] != "committed" || nextPlacement["approved_by"] != "policy:auto" || nextPlacement["id"] == waitingID {
		t.Fatalf("next publish: %d %s", next.Code, next.Body.String())
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, waitingID).Scan(&still); err != nil || still != "pending" {
		t.Fatalf("first placement after auto %s %v", still, err)
	}
}
