package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRestorePauseAndLimits(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	current := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }

	owner, cookie := testOwnerSession(t, h, "limits-owner", "limits-owner@example.com")
	siteID := createTestSite(t, db, "limits-site-key", "limits.example", "Limits")
	makerID, offerID := seedListedOffer(t, db, "limits-maker-key", "limits-maker@example.com", "Limits Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("yolo: %d %s", saved.Code, saved.Body.String())
	}
	siteKey := map[string]string{"X-API-Key": "limits-site-key"}

	approve := func(surface string, relevance int) string {
		t.Helper()
		id := insertPlacement(t, db, siteID, offerID, surface, "Honest note about the product.")
		decided, decision := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/decide", map[string]any{"relevance": relevance, "position": "end", "rel_sponsored": true}, siteKey)
		if decided.Code != http.StatusOK || decision["approved_by"] != "policy:yolo" {
			t.Fatalf("approve %s: %d %s", surface, decided.Code, decided.Body.String())
		}
		return id
	}
	undo := func(id string) {
		t.Helper()
		revoked, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", map[string]any{"reason": "undo"}, siteKey)
		if revoked.Code != http.StatusOK || body["undo"] != true {
			t.Fatalf("undo %s: %d %s", id, revoked.Code, revoked.Body.String())
		}
	}
	statusOf := func(id string) string {
		t.Helper()
		var status string
		if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	hasFailure := func(body map[string]any, id string) bool {
		t.Helper()
		failures, _ := body["failures"].([]any)
		for _, failure := range failures {
			if failure == id {
				return true
			}
		}
		return false
	}

	pausedSite := approve("https://limits.example/site", 80)
	undo(pausedSite)
	paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID}, siteKey)
	if paused.Code != http.StatusOK {
		t.Fatalf("pause site: %d %s", paused.Code, paused.Body.String())
	}
	refused, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+pausedSite+"/restore", nil, siteKey)
	if refused.Code != http.StatusConflict || body["reason"] != "paused" {
		t.Fatalf("pause then restore: %d %s", refused.Code, refused.Body.String())
	}
	if statusOf(pausedSite) != "revoked" {
		t.Fatal("paused restore changed the placement")
	}
	resumed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "site", "id": siteID})
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume site: %d %s", resumed.Code, resumed.Body.String())
	}
	current = current.Add(time.Second)

	pausedOffer := approve("https://limits.example/offer", 80)
	undo(pausedOffer)
	paused, _ = postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "offer", "id": offerID}, siteKey)
	if paused.Code != http.StatusOK {
		t.Fatalf("pause offer: %d %s", paused.Code, paused.Body.String())
	}
	resumed, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "offer", "id": offerID})
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume offer: %d %s", resumed.Code, resumed.Body.String())
	}
	refused, body = postJSON(t, r, http.MethodPost, "/v1/placements/"+pausedOffer+"/restore", nil, siteKey)
	if refused.Code != http.StatusConflict || body["reason"] != "paused" || body["error"] != "restore refused" {
		t.Fatalf("pause history then restore: %d %s", refused.Code, refused.Body.String())
	}
	if statusOf(pausedOffer) != "revoked" {
		t.Fatal("resumed pause restored the placement")
	}
	var offerStatus string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM offers WHERE id=$1`, offerID).Scan(&offerStatus); err != nil || offerStatus != "listed" {
		t.Fatalf("account pause changed the offer status: %s %v", offerStatus, err)
	}
	current = current.Add(time.Second)

	low := approve("https://limits.example/relevance", 60)
	undo(low)
	raised, _ := postJSON(t, r, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "min_relevance": 80}, siteKey)
	if raised.Code != http.StatusOK {
		t.Fatalf("raise min relevance: %d %s", raised.Code, raised.Body.String())
	}
	denied, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+low+"/restore", nil, siteKey)
	if denied.Code != http.StatusUnprocessableEntity || body["decision"] != "denied" || !hasFailure(body, "below_min_relevance") || hasFailure(body, "placement_cap") {
		t.Fatalf("raised min relevance: %d %s", denied.Code, denied.Body.String())
	}
	if statusOf(low) != "revoked" {
		t.Fatal("min relevance failure restored the placement")
	}
	current = current.Add(time.Second)

	capped := approve("https://limits.example/cap", 90)
	undo(capped)
	lowered, _ := postJSON(t, r, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 0}, siteKey)
	if lowered.Code != http.StatusOK {
		t.Fatalf("lower cap: %d %s", lowered.Code, lowered.Body.String())
	}
	denied, body = postJSON(t, r, http.MethodPost, "/v1/placements/"+capped+"/restore", nil, siteKey)
	if denied.Code != http.StatusUnprocessableEntity || !hasFailure(body, "placement_cap") || hasFailure(body, "below_min_relevance") {
		t.Fatalf("lowered cap: %d %s", denied.Code, denied.Body.String())
	}
	if statusOf(capped) != "revoked" {
		t.Fatal("cap failure restored the placement")
	}
}

func TestRevokedGoLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	r.HEAD("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)

	owner, _ := testOwnerSession(t, h, "golink-owner", "golink-owner@example.com")
	siteID := createTestSite(t, db, "golink-site-key", "golink.example", "Go Link")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "golink-maker-key", "golink-maker@example.com", "Go Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://golink.example/post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'revoked-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', campaign_id=$2, expires_at=NOW()+INTERVAL '2 days', approved_by_kind='policy:yolo' WHERE id=$1`, placementID, campaignID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "golink-site-key"}
	path := "/v1/go/revoked-go?placement=" + placementID

	live := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	if live.Code != http.StatusFound || live.Header().Get("Location") == "" || !containsAll(live.Header().Get("Location"), "https://example.com", "via=") {
		t.Fatalf("live redirect: %d %s", live.Code, live.Header().Get("Location"))
	}
	if placementClickCount(t, db, placementID) != 1 {
		t.Fatal("live click was not recorded")
	}

	revoked, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if revoked.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", revoked.Code, revoked.Body.String())
	}
	for _, hit := range []struct {
		method string
		agent  string
	}{
		{http.MethodGet, "Mozilla/5.0"},
		{http.MethodHead, "Mozilla/5.0"},
		{http.MethodGet, "Twitterbot/1.0"},
	} {
		stopped := hitGo(t, r, hit.method, path, hit.agent)
		if stopped.Code != http.StatusGone || stopped.Header().Get("Location") != "" {
			t.Fatalf("%s %s still redirects: %d %s", hit.method, hit.agent, stopped.Code, stopped.Header().Get("Location"))
		}
		if hit.method == http.MethodGet && (bodyReason(t, stopped) != "revoked") {
			t.Fatalf("%s body: %s", hit.agent, stopped.Body.String())
		}
	}
	if placementClickCount(t, db, placementID) != 1 {
		t.Fatal("revoked link credited a click")
	}

	restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	again := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	if again.Code != http.StatusFound || !containsAll(again.Header().Get("Location"), "https://example.com", "via=") {
		t.Fatalf("restored redirect: %d %s", again.Code, again.Header().Get("Location"))
	}
	if placementClickCount(t, db, placementID) != 2 {
		t.Fatal("restored link did not credit a click")
	}
}

