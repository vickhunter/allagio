package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/services"
)

func TestOwnerPagesRenderCSRFMeta(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	owner, cookie := testOwnerSession(t, h, "csrf-meta-owner", "csrf-meta@example.com")
	pages := []string{"/owner", "/owner/settings", "/owner/products", "/owner/placements", "/owner/earnings"}
	assertCSRFMeta := func(path string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		body := rec.Body.String()
		meta := `<meta name="csrf-token" content="` + owner.CSRF + `">`
		if rec.Code != http.StatusOK || !strings.Contains(body, meta) {
			t.Fatalf("%s csrf meta: %d %s", path, rec.Code, body)
		}
		if strings.Contains(body, "window.csrf") || strings.Contains(body, "window.CSRF") || strings.Contains(body, `window["csrf`) || strings.Contains(body, "window['csrf") {
			t.Fatalf("%s set a csrf window global", path)
		}
	}
	for _, path := range pages {
		assertCSRFMeta(path)
	}
	h.AssetsPath = "../../../affilio-wp/admin/build"
	if err := h.LoadWorkspace(); err != nil {
		t.Fatal(err)
	}
	for _, path := range pages {
		assertCSRFMeta(path)
	}
}

func TestSetupReadDefaultsInstallPath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "setup-default-owner", "setup-default@example.com")
	siteID := createTestSite(t, db, "setup-default-site", "setup-default.example", "Setup Default")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='cursor' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	if rec.Code != http.StatusOK || body["install_path"] != "cursor" || body["saved"] != false || body["mode_preselected"] != "yolo" {
		t.Fatalf("default read: %d %s", rec.Code, rec.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher&install_path=cli", nil)
	if rec.Code != http.StatusOK || body["install_path"] != "cli" || body["mode_preselected"] != "yolo" {
		t.Fatalf("explicit path: %d %s", rec.Code, rec.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", map[string]any{
		"role": "publisher", "mode": "yolo", "mode_shown": "yolo",
	})
	if rec.Code != http.StatusOK || body["install_path"] != "cursor" || body["saved"] != true {
		t.Fatalf("first save: %d %s", rec.Code, rec.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT install_path FROM owner_setup WHERE owner_id=$1 AND role='publisher'`, owner.ID).Scan(&stored); err != nil || stored != "cursor" {
		t.Fatalf("stored path %s %v", stored, err)
	}
	rec, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", map[string]any{
		"role": "publisher", "install_path": "nope", "mode": "yolo", "mode_shown": "yolo",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid path: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET source='chatgpt' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=maker", nil)
	if rec.Code != http.StatusOK || body["install_path"] != "chatgpt_dir" || body["mode_preselected"] != "manual" || body["saved"] != false {
		t.Fatalf("chatgpt default: %d %s", rec.Code, rec.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	if rec.Code != http.StatusOK || body["install_path"] != "cursor" || body["saved"] != true {
		t.Fatalf("saved path stayed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPluginKeyErrorsIncludeReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "plugin-reason-owner", "plugin-reason@example.com")
	siteID := createTestSite(t, db, "plugin-reason-site", "plugin-reason.example", "Plugin Reason")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, _ = seedListedOffer(t, db, "plugin-reason-maker", "plugin-reason-maker@example.com", "Plugin Reason Product")
	projectKey := "or_plugin_reason_project"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_reas',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	created, createdBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account-keys", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("account key: %d %s", created.Code, created.Body.String())
	}
	accountKey := createdBody["key"].(string)
	paths := []string{
		"/v1/sites/" + siteID + "/plugin-key",
		"/v1/sites/" + siteID + "/plugin-key/rotate",
		"/v1/sites/" + siteID + "/plugin-key/revoke",
	}
	assertReason := func(rec *httptest.ResponseRecorder, body map[string]any, status int, reason, message, compat string) {
		t.Helper()
		if rec.Code != status || body["reason"] != reason || body["message"] != message || body["error"] != compat {
			t.Fatalf("reason %s: %d %s", reason, rec.Code, rec.Body.String())
		}
	}
	for _, path := range paths {
		method := http.MethodPost
		if path == "/v1/sites/"+siteID+"/plugin-key" {
			rec, body := postJSON(t, r, http.MethodGet, path, nil, nil)
			assertReason(rec, body, http.StatusUnauthorized, "reauth_required", "Sign in again to manage plugin keys.", "sign in at /auth/login")
		}
		rec, body := postJSON(t, r, method, path, nil, nil)
		assertReason(rec, body, http.StatusUnauthorized, "reauth_required", "Sign in again to manage plugin keys.", "sign in at /auth/login")
	}
	callers := []map[string]string{
		{"X-API-Key": "plugin-reason-site"},
		{"X-API-Key": projectKey},
		{"X-API-Key": accountKey},
		{"X-Maker-Key": "plugin-reason-maker"},
		{"Authorization": "Bearer " + accountKey},
	}
	for _, path := range paths {
		for _, headers := range callers {
			headers["Origin"] = h.BaseURL
			rec, body := postJSON(t, r, http.MethodPost, path, nil, headers)
			assertReason(rec, body, http.StatusForbidden, "forbidden_key_type", "A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.", pluginKeySessionError)
		}
		cookieHeaders := map[string]string{"X-API-Key": "plugin-reason-site", "Origin": h.BaseURL, "X-CSRF-Token": owner.CSRF}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		for key, value := range cookieHeaders {
			req.Header.Set(key, value)
		}
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		assertReason(rec, body, http.StatusForbidden, "forbidden_key_type", "A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.", pluginKeySessionError)
	}
	listed, listedBody := postJSON(t, r, http.MethodGet, "/v1/sites/"+siteID+"/plugin-key", nil, map[string]string{"X-API-Key": "plugin-reason-site"})
	assertReason(listed, listedBody, http.StatusForbidden, "forbidden_key_type", "A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.", pluginKeySessionError)
	other := createTestSite(t, db, "plugin-reason-other", "plugin-reason-other.example", "Other")
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+other+"/plugin-key", nil)
	assertReason(rec, body, http.StatusForbidden, "forbidden", pluginKeySessionError, pluginKeySessionError)
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/not-a-uuid/plugin-key", nil)
	assertReason(rec, body, http.StatusBadRequest, "invalid_site_id", "The site id is not valid.", "invalid site id")

	issued, issuedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key", nil)
	if issued.Code != http.StatusCreated || issuedBody["plugin_key"] == "" {
		t.Fatalf("issue: %d %s", issued.Code, issued.Body.String())
	}
	again, againBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/sites/"+siteID+"/plugin-key", nil)
	assertReason(again, againBody, http.StatusConflict, "key_exists", "A plugin key already exists.", "a plugin key already exists")

	if _, err := db.Pool.Exec(context.Background(), `UPDATE owner_sessions SET expires_at = NOW() + INTERVAL '8 hours' - INTERVAL '16 minutes' WHERE token_hash=$1`, services.HashAPIKey(cookie.Value)); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if path == "/v1/sites/"+siteID+"/plugin-key" {
			rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, path, nil)
			assertReason(rec, body, http.StatusUnauthorized, "reauth_required", "This session is too old. Sign in again to manage plugin keys.", "sign in at /auth/login")
		}
		rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, path, nil)
		assertReason(rec, body, http.StatusUnauthorized, "reauth_required", "This session is too old. Sign in again to manage plugin keys.", "sign in at /auth/login")
	}
}

func TestDigestItemsIncludeUndoneBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "undone-by-owner", "undone-by@example.com")
	siteID := createTestSite(t, db, "undone-by-site", "undone-by.example", "Undone By")
	makerID, offerID := seedListedOffer(t, db, "undone-by-maker", "undone-by-maker@example.com", "Undone Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	liveID := insertPlacement(t, db, siteID, offerID, "https://undone-by.example/live", "Undone Product live.")
	undoneID := insertPlacement(t, db, siteID, offerID, "https://undone-by.example/undone", "Undone Product undone.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', approved_at=NOW(), approved_by_kind='owner' WHERE id=$1`, liveID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='remove', approved_at=NOW(), undone_by_user=$2 WHERE id=$1`, undoneID, owner.ID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	var live, undone, offer map[string]any
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		switch item["id"] {
		case liveID:
			live = item
		case undoneID:
			undone = item
		case offerID:
			offer = item
		}
	}
	if live == nil || undone == nil || offer == nil {
		t.Fatalf("items: %s", rec.Body.String())
	}
	if live["undone_by"] != "" || undone["undone_by"] != owner.ID || offer["undone_by"] != "" {
		t.Fatalf("undone_by live=%v undone=%v offer=%v", live["undone_by"], undone["undone_by"], offer["undone_by"])
	}
	accountRec, account := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if accountRec.Code != http.StatusOK {
		t.Fatalf("account digest: %d %s", accountRec.Code, accountRec.Body.String())
	}
	found := false
	for _, raw := range account["projects"].([]any) {
		project := raw.(map[string]any)
		items, _ := project["items"].([]any)
		for _, itemRaw := range items {
			item := itemRaw.(map[string]any)
			if item["id"] == undoneID && item["undone_by"] == owner.ID {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("account digest missing undone_by: %s", accountRec.Body.String())
	}
}
