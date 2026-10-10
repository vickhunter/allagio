package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"

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
	hasCheck := func(body map[string]any, id string) bool {
		t.Helper()
		return body["check_id"] == id && body["check_text"] != "" && body["failures"] == nil
	}

	pausedSite := approve("https://limits.example/site", 80)
	undo(pausedSite)
	paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID}, siteKey)
	if paused.Code != http.StatusOK {
		t.Fatalf("pause site: %d %s", paused.Code, paused.Body.String())
	}
	refused, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+pausedSite+"/restore", nil, siteKey)
	if refused.Code != http.StatusConflict || body["reason"] != "paused" || body["message"] == "" {
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
	if refused.Code != http.StatusConflict || body["reason"] != "paused" || body["message"] == "" || body["error"] != nil {
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
	if denied.Code != http.StatusUnprocessableEntity || !hasCheck(body, "relevance_min") || body["check_id"] == "cap" {
		t.Fatalf("raised min relevance: %d %s", denied.Code, denied.Body.String())
	}
	if statusOf(low) != "revoked" {
		t.Fatal("min relevance failure restored the placement")
	}
	current = current.Add(time.Second)
	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "min_relevance": 40})
	if opened.Code != http.StatusOK {
		t.Fatalf("reopen min relevance: %d %s", opened.Code, opened.Body.String())
	}

	capped := approve("https://limits.example/cap", 90)
	undo(capped)
	lowered, _ := postJSON(t, r, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 0}, siteKey)
	if lowered.Code != http.StatusOK {
		t.Fatalf("lower cap: %d %s", lowered.Code, lowered.Body.String())
	}
	denied, body = postJSON(t, r, http.MethodPost, "/v1/placements/"+capped+"/restore", nil, siteKey)
	if denied.Code != http.StatusUnprocessableEntity || !hasCheck(body, "cap") || body["check_id"] == "relevance_min" {
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

	pending := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	requirePlainRedirect(t, pending, "https://example.com")
	if placementClickCount(t, db, placementID) != 0 {
		t.Fatal("a committed placement that is not placed recorded a click")
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='placed', plugin_wait_at=NOW(), plugin_failed_reason='', failed_action='', restore_unconfirmed=false WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
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
		requirePlainRedirect(t, stopped, "https://example.com")
	}
	if placementClickCount(t, db, placementID) != 1 {
		t.Fatal("revoked link credited a click")
	}

	restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	again := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	requirePlainRedirect(t, again, "https://example.com")
	if placementClickCount(t, db, placementID) != 1 {
		t.Fatal("a restored placement credited a click before the plugin acknowledged placed")
	}

	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET post_url='https://golink.example/post' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	paused := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	requirePlainRedirect(t, paused, "https://golink.example/post")
	if placementClickCount(t, db, placementID) != 1 {
		t.Fatal("an offer that is not live credited a click")
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET post_url='' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	home := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
	requirePlainRedirect(t, home, "https://golink.example/")
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

	assertPlacementStates := func(raw string) {
		t.Helper()
		var payload struct {
			Items []struct {
				Kind               string  `json:"kind"`
				PluginState        *string `json:"plugin_state"`
				PluginFailedReason *string `json:"plugin_failed_reason"`
			} `json:"items"`
			Projects []struct {
				Items []struct {
					Kind               string  `json:"kind"`
					PluginState        *string `json:"plugin_state"`
					PluginFailedReason *string `json:"plugin_failed_reason"`
				} `json:"items"`
			} `json:"projects"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatal(err)
		}
		check := func(kind string, state, failedReason *string) {
			t.Helper()
			if kind == "offer" {
				if state != nil || failedReason != nil {
					t.Fatalf("offer plugin fields are not null: %s", raw)
				}
				return
			}
			if kind != "placement" {
				return
			}
			if state == nil || (*state != "pending" && *state != "placed" && *state != "removed" && *state != "offline" && *state != "failed") || failedReason == nil {
				t.Fatalf("plugin_state is null or unknown: %s", raw)
			}
		}
		for _, item := range payload.Items {
			check(item.Kind, item.PluginState, item.PluginFailedReason)
		}
		for _, project := range payload.Projects {
			for _, item := range project.Items {
				check(item.Kind, item.PluginState, item.PluginFailedReason)
			}
		}
	}
	digestState := func() string {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
		}
		assertPlacementStates(rec.Body.String())
		return pluginStateOf(t, body, placementID)
	}
	if digestState() != "pending" {
		t.Fatal("a brand-new placement should be pending")
	}
	firstAck, firstBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if firstAck.Code != http.StatusOK || firstBody["plugin_state"] != "placed" || digestState() != "placed" {
		t.Fatalf("first ack: %d %s", firstAck.Code, firstAck.Body.String())
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
	assertPlacementStates(account.Body.String())

	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	if digestState() != "offline" {
		t.Fatal("a revoke with no ack after 10 minutes should be offline")
	}
	polled, polledBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+placementID, nil, siteKey)
	if polled.Code != http.StatusOK || polledBody["plugin_state"] != "offline" {
		t.Fatalf("poll: %d %s", polled.Code, polled.Body.String())
	}
	if digestState() != "offline" {
		t.Fatal("a poll must not clear an offline revoke")
	}

	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "removed"}, siteKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "removed" || digestState() != "removed" {
		t.Fatalf("ack removed: %d %s", acked.Code, acked.Body.String())
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
	assertPlacementStates(rec.Body.String())
	acked, ackBody = postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("ack restored: %d %s", acked.Code, acked.Body.String())
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if pluginStateOf(t, body, second) != "placed" || pluginStateOf(t, body, placementID) != "removed" {
		t.Fatalf("digest after restore ack: %s", rec.Body.String())
	}
	statusRec, statusBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+second, nil, siteKey)
	if statusRec.Code != http.StatusOK || statusBody["plugin_state"] != "placed" || statusBody["status"] == "" {
		t.Fatalf("status read: %d %s", statusRec.Code, statusRec.Body.String())
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
	requireGoLinkHeaders(t, rec)
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

func TestPlacementStatusPollAllowsFourSeconds(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	now := time.Date(2026, 10, 10, 5, 0, 0, 0, time.UTC)
	limiter := NewPlacementPollLimiter()
	limiter.nowFn = func() time.Time { return now }
	api.StatusLimiter = limiter
	if PlacementPollInterval != 2*time.Second || limiter.limit < 1 {
		t.Fatal("placement status limit must be one request every 2 seconds")
	}
	query := strings.ToLower(database.PlacementStatusSQL)
	if strings.Contains(query, " join ") || strings.Count(query, " from ") != 1 || !strings.Contains(query, "from placement_proposals") {
		t.Fatal(database.PlacementStatusSQL)
	}

	owner, cookie := testOwnerSession(t, h, "poll-owner", "poll-owner@example.com")
	siteID := createTestSite(t, db, "poll-site-key", "poll.example", "Poll")
	_, offerID := seedListedOffer(t, db, "poll-maker-key", "poll-maker@example.com", "Poll Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "poll-site-key"}
	first := insertPlacement(t, db, siteID, offerID, "https://poll.example/one", "Honest note about the product.")
	second := insertPlacement(t, db, siteID, offerID, "https://poll.example/two", "Honest note about the product.")

	got, body := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, siteKey)
	if got.Code != http.StatusOK || body["plugin_state"] != "pending" || body["status"] == "" {
		t.Fatalf("first poll: %d %s", got.Code, got.Body.String())
	}
	again, _ := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, siteKey)
	if again.Code != http.StatusTooManyRequests {
		t.Fatalf("burst: %d %s", again.Code, again.Body.String())
	}
	otherPlacement, _ := postJSON(t, r, http.MethodGet, "/v1/placements/"+second, nil, siteKey)
	if otherPlacement.Code != http.StatusOK {
		t.Fatalf("other placement: %d %s", otherPlacement.Code, otherPlacement.Body.String())
	}
	session, sessionBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/placements/"+first, nil)
	if session.Code != http.StatusOK || sessionBody["plugin_state"] != "pending" {
		t.Fatalf("other session: %d %s", session.Code, session.Body.String())
	}
	now = now.Add(time.Second)
	tooSoon, _ := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, siteKey)
	if tooSoon.Code != http.StatusTooManyRequests {
		t.Fatalf("poll after 1s: %d %s", tooSoon.Code, tooSoon.Body.String())
	}
	now = now.Add(time.Second)
	afterWindow, _ := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, siteKey)
	if afterWindow.Code != http.StatusOK {
		t.Fatalf("poll after 2s: %d %s", afterWindow.Code, afterWindow.Body.String())
	}
	now = now.Add(4 * time.Second)
	after, _ := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, siteKey)
	if after.Code != http.StatusOK {
		t.Fatalf("poll after 4s: %d %s", after.Code, after.Body.String())
	}
}

func TestEveryHardCheckReturns422(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	current := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }
	owner, cookie := testOwnerSession(t, h, "checks-owner", "checks-owner@example.com")
	siteID := createTestSite(t, db, "checks-site-key", "checks.example", "Checks")
	_, offerID := seedListedOffer(t, db, "checks-maker-key", "checks-maker@example.com", "Checks Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("yolo: %d %s", saved.Code, saved.Body.String())
	}
	siteKey := map[string]string{"X-API-Key": "checks-site-key"}
	textOf := func(id string) string {
		t.Helper()
		for _, check := range entry.HardCheckCatalog {
			if check.ID == id {
				return check.Text
			}
		}
		t.Fatalf("catalog missing %s", id)
		return ""
	}
	expect := func(rec *httptest.ResponseRecorder, body map[string]any, id string) {
		t.Helper()
		if rec.Code != http.StatusUnprocessableEntity || body["check_id"] != id || body["check_text"] != textOf(id) || body["failures"] != nil || body["decision"] != nil {
			t.Fatalf("%s: %d %s", id, rec.Code, rec.Body.String())
		}
	}
	decide := func(id string, payload map[string]any) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		return postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/decide", payload, siteKey)
	}
	okPayload := map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true}

	disclosureID := insertPlacement(t, db, siteID, offerID, "https://checks.example/disclosure", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET disclosure='' WHERE id=$1`, disclosureID); err != nil {
		t.Fatal(err)
	}
	disclosureRec, disclosureBody := decide(disclosureID, okPayload)
	expect(disclosureRec, disclosureBody, "disclosure")

	relID := insertPlacement(t, db, siteID, offerID, "https://checks.example/rel", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET disclosure='See the product.' WHERE id=$1`, relID); err != nil {
		t.Fatal(err)
	}
	relRec, relBody := decide(relID, map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	expect(relRec, relBody, "rel_sponsored")

	api.Claims = rejectingClaims{}
	claimsID := insertPlacement(t, db, siteID, offerID, "https://checks.example/claims", "Honest note about the product.")
	claimsRec, claimsBody := decide(claimsID, map[string]any{"relevance": 10, "position": "end", "rel_sponsored": false, "offer_data": "unrelated"})
	api.Claims = entry.LiveClaimChecker{}
	expect(claimsRec, claimsBody, "claims_from_offer")

	copyID := insertPlacement(t, db, siteID, offerID, "https://checks.example/copy", strings.Repeat("a", 141))
	copyRec, copyBody := decide(copyID, okPayload)
	expect(copyRec, copyBody, "copy_length")

	relevanceID := insertPlacement(t, db, siteID, offerID, "https://checks.example/relevance", "Unrelated gardening notes.")
	relevanceRec, relevanceBody := decide(relevanceID, map[string]any{"relevance": 10, "position": "end", "rel_sponsored": true})
	expect(relevanceRec, relevanceBody, "relevance_min")

	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	offerIDPlacement := insertPlacement(t, db, siteID, offerID, "https://checks.example/offer", "Honest note about the product.")
	offerRec, offerBody := decide(offerIDPlacement, okPayload)
	expect(offerRec, offerBody, "offer_live")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}

	revokeID := insertPlacement(t, db, siteID, offerID, "https://checks.example/revoke", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked' WHERE id=$1`, revokeID); err != nil {
		t.Fatal(err)
	}
	revokeRec, revokeBody := decide(revokeID, okPayload)
	expect(revokeRec, revokeBody, "revoke_path")

	first := insertPlacement(t, db, siteID, offerID, "https://checks.example/one", "Honest note about the product.")
	approved, _ := decide(first, okPayload)
	if approved.Code != http.StatusOK {
		t.Fatalf("first box: %d %s", approved.Code, approved.Body.String())
	}
	second := insertPlacement(t, db, siteID, offerID, "https://checks.example/one", "Another honest note about Checks Product.")
	oneRec, oneBody := decide(second, okPayload)
	expect(oneRec, oneBody, "one_per_post")

	capped := insertPlacement(t, db, siteID, offerID, "https://checks.example/cap", "Honest note about the product.")
	approved, _ = decide(capped, map[string]any{"relevance": 90, "position": "end", "rel_sponsored": true})
	if approved.Code != http.StatusOK {
		t.Fatalf("cap approve: %d %s", approved.Code, approved.Body.String())
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+capped+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	lowered, _ := postJSON(t, r, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 0}, siteKey)
	if lowered.Code != http.StatusOK {
		t.Fatalf("cap: %d %s", lowered.Code, lowered.Body.String())
	}
	current = current.Add(time.Second)
	restored, restoreBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+capped+"/restore", nil, siteKey)
	expect(restored, restoreBody, "cap")
}

func TestPluginFailureAndRetry(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	current := time.Date(2026, 10, 10, 7, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }

	owner, cookie := testOwnerSession(t, h, "fail-owner", "fail-owner@example.com")
	siteID := createTestSite(t, db, "fail-site-key", "fail.example", "Fail")
	createTestSite(t, db, "other-site-key", "other.example", "Other")
	makerID, offerID := seedListedOffer(t, db, "fail-maker-key", "fail-maker@example.com", "Fail Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "fail-site-key"}
	makerKey := map[string]string{"X-Maker-Key": "fail-maker-key"}
	otherKey := map[string]string{"X-API-Key": "other-site-key"}

	approve := func(surface string) string {
		t.Helper()
		id := insertPlacement(t, db, siteID, offerID, surface, "Honest note about the product.")
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	statusOf := func(id string) (state, reason string) {
		t.Helper()
		rec, body := postJSON(t, r, http.MethodGet, "/v1/placements/"+id, nil, siteKey)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %s: %d %s", id, rec.Code, rec.Body.String())
		}
		state, _ = body["plugin_state"].(string)
		reason, _ = body["plugin_failed_reason"].(string)
		if _, ok := body["plugin_failed_reason"]; !ok || state == "" {
			t.Fatalf("status payload: %s", rec.Body.String())
		}
		return state, reason
	}

	placementID := approve("https://fail.example/post")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'fail-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', campaign_id=$2 WHERE id=$1`, placementID, campaignID); err != nil {
		t.Fatal(err)
	}
	goPath := "/v1/go/fail-go?placement=" + placementID
	live := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	if live.Code != http.StatusFound {
		t.Fatalf("live redirect: %d", live.Code)
	}
	clicks := placementClickCount(t, db, placementID)
	stopped := func() {
		t.Helper()
		hit := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
		requirePlainRedirect(t, hit, "https://example.com")
		if placementClickCount(t, db, placementID) != clicks {
			t.Fatal("revoked go link credited a click")
		}
	}
	early, earlyBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, siteKey)
	if early.Code != http.StatusConflict || earlyBody["reason"] != "not_failed" || earlyBody["message"] == "" || earlyBody["error"] != nil {
		t.Fatalf("retry before failure: %d %s", early.Code, early.Body.String())
	}
	missing, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed"}, siteKey)
	blank, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "   "}, siteKey)
	if missing.Code != http.StatusBadRequest || blank.Code != http.StatusBadRequest {
		t.Fatalf("empty reason: %d %d", missing.Code, blank.Code)
	}
	failed, failedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "post re-read failed"}, siteKey)
	if failed.Code != http.StatusOK || failedBody["plugin_state"] != "failed" || failedBody["plugin_failed_reason"] != "post re-read failed" || failedBody["failed_action"] != "restored" {
		t.Fatalf("failed ack: %d %s", failed.Code, failed.Body.String())
	}
	digest, digestBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if digest.Code != http.StatusOK || pluginStateOf(t, digestBody, placementID) != "failed" || pluginFailedReasonOf(t, digestBody, placementID) != "post re-read failed" {
		t.Fatalf("digest failure: %d %s", digest.Code, digest.Body.String())
	}
	account, accountBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if account.Code != http.StatusOK || pluginStateOf(t, accountBody, placementID) != "failed" || pluginFailedReasonOf(t, accountBody, placementID) != "post re-read failed" {
		t.Fatalf("account digest failure: %d %s", account.Code, account.Body.String())
	}
	if state, reason := statusOf(placementID); state != "failed" || reason != "post re-read failed" {
		t.Fatalf("status after failure: %s %s", state, reason)
	}
	current = current.Add(11 * time.Minute)
	if state, reason := statusOf(placementID); state != "failed" || reason != "post re-read failed" {
		t.Fatalf("aged brand-new failure: %s %s", state, reason)
	}

	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	if state, reason := statusOf(placementID); state != "pending" || reason != "" {
		t.Fatalf("undo clears failure: %s %q", state, reason)
	}
	stopped()
	current = current.Add(11 * time.Minute)
	if state, _ := statusOf(placementID); state != "offline" {
		t.Fatalf("silent undo: %s", state)
	}
	offlineRetry, offlineBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, siteKey)
	if offlineRetry.Code != http.StatusConflict || offlineBody["reason"] != "not_failed" || offlineBody["message"] == "" {
		t.Fatalf("retry offline: %d %s", offlineRetry.Code, offlineRetry.Body.String())
	}
	acked, ackedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "still missing"}, siteKey)
	if acked.Code != http.StatusOK || ackedBody["failed_action"] != "removed" {
		t.Fatalf("failed after offline: %d %s", acked.Code, acked.Body.String())
	}
	stopped()
	if state, reason := statusOf(placementID); state != "failed" || reason != "still missing" {
		t.Fatalf("failure beats offline: %s %s", state, reason)
	}
	updated, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-read failed again"}, siteKey)
	if updated.Code != http.StatusOK {
		t.Fatalf("second failure: %d %s", updated.Code, updated.Body.String())
	}
	wrong, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if wrong.Code != http.StatusConflict {
		t.Fatalf("wrong ack: %d %s", wrong.Code, wrong.Body.String())
	}
	if state, reason := statusOf(placementID); state != "failed" || reason != "re-read failed again" {
		t.Fatalf("wrong ack cleared failure: %s %s", state, reason)
	}
	current = current.Add(11 * time.Minute)
	if state, _ := statusOf(placementID); state != "failed" {
		t.Fatalf("aged failure became %s", state)
	}

	makerRetry, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, makerKey)
	otherRetry, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, otherKey)
	if makerRetry.Code != http.StatusForbidden || otherRetry.Code != http.StatusForbidden {
		t.Fatalf("other side: maker %d other %d", makerRetry.Code, otherRetry.Code)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Minute)
	sessionRetry, sessionBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil)
	if sessionRetry.Code != http.StatusOK || sessionBody["plugin_state"] != "pending" || sessionBody["plugin_failed_reason"] != "" || sessionBody["failed_action"] != "" {
		t.Fatalf("removal retry: %d %s", sessionRetry.Code, sessionRetry.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	var waited time.Time
	var storedReason, storedWait, storedStatus string
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait_at, plugin_failed_reason, plugin_wait, status FROM placement_proposals WHERE id=$1`, placementID).Scan(&waited, &storedReason, &storedWait, &storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedWait != "remove" || storedReason != "" || storedStatus != "revoked" || !waited.UTC().Equal(current) {
		t.Fatalf("re-queued remove: %s %q %s %s", storedWait, storedReason, storedStatus, waited.UTC())
	}
	stopped()
	placedAttempt, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if placedAttempt.Code != http.StatusConflict {
		t.Fatalf("removal retry accepted restored: %d %s", placedAttempt.Code, placedAttempt.Body.String())
	}
	if state, _ := statusOf(placementID); state != "pending" {
		t.Fatalf("removal retry became %s", state)
	}
	stopped()
	removed, removedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "removed"}, siteKey)
	if removed.Code != http.StatusOK || removedBody["plugin_state"] != "removed" || removedBody["plugin_failed_reason"] != "" || removedBody["failed_action"] != "" {
		t.Fatalf("ack removed: %d %s", removed.Code, removed.Body.String())
	}
	stopped()

	second := approve("https://fail.example/restored")
	undone, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("second undo: %d %s", undone.Code, undone.Body.String())
	}
	current = current.Add(time.Second)
	restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	failed, failedBody = postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-insert re-read failed"}, siteKey)
	if failed.Code != http.StatusOK || failedBody["failed_action"] != "restored" {
		t.Fatalf("restore failure: %d %s", failed.Code, failed.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='paused' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	denied, deniedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/retry", nil, siteKey)
	if denied.Code != http.StatusUnprocessableEntity || deniedBody["check_id"] != "offer_live" || deniedBody["check_text"] == "" {
		t.Fatalf("restore retry check: %d %s", denied.Code, denied.Body.String())
	}
	if state, reason := statusOf(second); state != "failed" || reason != "re-insert re-read failed" {
		t.Fatalf("failed restore check changed the failure: %s %s", state, reason)
	}
	var stillAction string
	if err := db.Pool.QueryRow(context.Background(), `SELECT failed_action FROM placement_proposals WHERE id=$1`, second).Scan(&stillAction); err != nil || stillAction != "restored" {
		t.Fatalf("failed action after 422: %s %v", stillAction, err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/retry", nil, siteKey)
	if retried.Code != http.StatusOK || retriedBody["plugin_state"] != "pending" {
		t.Fatalf("site retry: %d %s", retried.Code, retried.Body.String())
	}
	placed, placedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if placed.Code != http.StatusOK || placedBody["plugin_state"] != "placed" || placedBody["plugin_failed_reason"] != "" {
		t.Fatalf("ack restored: %d %s", placed.Code, placed.Body.String())
	}

	hidden := approve("https://fail.example/hidden")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET approved_at=$2, created_at=$2, plugin_wait='' WHERE id=$1`, hidden, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/events", map[string]any{"kind": "digest_opened"})
	if opened.Code != http.StatusCreated {
		t.Fatalf("digest open: %d %s", opened.Code, opened.Body.String())
	}
	before, beforeBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if before.Code != http.StatusOK || digestHas(beforeBody, hidden) {
		t.Fatalf("stale placement stayed in the digest: %s", before.Body.String())
	}
	failed, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+hidden+"/plugin-ack", map[string]any{"action": "failed", "reason": "old post re-read failed"}, siteKey)
	if failed.Code != http.StatusOK {
		t.Fatalf("hidden failure: %d %s", failed.Code, failed.Body.String())
	}
	after, afterBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if after.Code != http.StatusOK || pluginStateOf(t, afterBody, hidden) != "failed" || pluginFailedReasonOf(t, afterBody, hidden) != "old post re-read failed" {
		t.Fatalf("failed placement returned to the digest: %d %s", after.Code, after.Body.String())
	}
}

func TestFailedRestoreIsNotLive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	current := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }

	owner, cookie := testOwnerSession(t, h, "dark-owner", "dark-owner@example.com")
	siteID := createTestSite(t, db, "dark-site-key", "dark.example", "Dark")
	makerID, offerID := seedListedOffer(t, db, "dark-maker-key", "dark-maker@example.com", "Dark Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "dark-site-key"}
	id := insertPlacement(t, db, siteID, offerID, "https://dark.example/post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'dark-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', approved_at=NOW(), campaign_id=$2 WHERE id=$1`, id, campaignID); err != nil {
		t.Fatal(err)
	}
	var clickID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO clicks(site_id,campaign_id,placement_id,referrer) VALUES($1,$2,$3,'https://news.example/story') RETURNING id`, siteID, campaignID, id).Scan(&clickID); err != nil {
		t.Fatal(err)
	}
	var attributed string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id,campaign_id,click_id,provider,provider_event_id,kind,amount,currency,commission_amount,occurred_at,payment_mode)
VALUES($1,$2,$3,'stripe','dark-attributed','sale',10,'USD',1.50,NOW(),'test') RETURNING id`, offerID, campaignID, clickID).Scan(&attributed); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id,conversion_id,kind,amount,currency) VALUES($1,$2,'commission',1.50,'USD')`, siteID, attributed); err != nil {
		t.Fatal(err)
	}
	var other string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id,provider,provider_event_id,kind,amount,currency,commission_amount,occurred_at,payment_mode)
VALUES($1,'stripe','dark-other','sale',20,'USD',2.00,NOW(),'test') RETURNING id`, offerID).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id,conversion_id,kind,amount,currency) VALUES($1,$2,'commission',2.00,'USD')`, siteID, other); err != nil {
		t.Fatal(err)
	}

	goPath := "/v1/go/dark-go?placement=" + id
	numbers := func() (wentLive, earnings float64) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
		}
		totals := body["totals"].(map[string]any)
		wentLive = totals["went_live"].(float64)
		earnings = moneyAmount(t, totals["earnings"], "USD")
		var sawSite bool
		for _, raw := range body["projects"].([]any) {
			project := raw.(map[string]any)
			if project["id"] != siteID {
				continue
			}
			sawSite = true
			if project["went_live"] != wentLive-1 {
				t.Fatalf("site live count: %v totals %v", project["went_live"], wentLive)
			}
			if moneyAmount(t, project["earnings"], "USD") != earnings {
				t.Fatalf("site earnings: %v totals %v", project["earnings"], earnings)
			}
		}
		if !sawSite {
			t.Fatalf("site missing from digest: %s", rec.Body.String())
		}
		return wentLive, earnings
	}
	clicks := placementClickCount(t, db, id)
	live := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	if live.Code != http.StatusFound {
		t.Fatalf("live redirect: %d %s", live.Code, live.Body.String())
	}
	clicks = placementClickCount(t, db, id)
	if went, earned := numbers(); went != 1 || earned != 3.5 {
		t.Fatalf("pending before ack: went %v earned %v", went, earned)
	}
	placedFirst, placedFirstBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if placedFirst.Code != http.StatusOK || placedFirstBody["plugin_state"] != "placed" {
		t.Fatalf("first ack: %d %s", placedFirst.Code, placedFirst.Body.String())
	}
	if went, earned := numbers(); went != 2 || earned != 3.5 {
		t.Fatalf("placed counts: went %v earned %v", went, earned)
	}

	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	current = current.Add(time.Second)
	restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	again := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	if again.Code != http.StatusFound {
		t.Fatalf("restore redirect: %d %s", again.Code, again.Body.String())
	}
	clicks = placementClickCount(t, db, id)
	if went, earned := numbers(); went != 1 || earned != 3.5 {
		t.Fatalf("restore before ack: went %v earned %v", went, earned)
	}

	failed, failedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-insert re-read failed"}, siteKey)
	if failed.Code != http.StatusOK || failedBody["failed_action"] != "restored" || failedBody["plugin_state"] != "failed" {
		t.Fatalf("failed restore: %d %s", failed.Code, failed.Body.String())
	}
	dark := func() {
		t.Helper()
		hit := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
		requirePlainRedirect(t, hit, "https://example.com")
		if placementClickCount(t, db, id) != clicks {
			t.Fatal("failed restore credited a click")
		}
		if went, earned := numbers(); went != 1 || earned != 2 {
			t.Fatalf("dark counts: went %v earned %v", went, earned)
		}
		var status string
		var held bool
		if err := db.Pool.QueryRow(context.Background(), `SELECT status, restore_unconfirmed FROM placement_proposals WHERE id=$1`, id).Scan(&status, &held); err != nil || status != "committed" || !held {
			t.Fatalf("held row: %s %v %v", status, held, err)
		}
	}
	dark()

	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/retry", nil, siteKey)
	if retried.Code != http.StatusOK || retriedBody["plugin_state"] != "pending" {
		t.Fatalf("retry: %d %s", retried.Code, retried.Body.String())
	}
	dark()

	placed, placedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if placed.Code != http.StatusOK || placedBody["plugin_state"] != "placed" {
		t.Fatalf("ack restored: %d %s", placed.Code, placed.Body.String())
	}
	back := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	if back.Code != http.StatusFound {
		t.Fatalf("confirmed redirect: %d %s", back.Code, back.Body.String())
	}
	if placementClickCount(t, db, id) != clicks+1 {
		t.Fatal("confirmed restore did not record a click")
	}
	if went, earned := numbers(); went != 2 || earned != 3.5 {
		t.Fatalf("confirmed counts: went %v earned %v", went, earned)
	}
}

func TestLeaveFailedRestoreRemoved(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	current := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }

	owner, cookie := testOwnerSession(t, h, "leave-owner", "leave-owner@example.com")
	siteID := createTestSite(t, db, "leave-site-key", "leave.example", "Leave")
	makerID, offerID := seedListedOffer(t, db, "leave-maker-key", "leave-maker@example.com", "Leave Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "leave-site-key"}
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'leave-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	liveNumbers := func() (wentLive float64, siteLive float64) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
		}
		wentLive = body["totals"].(map[string]any)["went_live"].(float64)
		for _, raw := range body["projects"].([]any) {
			project := raw.(map[string]any)
			if project["id"] == siteID {
				siteLive = project["went_live"].(float64)
			}
		}
		return wentLive, siteLive
	}
	commit := func(surface, label string) (id, goPath string) {
		t.Helper()
		id = insertPlacement(t, db, siteID, offerID, surface, "Honest note about the product.")
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', approved_at=NOW(), campaign_id=$2 WHERE id=$1`, id, campaignID); err != nil {
			t.Fatal(err)
		}
		var clickID string
		if err := db.Pool.QueryRow(context.Background(), `INSERT INTO clicks(site_id,campaign_id,placement_id,referrer) VALUES($1,$2,$3,'https://news.example/story') RETURNING id`, siteID, campaignID, id).Scan(&clickID); err != nil {
			t.Fatal(err)
		}
		var attributed string
		if err := db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id,campaign_id,click_id,provider,provider_event_id,kind,amount,currency,commission_amount,occurred_at,payment_mode)