func TestPluginPlacementState(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "plugin-owner", "plugin-owner@example.com")
	siteID := createTestSite(t, db, "plugin-site-key", "plugin.example", "Plugin")
	makerID, offerID := seedListedOffer(t, db, "plugin-maker-key", "plugin-maker@example.com", "Plugin Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "plugin-site-key"}
	placementID := insertPlacement(t, db, siteID, offerID, "https://plugin.example/post", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}

	digestState := func() string {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
		}
		for _, raw := range body["items"].([]any) {
			item := raw.(map[string]any)
			if item["kind"] == "offer" && item["plugin_state"] != nil {
				t.Fatalf("offer has plugin_state: %s", rec.Body.String())
			}
		}
		return pluginStateOf(t, body, placementID)
	}
	if digestState() != "offline" {
		t.Fatal("a placement with no contact should be offline")
	}

	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	if digestState() != "pending" {
		t.Fatal("undo should be pending")
	}
	account, accountBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if account.Code != http.StatusOK || pluginStateOf(t, accountBody, placementID) != "pending" {
		t.Fatalf("dashboard pending: %d %s", account.Code, account.Body.String())
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait_at=NOW()-INTERVAL '11 minutes', plugin_contact_at=NULL WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	if digestState() != "offline" {
		t.Fatal("a stale unacked undo should be offline")
	}
	polled, _ := postJSON(t, r, http.MethodGet, "/v1/placements", nil, siteKey)
	if polled.Code != http.StatusOK {
		t.Fatalf("poll: %d %s", polled.Code, polled.Body.String())
	}
	if digestState() != "pending" {
		t.Fatal("a poll should bring a stale undo back to pending")
	}

	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "removed"}, siteKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "removed" || digestState() != "removed" {
		t.Fatalf("ack removed: %d %s", acked.Code, acked.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_contact_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	if digestState() != "removed" {
		t.Fatal("an acknowledged removal stays removed when the plugin goes silent")
	}

	second := insertPlacement(t, db, siteID, offerID, "https://plugin.example/again", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, second); err != nil {
		t.Fatal(err)
	}
	undone, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("second undo: %d %s", undone.Code, undone.Body.String())
	}
	restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK || pluginStateOf(t, body, second) != "pending" {
		t.Fatalf("restore pending: %d %s", rec.Code, rec.Body.String())
	}
	acked, ackBody = postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "current" {
		t.Fatalf("ack restored: %d %s", acked.Code, acked.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if pluginStateOf(t, body, second) != "current" {
		t.Fatalf("digest current: %s", rec.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_contact_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, second); err != nil {
		t.Fatal(err)
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if pluginStateOf(t, body, second) != "offline" {
		t.Fatalf("silent after restore: %s", rec.Body.String())
	}
	polled, _ = postJSON(t, r, http.MethodGet, "/v1/placements", nil, siteKey)
	if polled.Code != http.StatusOK {
		t.Fatalf("second poll: %d", polled.Code)
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if pluginStateOf(t, body, second) != "current" || pluginStateOf(t, body, placementID) != "removed" {
		t.Fatalf("poll after silence: %s", rec.Body.String())
	}

	maker, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "removed"}, map[string]string{"X-Maker-Key": "plugin-maker-key"})
	if maker.Code != http.StatusForbidden {
		t.Fatalf("maker ack: %d %s", maker.Code, maker.Body.String())
	}
	session, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "removed"})
	if session.Code != http.StatusForbidden {
		t.Fatalf("session ack: %d %s", session.Code, session.Body.String())
	}
	bad, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "nope"}, siteKey)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad action: %d %s", bad.Code, bad.Body.String())
	}
	nothing, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "removed"}, siteKey)
	if nothing.Code != http.StatusConflict {
		t.Fatalf("nothing to acknowledge: %d %s", nothing.Code, nothing.Body.String())
	}
}

func pluginStateOf(t *testing.T, body map[string]any, id string) string {
	t.Helper()
	if items, ok := body["items"].([]any); ok {
		for _, raw := range items {
			item := raw.(map[string]any)
			if item["id"] == id {
				state, _ := item["plugin_state"].(string)
				return state
			}
		}
	}
	for _, raw := range body["projects"].([]any) {
		project := raw.(map[string]any)
		for _, itemRaw := range project["items"].([]any) {
			item := itemRaw.(map[string]any)
			if item["id"] == id {
				state, _ := item["plugin_state"].(string)
				return state
			}
		}
	}
	t.Fatalf("placement %s missing", id)
	return ""
}

func hitGo(t *testing.T, r *gin.Engine, method, path, agent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("User-Agent", agent)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func bodyReason(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	reason, _ := body["reason"].(string)
	return reason
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
