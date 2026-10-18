package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func newD063Router(t *testing.T, db *database.DB) (*HostedHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &HostedHandler{
		DB:      db,
		BaseURL: "http://127.0.0.1:8080",
		Login:   &fakeOwnerLogin{owner: database.Owner{Issuer: "https://identity.example", Subject: "device-human", Email: "human@example.com", Name: "Human"}},
	}
	h.Mount(r)
	api := &AgentEntry{DB: db, BaseURL: h.BaseURL, Owner: h.RequireOwnerSession, Claims: entry.LiveClaimChecker{}}
	api.Mount(r)
	placements := &PlacementsHandler{DB: db, Claims: entry.LiveClaimChecker{}}
	secured := r.Group("/v1", middleware.APIKeyAuth(db, nil))
	secured.POST("/placements", placements.Prepare)
	secured.POST("/placements/:id/commit", placements.Commit)
	secured.POST("/placements/:id/receipt", placements.RecordReceipt)
	admin := r.Group("/v1/offer-review", middleware.AdminKeyAuth("test-admin", nil))
	admin.POST("/placements/:id/approve", placements.Approve)
	redirects := &RedirectsHandler{DB: db, HomeURL: "https://offerrails.example/", IPSaltSecret: "d063-salt"}
	r.GET("/v1/go/:code", redirects.Go)
	return h, r
}