VALUES($1,$2,$3,'stripe',$4,'sale',10,'USD',1.50,NOW(),'test') RETURNING id`, offerID, campaignID, clickID, label+"-attributed").Scan(&attributed); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id,conversion_id,kind,amount,currency) VALUES($1,$2,'commission',1.50,'USD')`, siteID, attributed); err != nil {
			t.Fatal(err)
		}
		goPath = "/v1/go/leave-go?placement=" + id
		return id, goPath
	}
	failRestore := func(id string) {
		t.Helper()
		undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", map[string]any{"reason": "undo"}, siteKey)
		if undone.Code != http.StatusOK {
			t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
		}
		current = current.Add(time.Second)
		restored, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/restore", nil, siteKey)
		if restored.Code != http.StatusOK {
			t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
		}
		failed, failedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "failed", "reason": "re-insert re-read failed"}, siteKey)
		if failed.Code != http.StatusOK || failedBody["plugin_state"] != "failed" || failedBody["failed_action"] != "restored" {
			t.Fatalf("failed restore: %d %s", failed.Code, failed.Body.String())
		}
	}

	id, goPath := commit("https://leave.example/post", "leave")
	var other string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id,provider,provider_event_id,kind,amount,currency,commission_amount,occurred_at,payment_mode)
VALUES($1,'stripe','leave-other','sale',20,'USD',2.00,NOW(),'test') RETURNING id`, offerID).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id,conversion_id,kind,amount,currency) VALUES($1,$2,'commission',2.00,'USD')`, siteID, other); err != nil {
		t.Fatal(err)
	}
	placedFirst, placedFirstBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "restored"}, siteKey)
	if placedFirst.Code != http.StatusOK || placedFirstBody["plugin_state"] != "placed" {
		t.Fatalf("first ack: %d %s", placedFirst.Code, placedFirst.Body.String())
	}
	if went, site := liveNumbers(); went != 2 || site != 1 {
		t.Fatalf("live counts: totals %v site %v", went, site)
	}
	failRestore(id)
	if went, site := liveNumbers(); went != 1 || site != 0 {
		t.Fatalf("failed restore still live: totals %v site %v", went, site)
	}
	clicks := placementClickCount(t, db, id)
	left, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if left.Code != http.StatusOK {
		t.Fatalf("leave removed: %d %s", left.Code, left.Body.String())
	}
	view := func(id string) map[string]any {
		t.Helper()
		status, body := postJSON(t, r, http.MethodGet, "/v1/placements/"+id, nil, siteKey)
		if status.Code != http.StatusOK {
			t.Fatalf("status %s: %d %s", id, status.Code, status.Body.String())
		}
		return body
	}
	stopped := func(path, id string, clicks int) {
		t.Helper()
		hit := hitGo(t, r, http.MethodGet, path, "Mozilla/5.0")
		requirePlainRedirect(t, hit, "https://example.com")
		if placementClickCount(t, db, id) != clicks {
			t.Fatal("clicks-off go link credited a click")
		}
	}
	notLive := func() {
		t.Helper()
		if went, site := liveNumbers(); went != 1 || site != 0 {
			t.Fatalf("still live: totals %v site %v", went, site)
		}
	}

	body := view(id)
	if body["plugin_state"] != "pending" || body["failed_action"] != "" || body["status"] != "revoked" {
		t.Fatalf("undo of a failed restore: %v", body)
	}
	digest, digestBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if digest.Code != http.StatusOK || pluginStateOf(t, digestBody, id) != "pending" {
		t.Fatalf("digest pending: %d %s", digest.Code, digest.Body.String())
	}
	account, accountBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if account.Code != http.StatusOK || pluginStateOf(t, accountBody, id) != "pending" {
		t.Fatalf("site digest pending: %d %s", account.Code, account.Body.String())
	}
	notLive()
	stopped(goPath, id, clicks)
	confirmed, confirmedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "removed"}, siteKey)
	if confirmed.Code != http.StatusOK || confirmedBody["plugin_state"] != "removed" || confirmedBody["failed_action"] != "" {
		t.Fatalf("ack removed: %d %s", confirmed.Code, confirmed.Body.String())
	}
	notLive()
	stopped(goPath, id, clicks)

	second, secondPath := commit("https://leave.example/again", "again")
	failRestore(second)
	secondClicks := placementClickCount(t, db, second)
	againLeft, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if againLeft.Code != http.StatusOK || view(second)["plugin_state"] != "pending" {
		t.Fatalf("second undo: %d %s", againLeft.Code, againLeft.Body.String())
	}
	current = current.Add(11 * time.Minute)
	if view(second)["plugin_state"] != "offline" {
		t.Fatalf("silent undo: %v", view(second))
	}
	notLive()
	stopped(secondPath, second, secondClicks)

	third, thirdPath := commit("https://leave.example/third", "third")
	failRestore(third)
	thirdClicks := placementClickCount(t, db, third)
	thirdLeft, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+third+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if thirdLeft.Code != http.StatusOK || view(third)["plugin_state"] != "pending" {
		t.Fatalf("third undo: %d %s", thirdLeft.Code, thirdLeft.Body.String())
	}
	failedAck, failedAckBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+third+"/plugin-ack", map[string]any{"action": "failed", "reason": "remove re-read failed"}, siteKey)
	if failedAck.Code != http.StatusOK || failedAckBody["plugin_state"] != "failed" || failedAckBody["failed_action"] != "removed" {
		t.Fatalf("failed ack: %d %s", failedAck.Code, failedAck.Body.String())
	}
	notLive()
	stopped(thirdPath, third, thirdClicks)
	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+third+"/retry", nil, siteKey)
	if retried.Code != http.StatusOK || retriedBody["plugin_state"] != "pending" || retriedBody["failed_action"] != "" {
		t.Fatalf("removal retry: %d %s", retried.Code, retried.Body.String())
	}
	stopped(thirdPath, third, thirdClicks)

	fourth, _ := commit("https://leave.example/fourth", "fourth")
	failRestore(fourth)
	fourthLeft, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+fourth+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if fourthLeft.Code != http.StatusOK || view(fourth)["plugin_state"] != "pending" {
		t.Fatalf("fourth undo: %d %s", fourthLeft.Code, fourthLeft.Body.String())
	}
	earlyRetry, earlyBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+fourth+"/retry", nil, siteKey)
	if earlyRetry.Code != http.StatusConflict || earlyBody["reason"] != "not_failed" || view(fourth)["plugin_state"] != "pending" {
		t.Fatalf("retry while pending: %d %s", earlyRetry.Code, earlyRetry.Body.String())
	}
	restored, restoredBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+fourth+"/restore", nil, siteKey)
	if restored.Code != http.StatusOK || restoredBody["plugin_state"] == "placed" || view(fourth)["plugin_state"] != "pending" {
		t.Fatalf("restore queued placed: %d %s %v", restored.Code, restored.Body.String(), view(fourth))
	}
	paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID}, siteKey)
	if paused.Code != http.StatusOK || view(fourth)["plugin_state"] == "placed" || view(fourth)["plugin_state"] == "removed" {
		t.Fatalf("pause set a terminal plugin state: %d %v", paused.Code, view(fourth))
	}
	killed, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", map[string]any{}, siteKey)
	if killed.Code != http.StatusOK || view(fourth)["plugin_state"] == "placed" || view(fourth)["plugin_state"] == "removed" {
		t.Fatalf("kill switch set a terminal plugin state: %d %v", killed.Code, view(fourth))
	}
}

