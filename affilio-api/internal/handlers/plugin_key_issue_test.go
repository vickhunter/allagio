package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
)

func TestIssuedPluginKeyAcksAndCannotBeReadBack(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	r.POST("/v1/sites/register", (&SitesHandler{DB: db}).Register)
	installation := strings.Repeat("ab", 32)
	registered, created := postJSON(t, r, http.MethodPost, "/v1/sites/register", map[string]any{
		"domain": "plugin-issue.example", "name": "Plugin", "installation_key": installation,
	}, nil)
	if registered.Code != http.StatusOK || created["api_key"] != installation || created["site_id"] == "" || created["plugin_key"] == "" {
		t.Fatalf("register: %d %s", registered.Code, registered.Body.String())
	}
	siteID := created["site_id"].(string)
	raw := created["plugin_key"].(string)
	if strings.Contains(created["api_key"].(string), raw) && raw == installation {
		t.Fatal("plugin key matched the site key")
	}
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/sites/register", map[string]any{
		"domain": "plugin-issue.example", "installation_key": installation,
	}, nil)
	if again.Code != http.StatusOK || againBody["plugin_key"] != nil || againBody["api_key"] != installation {
		t.Fatalf("re-register revealed a plugin key: %d %s", again.Code, again.Body.String())
	}

	owner, cookie := testOwnerSession(t, h, "plugin-issue-owner", "plugin-issue@example.com")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "plugin-issue-maker", "plugin-issue-maker@example.com", "Plugin Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://plugin-issue.example/post", "Plugin Product note.")
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": raw})
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("issued key ack: %d %s", acked.Code, acked.Body.String())
	}
	listed, listedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/sites/"+siteID+"/plugin-key", nil)
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), raw) {
		t.Fatalf("raw key came back: %d %s", listed.Code, listed.Body.String())
	}
	keys, _ := listedBody["keys"].([]any)
	if len(keys) != 1 || keys[0].(map[string]any)["key_prefix"] != services.KeyPrefix(raw) {
		t.Fatalf("prefix list: %s", listed.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT key_hash FROM plugin_keys WHERE site_id=$1 AND revoked_at IS NULL`, siteID).Scan(&stored); err != nil || stored != services.HashAPIKey(raw) || stored == raw {
		t.Fatalf("stored hash %s %v", stored, err)
	}

	connectHost, connectRouter, _ := hostedTestServer(t, db)
	connectSite := createTestSite(t, db, "connect-plugin-key", "connect-plugin.example", "Connect")
	group := connectRouter.Group("/v1-connect", middleware.APIKeyAuth(db, nil, connectHost.resource()))
	group.POST("/sites/owner-connection", connectHost.StartSiteConnection)
	started := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1-connect/sites/owner-connection", strings.NewReader(`{}`))
	req.Header.Set("X-API-Key", "connect-plugin-key")
	connectRouter.ServeHTTP(started, req)
	if started.Code != http.StatusCreated {
		t.Fatalf("connect start: %d %s", started.Code, started.Body.String())
	}
	var startBody map[string]any
	if err := json.Unmarshal(started.Body.Bytes(), &startBody); err != nil {
		t.Fatal(err)
	}
	connectURL, _ := url.Parse(startBody["connect_url"].(string))
	connectOwner, connectCookie := testOwnerSession(t, connectHost, "connect-plugin-owner", "connect-plugin@example.com")
	completed := ownerForm(t, connectHost, connectRouter, connectCookie, "/owner/connect-site", url.Values{"csrf": {connectOwner.CSRF}, "token": {connectURL.Query().Get("token")}})
	if completed.Code != http.StatusOK || !strings.Contains(completed.Body.String(), "shown once") {
		t.Fatalf("connect page: %d %s", completed.Code, completed.Body.String())
	}
	found := regexp.MustCompile(`<code>([0-9a-f]{48})</code>`).FindStringSubmatch(completed.Body.String())
	if len(found) != 2 {
		t.Fatalf("connect key missing: %s", completed.Body.String())
	}
	connectPlacement := insertPlacement(t, db, connectSite, offerID, "https://connect-plugin.example/post", "Plugin Product note.")
	connectAck, connectAckBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+connectPlacement+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": found[1]})
	if connectAck.Code != http.StatusOK || connectAckBody["plugin_state"] != "placed" {
		t.Fatalf("connect key ack: %d %s", connectAck.Code, connectAck.Body.String())
	}
}

func TestPluginKeyRotateRevokeGenerateSessionOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "plugin-manage-owner", "plugin-manage@example.com")
	siteID := createTestSite(t, db, "plugin-manage-site", "plugin-manage.example", "Manage")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "plugin-manage-maker", "plugin-manage-maker@example.com", "Manage Product")
	projectKey := "or_plugin_manage_project"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_plug',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	created, createdBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account-keys", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("account key: %d %s", created.Code, created.Body.String())
	}
	accountKey := createdBody["key"].(string)
	paths := []string{"/v1/sites/" + siteID + "/plugin-key", "/v1/sites/" + siteID + "/plugin-key/rotate", "/v1/sites/" + siteID + "/plugin-key/revoke"}
	callers := []map[string]string{
		{"X-API-Key": "plugin-manage-site"},
		{"X-API-Key": projectKey},
		{"X-API-Key": accountKey},
		{"X-Maker-Key": "plugin-manage-maker"},
		{"Authorization": "Bearer " + accountKey},
	}
	for _, path := range paths {
		for _, headers := range callers {
			rec, body := postJSON(t, r, http.MethodPost, path, nil, headers)
			if rec.Code != http.StatusForbidden || body["plugin_key"] != nil {
				t.Fatalf("non-session %s %v: %d %s", path, headers, rec.Code, rec.Body.String())
			}
		}
	}

	issued, issuedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key", nil)
	if issued.Code != http.StatusCreated || issuedBody["plugin_key"] == "" {
		t.Fatalf("generate: %d %s", issued.Code, issued.Body.String())
	}
	raw := issuedBody["plugin_key"].(string)
	again, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key", nil)
	if again.Code != http.StatusConflict || strings.Contains(again.Body.String(), raw) {
		t.Fatalf("second generate: %d %s", again.Code, again.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://plugin-manage.example/post", "Manage Product note.")
	acked, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": raw})
	if acked.Code != http.StatusOK {
		t.Fatalf("generated key ack: %d %s", acked.Code, acked.Body.String())
	}
	rotated, rotatedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key/rotate", nil)
	if rotated.Code != http.StatusOK || rotatedBody["plugin_key"] == "" || rotatedBody["plugin_key"] == raw {
		t.Fatalf("rotate: %d %s", rotated.Code, rotated.Body.String())
	}
	next := rotatedBody["plugin_key"].(string)
	secondID := insertPlacement(t, db, siteID, offerID, "https://plugin-manage.example/next", "Manage Product note.")
	nextAck, nextBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+secondID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": next})
	if nextAck.Code != http.StatusOK || nextBody["plugin_state"] != "placed" {
		t.Fatalf("rotated key ack: %d %s", nextAck.Code, nextAck.Body.String())
	}
	oldAck, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": raw})
	if oldAck.Code != http.StatusForbidden {
		t.Fatalf("rotated-out key: %d %s", oldAck.Code, oldAck.Body.String())
	}
	revoked, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key/revoke", nil)
	if revoked.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", revoked.Code, revoked.Body.String())
	}
	newAck, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": next})
	if newAck.Code != http.StatusForbidden {
		t.Fatalf("revoked key: %d %s", newAck.Code, newAck.Body.String())
	}
	listed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/sites/"+siteID+"/plugin-key", nil)
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), raw) || strings.Contains(listed.Body.String(), next) {
		t.Fatalf("secrets listed: %d %s", listed.Code, listed.Body.String())
	}
}

func TestSiteAndMakerKeysCannotDecideOrRestore(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "gate-owner", "gate-owner@example.com")
	siteID := createTestSite(t, db, "gate-site-key", "gate.example", "Gate")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "gate-maker-key", "gate-maker@example.com", "Gate Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://gate.example/post", "Gate Product note.")
	payload := map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true}
	for _, headers := range []map[string]string{{"X-API-Key": "gate-site-key"}, {"X-Maker-Key": "gate-maker-key"}} {
		decided, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/decide", payload, headers)
		if decided.Code != http.StatusForbidden {
			t.Fatalf("decide %v: %d %s", headers, decided.Code, decided.Body.String())
		}
	}
	approved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", payload)
	if approved.Code != http.StatusOK {
		t.Fatalf("session decide: %d %s", approved.Code, approved.Body.String())
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, map[string]string{"X-API-Key": "gate-site-key"})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	for _, headers := range []map[string]string{{"X-API-Key": "gate-site-key"}, {"X-Maker-Key": "gate-maker-key"}} {
		restored, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil, headers)
		if restored.Code != http.StatusForbidden || body["error"] != "decide and restore require a signed-in dashboard session" {
			t.Fatalf("restore %v: %d %s", headers, restored.Code, restored.Body.String())
		}
	}
}

func TestRestoreAndRetryBlockedWhenClaimsFail(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	current := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }
	owner, cookie := testOwnerSession(t, h, "claims-owner", "claims-owner@example.com")
	siteID := createTestSite(t, db, "claims-site-key", "claims.example", "Claims")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "claims-maker", "claims-maker@example.com", "Claims Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://claims.example/post", "Claims Product is a solid fit.")
	approved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	if approved.Code != http.StatusOK {
		t.Fatalf("decide: %d %s", approved.Code, approved.Body.String())
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, map[string]string{"X-API-Key": "claims-site-key"})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	api.Claims = rejectingClaims{}
	current = current.Add(time.Second)
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "claims_from_offer" {
		t.Fatalf("restore claims: %d %s", blocked.Code, blocked.Body.String())
	}
	var status, wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&status, &wait); err != nil || status != "revoked" || wait == "restore" {
		t.Fatalf("restore wrote through a claim failure: %s %s %v", status, wait, err)
	}

	api.Claims = entry.LiveClaimChecker{}
	current = current.Add(time.Second)
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	pluginKey := issuePluginKey(t, db, siteID, "claims-plugin-key")
	failed, failedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-read failed"}, pluginKey)
	if failed.Code != http.StatusOK || failedBody["failed_action"] != "restored" {
		t.Fatalf("fail ack: %d %s", failed.Code, failed.Body.String())
	}
	api.Claims = rejectingClaims{}
	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, map[string]string{"X-API-Key": "claims-site-key"})
	if retried.Code != http.StatusUnprocessableEntity || retriedBody["check_id"] != "claims_from_offer" {
		t.Fatalf("retry claims: %d %s", retried.Code, retried.Body.String())
	}
	var action, reason string
	if err := db.Pool.QueryRow(context.Background(), `SELECT failed_action, plugin_failed_reason FROM placement_proposals WHERE id=$1`, placementID).Scan(&action, &reason); err != nil || action != "restored" || reason != "re-read failed" {
		t.Fatalf("retry wrote through a claim failure: %s %s %v", action, reason, err)
	}
}