func TestDevicePageSendsSignedOutHumansToSignIn(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newD063Router(t, db)

	plain := httptest.NewRecorder()
	r.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/device", nil))
	if plain.Code != http.StatusFound || plain.Header().Get("Location") != "/auth/login?return_to=%2Fdevice" {
		t.Fatalf("signed out: %d %s", plain.Code, plain.Header().Get("Location"))
	}

	coded := httptest.NewRecorder()
	r.ServeHTTP(coded, httptest.NewRequest(http.MethodGet, "/device?user_code=ABCD-EFGH", nil))
	if coded.Code != http.StatusFound || coded.Header().Get("Location") != "/auth/login?return_to=%2Fdevice%3Fuser_code%3DABCD-EFGH" {
		t.Fatalf("coded return: %d %s", coded.Code, coded.Header().Get("Location"))
	}

	stripped := httptest.NewRecorder()
	r.ServeHTTP(stripped, httptest.NewRequest(http.MethodGet, "/device?user_code=not-a-code&next=https://evil.example", nil))
	if stripped.Code != http.StatusFound || stripped.Header().Get("Location") != "/auth/login?return_to=%2Fdevice" {
		t.Fatalf("stripped return: %d %s", stripped.Code, stripped.Header().Get("Location"))
	}

	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodGet, coded.Header().Get("Location"), nil))
	if login.Code != http.StatusFound || !strings.Contains(login.Header().Get("Location"), "https://identity.example/authorize") {
		t.Fatalf("login accepted the device return: %d %s", login.Code, login.Body.String())
	}
	for _, raw := range []string{"/device?user_code=not-a-code", "/device?user_code=ABCD-EFGH&next=https://evil.example", "https://evil.example", "/device?next=%ZZ", "/device?user_code=ABCD-EFGH&next=%ZZ"} {
		bad := httptest.NewRecorder()
		r.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/auth/login?return_to="+url.QueryEscape(raw), nil))
		if bad.Code != http.StatusBadRequest {
			t.Fatalf("return %s: %d %s", raw, bad.Code, bad.Body.String())
		}
	}
	sourced := httptest.NewRecorder()
	r.ServeHTTP(sourced, httptest.NewRequest(http.MethodGet, "/auth/login?return_to=%2Fdevice&source=cli&ref=partner&via=click", nil))
	if sourced.Code != http.StatusFound || !strings.Contains(sourced.Header().Get("Location"), "https://identity.example/authorize") {
		t.Fatalf("device login with source: %d %s", sourced.Code, sourced.Body.String())
	}
	rows, err := db.Pool.Query(context.Background(), `SELECT return_to FROM owner_login_attempts`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var stored string
		if err = rows.Scan(&stored); err != nil {
			t.Fatal(err)
		}
		seen++
		if stored != "/device" && stored != "/device?user_code=ABCD-EFGH" {
			t.Fatalf("stored device return %s", stored)
		}
	}
	if err = rows.Err(); err != nil || seen == 0 {
		t.Fatalf("stored returns %d %v", seen, err)
	}

	owner, cookie := testOwnerSession(t, h, "device-page-owner", "device-page@example.com")
	siteID := createTestSite(t, db, "device-page-site", "device-page.example", "Device Blog")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	pageReq := httptest.NewRequest(http.MethodGet, "/device?user_code=ABCD-EFGH", nil)
	pageReq.AddCookie(cookie)
	r.ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `action="/device"`) || !strings.Contains(page.Body.String(), "ABCD-EFGH") || !strings.Contains(page.Body.String(), "site:"+siteID) {
		t.Fatalf("signed-in page: %d %s", page.Code, page.Body.String())
	}

	anonymous := ownerForm(t, h, r, nil, "/device", url.Values{"user_code": {"ABCD-EFGH"}, "decision": {"approve"}, "project": {"site:" + siteID}})
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous post: %d %s", anonymous.Code, anonymous.Body.String())
	}

	_, codeBody := postJSON(t, r, http.MethodPost, "/v1/device/code", map[string]any{}, nil)
	userCode := codeBody["user_code"].(string)
	deviceCode := codeBody["device_code"].(string)
	if codeBody["verification_uri"] != h.BaseURL+"/device" {
		t.Fatalf("verification uri: %v", codeBody["verification_uri"])
	}
	other := createTestSite(t, db, "device-other-site", "other-device.example", "Other")
	wrong := ownerForm(t, h, r, cookie, "/device", url.Values{"csrf": {owner.CSRF}, "user_code": {userCode}, "decision": {"approve"}, "project": {"site:" + other}})
	if wrong.Code != http.StatusForbidden || !strings.Contains(wrong.Body.String(), "That project is not on this account.") {
		t.Fatalf("wrong project: %d %s", wrong.Code, wrong.Body.String())
	}
	approved := ownerForm(t, h, r, cookie, "/device", url.Values{"csrf": {owner.CSRF}, "user_code": {userCode}, "decision": {"approve"}, "project": {"site:" + siteID}})
	if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), "Approved. The agent can pick up its project key.") {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	tokenRec, token := postJSON(t, r, http.MethodPost, "/v1/device/token", map[string]any{"device_code": deviceCode}, nil)
	if tokenRec.Code != http.StatusOK || token["key_header"] != "X-API-Key" || token["scope"] != "site:"+siteID || token["access_token"] == "" {
		t.Fatalf("token: %d %s", tokenRec.Code, tokenRec.Body.String())
	}
	again := ownerForm(t, h, r, cookie, "/device", url.Values{"csrf": {owner.CSRF}, "user_code": {userCode}, "decision": {"approve"}, "project": {"site:" + siteID}})
	if again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), "That device code was already used.") {
		t.Fatalf("reuse: %d %s", again.Code, again.Body.String())
	}

	_, denyCode := postJSON(t, r, http.MethodPost, "/v1/device/code", map[string]any{}, nil)
	denied := ownerForm(t, h, r, cookie, "/device", url.Values{"csrf": {owner.CSRF}, "user_code": {denyCode["user_code"].(string)}, "decision": {"deny"}})
	if denied.Code != http.StatusOK || !strings.Contains(denied.Body.String(), "Denied. The agent will not get a key.") {
		t.Fatalf("deny: %d %s", denied.Code, denied.Body.String())
	}
	deniedPoll, poll := postJSON(t, r, http.MethodPost, "/v1/device/token", map[string]any{"device_code": denyCode["device_code"]}, nil)
	if deniedPoll.Code != http.StatusBadRequest || poll["error"] != "access_denied" {
		t.Fatalf("denied poll: %d %s", deniedPoll.Code, deniedPoll.Body.String())
	}
}

