package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if denied.Code != http.StatusUnprocessableEntity || !hasCheck(body, "relevance_min") || body["check_id"] == "cap" {
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
	if PlacementPollInterval < 4*time.Second || limiter.limit < 1 {
		t.Fatal("placement status limit is stricter than one request per 4 seconds")
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
	relRec, relBody := decide(relID, map[string]any{"relevance": 80, "position": "end", "rel_sponsored": false})
	expect(relRec, relBody, "rel_sponsored")

	api.Claims = rejectingClaims{}
	claimsID := insertPlacement(t, db, siteID, offerID, "https://checks.example/claims", "Honest note about the product.")
	claimsRec, claimsBody := decide(claimsID, map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true, "offer_data": "A short honest note."})
	api.Claims = entry.StubClaimChecker{}
	expect(claimsRec, claimsBody, "claims_from_offer")

	copyID := insertPlacement(t, db, siteID, offerID, "https://checks.example/copy", strings.Repeat("a", 141))
	copyRec, copyBody := decide(copyID, okPayload)
	expect(copyRec, copyBody, "copy_length")

	relevanceID := insertPlacement(t, db, siteID, offerID, "https://checks.example/relevance", "Honest note about the product.")
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
	second := insertPlacement(t, db, siteID, offerID, "https://checks.example/one", "Another honest note.")
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
		if hit.Code != http.StatusGone || hit.Header().Get("Location") != "" || bodyReason(t, hit) != "revoked" {
			t.Fatalf("go link left revoked: %d %s %s", hit.Code, hit.Header().Get("Location"), hit.Body.String())
		}
		if placementClickCount(t, db, placementID) != clicks {
			t.Fatal("revoked go link credited a click")
		}
	}
	early, earlyBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/retry", nil, siteKey)
	if early.Code != http.StatusConflict || earlyBody["error"] != "not_failed" || earlyBody["reason"] != "not_failed" {
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
	if offlineRetry.Code != http.StatusConflict || offlineBody["error"] != "not_failed" {
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