// TestPlacedAndRemovedOnlyFromPluginAck scans every plugin_wait write.
// placed and removed are stored only by AckPluginPlacement, and only through
// VerifiedPluginWait. Revoke, restore, retry, pause, and the kill switch cannot.
func TestPlacedAndRemovedOnlyFromPluginAck(t *testing.T) {
	for _, action := range []string{"revoke", "restore", "retry", "pause", "kill", "kill_switch", "undo"} {
		if wait, ok := entry.VerifiedPluginWait(action); ok || wait != "" {
			t.Fatalf("%s stored %q", action, wait)
		}
	}
	if wait, ok := entry.VerifiedPluginWait("restored"); !ok || wait != "placed" {
		t.Fatalf("restored ack stored %q ok=%v", wait, ok)
	}
	if wait, ok := entry.VerifiedPluginWait("removed"); !ok || wait != "removed" {
		t.Fatalf("removed ack stored %q ok=%v", wait, ok)
	}

	root := apiModuleRoot(t)
	forbidden := regexp.MustCompile(`plugin_wait\s*=\s*(?:CASE\b[\s\S]{0,800}?THEN\s+)?'(?:placed|removed)'`)
	var callers []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, "_test.go") || !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".sql")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		text := string(data)
		for _, loc := range forbidden.FindAllStringIndex(text, -1) {
			// A qualified column, as in the live-count view, is a read.
			if loc[0] > 0 && text[loc[0]-1] == '.' {
				continue
			}
			line := 1 + strings.Count(text[:loc[0]], "\n")
			t.Errorf("%s:%d assigns plugin_wait to placed or removed", rel, line)
		}
		if !strings.Contains(text, "VerifiedPluginWait(") {
			return nil
		}
		for i, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "VerifiedPluginWait(") && !strings.HasPrefix(strings.TrimSpace(line), "func ") {
				callers = append(callers, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(callers) != 1 || !strings.HasPrefix(callers[0], "internal/database/placements.go:") {
		t.Fatalf("VerifiedPluginWait callers: %v", callers)
	}
	ack, err := os.ReadFile(filepath.Join(root, "internal/database/placements.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(ack)
	call := strings.Index(body, "entry.VerifiedPluginWait(")
	fn := strings.LastIndex(body[:call], "func ")
	if call < 0 || fn < 0 || !strings.Contains(body[fn:call], "AckPluginPlacement") {
		t.Fatal("VerifiedPluginWait is not called from AckPluginPlacement")
	}
}

func apiModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func pluginFailedReasonOf(t *testing.T, body map[string]any, id string) string {
	t.Helper()
	if items, ok := body["items"].([]any); ok {
		for _, raw := range items {
			item := raw.(map[string]any)
			if item["id"] == id {
				reason, _ := item["plugin_failed_reason"].(string)
				return reason
			}
		}
	}
	projects, _ := body["projects"].([]any)
	for _, raw := range projects {
		project := raw.(map[string]any)
		for _, itemRaw := range project["items"].([]any) {
			item := itemRaw.(map[string]any)
			if item["id"] == id {
				reason, _ := item["plugin_failed_reason"].(string)
				return reason
			}
		}
	}
	t.Fatalf("placement %s missing", id)
	return ""
}

func digestHas(body map[string]any, id string) bool {
	items, _ := body["items"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["id"] == id {
			return true
		}
	}
	return false
}

func TestPlacementLiveCountExclusions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	current := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }

	root := apiModuleRoot(t)
	viewSQL, err := os.ReadFile(filepath.Join(root, "internal/migrations/028_d032_placement_live.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		"plugin_wait = 'placed'",
		"pp.status = 'committed'",
		"NOT pp.restore_unconfirmed",
		"o.status = 'listed'",
		"c.status = 'active'",
		"pol.kill_switch",
	} {
		if !strings.Contains(string(viewSQL), part) {
			t.Fatalf("live view missing %s", part)
		}
	}
	entrySQL, err := os.ReadFile(filepath.Join(root, "internal/database/agent_entry.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entrySQL), "placement_counts_as_live") || strings.Contains(string(entrySQL), "status IN ('approved','committed') AND NOT restore_unconfirmed") {
		t.Fatal("digest live count does not use placement_counts_as_live")
	}

	owner, cookie := testOwnerSession(t, h, "live-owner", "live-owner@example.com")
	siteID := createTestSite(t, db, "live-site-key", "live.example", "Live")
	makerID, offerID := seedListedOffer(t, db, "live-maker-key", "live-maker@example.com", "Live Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "live-site-key"}
	id := insertPlacement(t, db, siteID, offerID, "https://live.example/post", "Honest note about the product.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'live-go',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	goPath := "/v1/go/live-go?placement=" + id

	makeLive := func() {
		t.Helper()
		if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET status='listed' WHERE id=$1`, offerID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE campaigns SET status='active' WHERE id=$1`, campaignID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE approval_policies SET kill_switch=false WHERE site_id=$1 OR offer_id=$2`, siteID, offerID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='placed', plugin_wait_at=$2, plugin_failed_reason='', failed_action='', restore_unconfirmed=false,
campaign_id=$3, expires_at=$4
WHERE id=$1`, id, current, campaignID, current.Add(48*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	read := func() (totals, site float64, state string, viewed bool) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("account digest: %d %s", rec.Code, rec.Body.String())
		}
		totals = body["totals"].(map[string]any)["went_live"].(float64)
		for _, raw := range body["projects"].([]any) {
			project := raw.(map[string]any)
			if project["id"] == siteID {
				site = project["went_live"].(float64)
			}
		}
		state = pluginStateOf(t, body, id)
		digest, digestBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
		if digest.Code != http.StatusOK || pluginStateOf(t, digestBody, id) != state {
			t.Fatalf("digest state: %d %s account %s", digest.Code, digest.Body.String(), state)
		}
		if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1)`, id).Scan(&viewed); err != nil {
			t.Fatal(err)
		}
		return totals, site, state, viewed
	}
	expect := func(label string, totals, site float64, state string, viewed bool) {
		t.Helper()
		gotTotals, gotSite, gotState, gotView := read()
		if gotTotals != totals || gotSite != site || gotState != state || gotView != viewed {
			t.Fatalf("%s: totals %v site %v state %s view %v", label, gotTotals, gotSite, gotState, gotView)
		}
	}

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='', plugin_failed_reason='', failed_action='', restore_unconfirmed=false WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("pending", 1, 0, "pending", false)

	makeLive()
	hit := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	if hit.Code != http.StatusFound {
		t.Fatalf("placed go link: %d %s", hit.Code, hit.Body.String())
	}
	expect("placed", 2, 1, "placed", true)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='restore', plugin_failed_reason='re-read failed', failed_action='restored', restore_unconfirmed=true
WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("failed", 1, 0, "failed", false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='remove', plugin_wait_at=$2, plugin_failed_reason='', failed_action='', restore_unconfirmed=false
WHERE id=$1`, id, current.Add(-11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	expect("offline", 1, 0, "offline", false)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET plugin_wait='removed', plugin_failed_reason='', failed_action='', restore_unconfirmed=false
WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("removed", 1, 0, "removed", false)

	pauseThen := func(scope string, payload map[string]any) {
		t.Helper()
		makeLive()
		expect("before pause "+scope, 2, 1, "placed", true)
		paused, _ := postJSON(t, r, http.MethodPost, "/v1/account/pause", payload, siteKey)
		if paused.Code != http.StatusOK {
			t.Fatalf("pause %s: %d %s", scope, paused.Code, paused.Body.String())
		}
		expect("paused "+scope, 1, 0, "placed", false)
		resumed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", payload)
		if resumed.Code != http.StatusOK {
			t.Fatalf("resume %s: %d %s", scope, resumed.Code, resumed.Body.String())
		}
		expect("resumed "+scope, 2, 1, "placed", true)
	}
	pauseThen("all", map[string]any{"scope": "all"})
	pauseThen("site", map[string]any{"scope": "site", "id": siteID})
	pauseThen("offer", map[string]any{"scope": "offer", "id": offerID})

	makeLive()
	killed, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", map[string]any{}, siteKey)
	if killed.Code != http.StatusOK {
		t.Fatalf("kill switch: %d %s", killed.Code, killed.Body.String())
	}
	expect("kill switch", 1, 0, "placed", false)
	resumed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "site", "id": siteID})
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume kill switch: %d %s", resumed.Code, resumed.Body.String())
	}
	expect("kill switch off", 2, 1, "placed", true)

	makeLive()
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked', plugin_wait='placed' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expect("revoked but placed", 1, 0, "placed", false)
	stopped := hitGo(t, r, http.MethodGet, goPath, "Mozilla/5.0")
	requirePlainRedirect(t, stopped, "https://example.com")
}

func TestHardCheckDocsFollowTheCatalog(t *testing.T) {
	llms := llmsBody()
	modes, ok := agentDoc("modes.md")
	if !ok {
		t.Fatal("modes doc missing")
	}
	for _, check := range entry.HardCheckCatalog {
		needle := "`" + check.ID + "`"
		if !strings.Contains(llms, needle) || !strings.Contains(llms, check.Text) {
			t.Fatalf("llms.txt drifted on %s", check.ID)
		}
		if !strings.Contains(modes, needle) || !strings.Contains(modes, check.Text) {
			t.Fatalf("modes.md drifted on %s", check.ID)
		}
	}
	if strings.Contains(llms, "disclosure_and_rel_sponsored") || strings.Contains(modes, "`current`") {
		t.Fatal("docs still describe a retired check or plugin state")
	}
	for _, refusal := range entry.RestoreRefusals {
		needle := "`" + refusal.Reason + "`"
		if !strings.Contains(llms, needle) || !strings.Contains(llms, refusal.Text) {
			t.Fatalf("llms.txt drifted on restore reason %s", refusal.Reason)
		}
		if !strings.Contains(modes, needle) || !strings.Contains(modes, refusal.Text) {
			t.Fatalf("modes.md drifted on restore reason %s", refusal.Reason)
		}
	}
	for _, doc := range []string{llms, modes} {
		if !strings.Contains(doc, "POST /v1/placements/:id/retry") || !strings.Contains(doc, "not_failed") {
			t.Fatal("docs drifted on plugin retry")
		}
		if !strings.Contains(doc, "A 409 returns `reason` and `message`") || !strings.Contains(doc, "A 422 returns `check_id` and `check_text`") {
			t.Fatal("docs drifted on the 409 and 422 body shapes")
		}
		if !strings.Contains(doc, "Digest totals, site totals, and Live now exclude") || !strings.Contains(doc, "Leave a failed restore removed") || !strings.Contains(doc, "counts as live only when `plugin_state` is `placed`") || !strings.Contains(doc, "`GET /v1/go/<placement_id>` always returns 302") || !strings.Contains(doc, "`GET /v1/go/<code>`") || !strings.Contains(doc, "never returns a 4xx") || !strings.Contains(doc, "`fallback_url`") || !strings.Contains(doc, "`X-OR-Client-Hash`") || !strings.Contains(doc, "/24") || !strings.Contains(doc, "00:00 UTC") {
			t.Fatal("docs drifted on leave-it-removed")
		}
	}
}

func TestEveryConflictHasAMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := map[string]bool{}
	for _, refusal := range entry.RestoreRefusals {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		writeNamedConflict(c, refusal.Reason)
		if rec.Code != http.StatusConflict {
			t.Fatalf("%s status: %d", refusal.Reason, rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		message, _ := body["message"].(string)
		if body["reason"] != refusal.Reason || message == "" || message != refusal.Text || body["error"] != nil || len(body) != 2 {
			t.Fatalf("%s body: %s", refusal.Reason, rec.Body.String())
		}
		seen[refusal.Reason] = true
	}
	for _, reason := range []string{"window_expired", "paused", "not_failed"} {
		if !seen[reason] {
			t.Fatalf("409 %s was not exercised", reason)
		}
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