func TestAgentAccountSiteClicksCarryVia(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Agent Path Blog", "email": "agent-path@example.com", "domain": "agent-path.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated || body["agent_only"] != true || body["owner_id"] == "" || body["site_id"] == "" || body["api_key"] == "" || body["plugin_key"] == "" || body["key_header"] != "X-API-Key" || body["plugin_key_header"] != "X-Plugin-Key" {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	ownerID := body["owner_id"].(string)
	siteID := body["site_id"].(string)
	siteKey := body["api_key"].(string)
	pluginKey := body["plugin_key"].(string)
	if pluginKey == siteKey {
		t.Fatal("plugin key matched the site key")
	}
	var linked string
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT site_id::text FROM owners WHERE id=$1`, ownerID).Scan(&linked); err != nil || linked != siteID {
		t.Fatalf("owner link %s %v", linked, err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT key_hash FROM plugin_keys WHERE site_id=$1 AND revoked_at IS NULL`, siteID).Scan(&stored); err != nil || stored != services.HashAPIKey(pluginKey) || stored == pluginKey {
		t.Fatalf("stored plugin hash %s %v", stored, err)
	}
	listed, _ := postJSON(t, r, http.MethodGet, "/v1/sites/"+siteID+"/plugin-key", nil, map[string]string{"X-API-Key": siteKey})
	if listed.Code != http.StatusForbidden || strings.Contains(listed.Body.String(), pluginKey) {
		t.Fatalf("site key listed the plugin key: %d %s", listed.Code, listed.Body.String())
	}
	_, makerBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "maker", "name": "Agent Path Maker", "email": "agent-path-maker@example.com",
	}, nil)
	if makerBody["plugin_key"] != nil || makerBody["maker_id"] == "" || makerBody["key_header"] != "X-Maker-Key" {
		t.Fatalf("maker: %v", makerBody)
	}
	managed, managedBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if managed.Code != http.StatusForbidden || managedBody["reason"] != "forbidden_key_type" || managedBody["plugin_key"] != nil {
		t.Fatalf("site key issued a plugin key: %d %s", managed.Code, managed.Body.String())
	}

	_, offerID := seedListedOffer(t, db, "agent-path-offer-maker", "agent-path-offer@example.com", "CodeLens")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	contextText := "CodeLens devtools reviews every pull request. CodeLens finds bugs before merge. This devtools note is about CodeLens."
	copy := "We use CodeLens to catch bugs before merge."
	surface := "https://agent-path.example/posts/review"
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": surface, "context": contextText, "copy": copy, "post_id": 42, "post_title": "Review notes",
	}, map[string]string{"X-API-Key": siteKey})
	if prepared.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %s", prepared.Code, prepared.Body.String())
	}
	placement, _ := preparedBody["placement"].(map[string]any)
	placementID, _ := placement["id"].(string)
	disclosure, _ := placement["disclosure"].(string)
	if placementID == "" || disclosure == "" || placement["copy"] != copy {
		t.Fatalf("placement: %s", prepared.Body.String())
	}
	siteAck, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-API-Key": siteKey})
	if siteAck.Code != http.StatusForbidden {
		t.Fatalf("site key ack: %d %s", siteAck.Code, siteAck.Body.String())
	}
	approved, approvedBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "agent-path@example.com", "evidence": "Operator approval for the agent owner.",
	}, map[string]string{"X-Admin-Key": "test-admin"})
	if approved.Code != http.StatusOK || approvedBody["status"] != "approved" {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", committed.Code, committed.Body.String())
	}
	published := "<p>" + copy + ` <a href="https://example.com/v1/go/` + placementID + `">CodeLens</a></p><p>` + disclosure + "</p>"
	receipt, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/receipt", map[string]any{
		"surface_url": surface, "published_text": published, "evidence": "Agent published the approved copy.", "post_title": "Review notes", "post_id": 42,
	}, map[string]string{"X-API-Key": siteKey})
	if receipt.Code != http.StatusOK {
		t.Fatalf("receipt: %d %s", receipt.Code, receipt.Body.String())
	}
	clicks := func() int {
		t.Helper()
		var n int
		if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, placementID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	pending := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	if pending.Code != http.StatusFound || pending.Header().Get("Location") != "https://example.com/buy" || clicks() != 0 {
		t.Fatalf("pending go: %d %s clicks %d", pending.Code, pending.Header().Get("Location"), clicks())
	}
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": pluginKey})
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("plugin ack: %d %s", acked.Code, ackBody)
	}
	live := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	location := live.Header().Get("Location")
	parsed, err := url.Parse(location)
	if live.Code != http.StatusFound || err != nil || parsed.Query().Get("via") == "" || clicks() != 1 {
		t.Fatalf("live go: %d %s clicks %d", live.Code, location, clicks())
	}
	var clickID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text FROM clicks WHERE placement_id=$1`, placementID).Scan(&clickID); err != nil || clickID != parsed.Query().Get("via") {
		t.Fatalf("via %s click %s err %v", parsed.Query().Get("via"), clickID, err)
	}
}
