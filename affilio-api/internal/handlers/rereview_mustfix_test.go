package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
)

func TestCommittedPlacementStillCountsClicksAfterExpiry(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	owner, _ := testOwnerSession(t, h, "expiry-owner", "expiry-owner@example.com")
	siteID := createTestSite(t, db, "expiry-site-key", "expiry.example", "Expiry")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "expiry-maker", "expiry-maker@example.com", "Expiry Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://expiry.example/post", "Expiry Product note.")
	markPlacementLive(t, db, siteID, offerID, placementID, "expiry-go", "https://expiry.example/post")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET expires_at=NOW()-INTERVAL '8 days' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	var live bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1)`, placementID).Scan(&live); err != nil || !live {
		t.Fatalf("committed placement dropped out of the live view: %v %v", live, err)
	}
	hit := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	if hit.Code != http.StatusFound || placementClickCount(t, db, placementID) != 1 {
		t.Fatalf("expired committed click: %d clicks %d", hit.Code, placementClickCount(t, db, placementID))
	}

	pendingID := insertPlacement(t, db, siteID, offerID, "https://expiry.example/pending", "Expiry Product pending.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', expires_at=NOW()-INTERVAL '8 days' WHERE id=$1`, pendingID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "expiry-site-key"}
	expired, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+pendingID+"/commit", nil, siteKey)
	if expired.Code != http.StatusForbidden {
		t.Fatalf("uncommitted expiry: %d %s", expired.Code, expired.Body.String())
	}
}

func TestPreUpgradeAckSetsPlacedWithoutBackfill(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, _ := testOwnerSession(t, h, "pre-owner", "pre-owner@example.com")
	siteID := createTestSite(t, db, "pre-site-key", "pre.example", "Pre")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "pre-maker", "pre-maker@example.com", "Pre Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://pre.example/post", "Pre Product note.")
	markPlacementLive(t, db, siteID, offerID, placementID, "pre-go", "https://pre.example/post")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	var live bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1)`, placementID).Scan(&live); err != nil || live {
		t.Fatalf("pre-upgrade row was live before an ack: %v %v", live, err)
	}
	pluginKey := issuePluginKey(t, db, siteID, "pre-plugin-key")
	acked, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("ack: %d %s", acked.Code, acked.Body.String())
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1)`, placementID).Scan(&live); err != nil || !live {
		t.Fatalf("ack did not make the existing row live: %v %v", live, err)
	}
	sqlText, err := os.ReadFile(filepath.Join(apiModuleRoot(t), "internal/migrations/031_d032_committed_live_plugin_keys.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(sqlText)), "UPDATE") {
		t.Fatal("migration 031 backfills plugin_wait")
	}
}

func TestCapExcludesThePlacementItself(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "cap-owner", "cap-owner@example.com")
	siteID := createTestSite(t, db, "cap-site-key", "cap.example", "Cap")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "cap-maker", "cap-maker@example.com", "Cap Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 1})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	siteKey := map[string]string{"X-API-Key": "cap-site-key"}
	only := insertPlacement(t, db, siteID, offerID, "https://cap.example/only", "Cap Product only.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, only); err != nil {
		t.Fatal(err)
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+only+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+only+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("self was counted against the cap: %d %s", restored.Code, restored.Body.String())
	}

	self := insertPlacement(t, db, siteID, offerID, "https://cap.example/self", "Cap Product self.")
	other := insertPlacement(t, db, siteID, offerID, "https://cap.example/other", "Cap Product other.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1 OR id=$2`, self, other); err != nil {
		t.Fatal(err)
	}
	undone, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+self+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("second undo: %d %s", undone.Code, undone.Body.String())
	}
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+self+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "cap" {
		t.Fatalf("other placement should fill the cap: %d %s", blocked.Code, blocked.Body.String())
	}
}

func TestOfferHeadProbeTreats403And405AsUp(t *testing.T) {
	cases := []struct {
		status int
		up     bool
	}{
		{http.StatusOK, true},
		{http.StatusMovedPermanently, true},
		{http.StatusFound, true},
		{http.StatusForbidden, true},
		{http.StatusMethodNotAllowed, true},
		{http.StatusNotFound, false},
		{http.StatusGone, false},
		{http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		if probeLanding(srv.URL) != tc.up {
			t.Errorf("status %d up=%v", tc.status, tc.up)
		}
		srv.Close()
	}
	if landingUp(0, errors.New("network")) {
		t.Fatal("a network error counted as up")
	}
}

func TestReceiptAcceptsSiteLocalGoPath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	r.POST("/v1/placements/:id/receipt", middleware.APIKeyAuth(db, nil), (&PlacementsHandler{DB: db}).RecordReceipt)
	siteID := createTestSite(t, db, "receipt-site-key", "receipt.example", "Receipt")
	_, offerID := seedListedOffer(t, db, "receipt-maker", "receipt-maker@example.com", "Receipt Product")
	siteKey := map[string]string{"X-API-Key": "receipt-site-key"}
	record := func(path string) {
		t.Helper()
		id := insertPlacement(t, db, siteID, offerID, "https://receipt.example/"+path, "Receipt Product note for "+path+".")
		receipt := `{"tracked_link":"/v1/go/` + id + `","publication_status":"awaiting_evidence"}`
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', receipt=$2::jsonb WHERE id=$1`, id, receipt); err != nil {
			t.Fatal(err)
		}
		var copy, disclosure, surface string
		if err := db.Pool.QueryRow(context.Background(), `SELECT copy, disclosure, surface_url FROM placement_proposals WHERE id=$1`, id).Scan(&copy, &disclosure, &surface); err != nil {
			t.Fatal(err)
		}
		published := copy + "\n" + disclosure + "\n" + path + id
		rec, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/receipt", map[string]any{
			"surface_url": surface, "published_text": published, "evidence": "sha256-of-the-post-body",
		}, siteKey)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	record("/go/")
	record("/v1/go/")

	missingID := insertPlacement(t, db, siteID, offerID, "https://receipt.example/missing", "Receipt Product note for missing.")
	receipt := `{"tracked_link":"/v1/go/` + missingID + `","publication_status":"awaiting_evidence"}`
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', receipt=$2::jsonb WHERE id=$1`, missingID, receipt); err != nil {
		t.Fatal(err)
	}
	var copy, disclosure, surface string
	if err := db.Pool.QueryRow(context.Background(), `SELECT copy, disclosure, surface_url FROM placement_proposals WHERE id=$1`, missingID).Scan(&copy, &disclosure, &surface); err != nil {
		t.Fatal(err)
	}
	miss, missBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+missingID+"/receipt", map[string]any{
		"surface_url": surface, "published_text": copy + "\n" + disclosure, "evidence": "sha256-of-the-post-body",
	}, siteKey)
	if miss.Code != http.StatusConflict || missBody["reason"] != "receipt_mismatch" {
		t.Fatalf("missing link: %d %s", miss.Code, miss.Body.String())
	}
}

func TestRelSponsoredComesFromPluginReread(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "rel-owner", "rel-owner@example.com")
	siteID := createTestSite(t, db, "rel-site-key", "rel.example", "Rel")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "rel-maker", "rel-maker@example.com", "Rel Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://rel.example/post", "Rel Product note.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET disclosure='(link sponsorizzato)' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	decided, decidedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 10, "position": "end", "rel_sponsored": false})
	if decided.Code != http.StatusOK || decidedBody["approved_by"] != "policy:yolo" {
		t.Fatalf("decide used disclosure words: %d %s", decided.Code, decided.Body.String())
	}
	pluginKey := issuePluginKey(t, db, siteID, "rel-plugin-key")
	words, wordsBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": false}, pluginKey)
	if words.Code != http.StatusUnprocessableEntity || wordsBody["check_id"] != "rel_sponsored" {
		t.Fatalf("words satisfied rel: %d %s", words.Code, words.Body.String())
	}
	var wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&wait); err != nil || wait != "" {
		t.Fatalf("stored wait %q %v", wait, err)
	}
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("rel markup: %d %s", acked.Code, acked.Body.String())
	}
}

func TestAPIKeysCanUndoTightenPauseAndKill(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "keys-owner", "keys-owner@example.com")
	siteID := createTestSite(t, db, "keys-fix-site", "keys-fix.example", "Keys")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "keys-fix-maker", "keys-fix-maker@example.com", "Keys Product")
	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 10, "min_relevance": 30})
	if opened.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", opened.Code, opened.Body.String())
	}
	projectKey := "or_keys_fix_project"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_keys',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	created, createdBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account-keys", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("account key: %d %s", created.Code, created.Body.String())
	}
	accountKey := createdBody["key"].(string)

	for _, key := range []string{projectKey, accountKey} {
		headers := map[string]string{"X-API-Key": key}
		tight, _ := postJSON(t, r, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "auto", "max_placements_per_30_days": 4, "min_relevance": 45}, headers)
		if tight.Code != http.StatusOK {
			t.Fatalf("tighten %s: %d %s", key, tight.Code, tight.Body.String())
		}
		reset, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 10, "min_relevance": 30})
		if reset.Code != http.StatusOK {
			t.Fatalf("reset: %d %s", reset.Code, reset.Body.String())
		}
		placementID := insertPlacement(t, db, siteID, offerID, "https://keys-fix.example/"+key, "Keys Product note.")
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, placementID); err != nil {
			t.Fatal(err)
		}
		undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, headers)
		if undone.Code != http.StatusOK {
			t.Fatalf("undo %s: %d %s", key, undone.Code, undone.Body.String())
		}
		decide, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true}, headers)
		restore, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil, headers)
		if decide.Code != http.StatusForbidden || restore.Code != http.StatusForbidden {
			t.Fatalf("decide/restore %s: %d %d", key, decide.Code, restore.Code)
		}
		sessionRestore, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
		if sessionRestore.Code != http.StatusOK {
			t.Fatalf("session restore after %s: %d %s", key, sessionRestore.Code, sessionRestore.Body.String())
		}
		killed, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/kill-switch", map[string]any{"enabled": true}, headers)
		if killed.Code != http.StatusOK {
			t.Fatalf("kill %s: %d %s", key, killed.Code, killed.Body.String())
		}
		resumed, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "site", "id": siteID})
		if resumed.Code != http.StatusOK {
			t.Fatalf("resume: %d %s", resumed.Code, resumed.Body.String())
		}
		paused, pausedBody := postJSON(t, r, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID}, headers)
		if paused.Code != http.StatusOK || appliedRow(t, pausedBody)["mode"] != "manual" {
			t.Fatalf("pause %s: %d %s", key, paused.Code, paused.Body.String())
		}
		resumed, _ = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "site", "id": siteID})
		if resumed.Code != http.StatusOK {
			t.Fatalf("resume after pause: %d %s", resumed.Code, resumed.Body.String())
		}
	}
	sessionDecideID := insertPlacement(t, db, siteID, offerID, "https://keys-fix.example/session", "Keys Product session.")
	sessionDecide, sessionBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+sessionDecideID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	if sessionDecide.Code != http.StatusOK || sessionBody["approved_by"] != "policy:yolo" {
		t.Fatalf("session decide: %d %s", sessionDecide.Code, sessionDecide.Body.String())
	}
}

func TestOfflineTimerStartsWhenPluginIsAsked(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	siteID := createTestSite(t, db, "offline-site-key", "offline.example", "Offline")
	_, offerID := seedListedOffer(t, db, "offline-maker", "offline-maker@example.com", "Offline Product")
	placementID := insertPlacement(t, db, siteID, offerID, "https://offline.example/post", "Offline Product note.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET created_at=NOW()-INTERVAL '11 minutes', plugin_wait='', plugin_wait_at=NULL WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	siteKey := map[string]string{"X-API-Key": "offline-site-key"}
	fresh, freshBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+placementID, nil, siteKey)
	if fresh.Code != http.StatusOK || freshBody["plugin_state"] != "pending" {
		t.Fatalf("created_at started the timer: %d %s", fresh.Code, fresh.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	aged, agedBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+placementID, nil, siteKey)
	if aged.Code != http.StatusOK || agedBody["plugin_state"] != "offline" {
		t.Fatalf("asked-at timer: %d %s", aged.Code, aged.Body.String())
	}
}

func TestStubClaimCheckerRefusesPublisherAutoAccept(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "claim-owner", "claim-owner@example.com")
	siteID := createTestSite(t, db, "claim-site-key", "claim.example", "Claim")
	makerID, offerID := seedListedOffer(t, db, "claim-maker-key", "claim-maker@example.com", "Claim Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/offers/"+offerID+"/mode", map[string]any{"mode": "yolo"})
	if opened.Code != http.StatusOK {
		t.Fatalf("offer mode: %d %s", opened.Code, opened.Body.String())
	}
	api.Claims = entry.StubClaimChecker{}
	makerKey := map[string]string{"X-Maker-Key": "claim-maker-key"}
	refused, refusedBody := postJSON(t, r, http.MethodPost, "/v1/offers/"+offerID+"/accept", map[string]any{"site_id": siteID, "site_quality": 90, "niche": "devtools"}, makerKey)
	if refused.Code != http.StatusOK || refusedBody["decision"] != "needs_owner" || refusedBody["reason"] != entry.ClaimCheckReason || refusedBody["message"] != entry.ClaimCheckMessage {
		t.Fatalf("stub accept: %d %s", refused.Code, refused.Body.String())
	}
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM publisher_acceptances WHERE offer_id=$1`, offerID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stub stored an acceptance: %d %v", n, err)
	}
	mainText, err := os.ReadFile(filepath.Join(apiModuleRoot(t), "cmd/server/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainText), "entry.StubClaimChecker{}") || strings.Contains(string(mainText), "LiveClaimChecker") {
		t.Fatal("production claim checker is not the stub")
	}
	if !strings.Contains(string(mainText), "entry.PageClaimChecker{}") || !strings.Contains(string(mainText), "config.ClaimsReal") {
		t.Fatal("real mode must wire the page checker")
	}
}

func TestProjectKeyCannotAcknowledgePlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, _ := testOwnerSession(t, h, "ack-owner", "ack-owner@example.com")
	siteID := createTestSite(t, db, "ack-site-key", "ack.example", "Ack")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "ack-maker", "ack-maker@example.com", "Ack Product")
	projectKey := "or_ack_project_key"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_ackk',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://ack.example/post", "Ack Product note.")
	refused, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, map[string]string{"X-API-Key": projectKey})
	if refused.Code != http.StatusForbidden {
		t.Fatalf("project key ack: %d %s", refused.Code, refused.Body.String())
	}
	siteRefused, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, map[string]string{"X-API-Key": "ack-site-key"})
	if siteRefused.Code != http.StatusForbidden {
		t.Fatalf("site key ack: %d %s", siteRefused.Code, siteRefused.Body.String())
	}
	pluginKey := issuePluginKey(t, db, siteID, "ack-plugin-key")
	acked, body := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || body["plugin_state"] != "placed" {
		t.Fatalf("plugin key ack: %d %s", acked.Code, acked.Body.String())
	}
}

func TestBareLegacyCodeNeedsExactlyOnePlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	siteID := createTestSite(t, db, "legacy-site-key", "legacy.example", "Legacy")
	_, offerID := seedListedOffer(t, db, "legacy-maker", "legacy-maker@example.com", "Legacy Product")
	first := insertPlacement(t, db, siteID, offerID, "https://legacy.example/one", "Legacy Product one.")
	second := insertPlacement(t, db, siteID, offerID, "https://legacy.example/two", "Legacy Product two.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status)
VALUES($1,$2,'legacy-code',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	clicks := func() int {
		t.Helper()
		var n int
		if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE campaign_id=$1`, campaignID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	none := hitGo(t, r, http.MethodGet, "/v1/go/legacy-code", testBrowserUA)
	requirePlainRedirect(t, none, "https://example.com")
	if clicks() != 0 {
		t.Fatal("zero live placements recorded a hit")
	}
	markPlacementLive(t, db, siteID, offerID, first, "legacy-code", "https://legacy.example/one")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), campaign_id=$2, expires_at=NOW()+INTERVAL '2 days' WHERE id=$1`, second, campaignID); err != nil {
		t.Fatal(err)
	}
	both := hitGo(t, r, http.MethodGet, "/v1/go/legacy-code", testBrowserUA)
	requirePlainRedirect(t, both, "https://example.com")
	if clicks() != 0 {
		t.Fatal("two live placements recorded a hit")
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='removed' WHERE id=$1`, second); err != nil {
		t.Fatal(err)
	}
	one := hitGo(t, r, http.MethodGet, "/v1/go/legacy-code", testBrowserUA)
	if one.Code != http.StatusFound || clicks() != 1 {
		t.Fatalf("exactly one: %d clicks %d", one.Code, clicks())
	}
}

func TestDigestUndoRecordsActor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "actor-owner", "actor-owner@example.com")
	siteID := createTestSite(t, db, "actor-site-key", "actor.example", "Actor")
	makerID, offerID := seedListedOffer(t, db, "actor-maker", "actor-maker@example.com", "Actor Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	placementID := insertPlacement(t, db, siteID, offerID, "https://actor.example/post", "Actor Product note.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	rec, digest := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	var token string
	for _, raw := range digest["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == placementID {
			token = item["undo_token"].(string)
		}
	}
	if token == "" {
		t.Fatal("placement missing")
	}
	undone, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/digest/undo", map[string]any{"undo_token": token})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	var actor string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(actor,'') FROM owner_events WHERE kind='placement_undo' AND placement_id=$1`, placementID).Scan(&actor); err != nil || actor != owner.ID {
		t.Fatalf("actor %q %v", actor, err)
	}
}

func TestRetryFailedRestoreRechecksCurrentState(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "retry-owner", "retry-owner@example.com")
	siteID := createTestSite(t, db, "retry-site-key", "retry.example", "Retry")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "retry-maker", "retry-maker@example.com", "Retry Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "max_placements_per_30_days": 1, "min_relevance": 25})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	siteKey := map[string]string{"X-API-Key": "retry-site-key"}
	pluginKey := issuePluginKey(t, db, siteID, "retry-plugin-key")
	failRestore := func(surface string) string {
		t.Helper()
		id := insertPlacement(t, db, siteID, offerID, surface, "Retry Product is a solid fit.")
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', approved_by_kind='policy:yolo', approved_at=NOW(),
plugin_wait='', plugin_failed_reason='re-read failed', failed_action='restored', restore_unconfirmed=true, plugin_wait_at=NOW()
WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := failRestore("https://retry.example/first")
	retried, retriedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+first+"/retry", nil, siteKey)
	if retried.Code != http.StatusOK || retriedBody["plugin_state"] != "pending" {
		t.Fatalf("retry: %d %s", retried.Code, retried.Body.String())
	}
	var status, wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait FROM placement_proposals WHERE id=$1`, first).Scan(&status, &wait); err != nil || status != "committed" || wait != "restore" {
		t.Fatalf("retry stored %s %s %v", status, wait, err)
	}
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+first+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("ack: %d %s", acked.Code, acked.Body.String())
	}

	other := insertPlacement(t, db, siteID, offerID, "https://retry.example/other", "Retry Product other.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='policy:yolo' WHERE id=$1`, other); err != nil {
		t.Fatal(err)
	}
	second := failRestore("https://retry.example/second")
	blocked, blockedBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+second+"/retry", nil, siteKey)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "cap" {
		t.Fatalf("full cap: %d %s", blocked.Code, blocked.Body.String())
	}
	var action string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait, failed_action FROM placement_proposals WHERE id=$1`, second).Scan(&status, &wait, &action); err != nil || status != "committed" || action != "restored" || wait == "placed" {
		t.Fatalf("failed retry changed the row: %s %s %s %v", status, wait, action, err)
	}
}

func TestSetupSaveKeepsStoredMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "setup-owner", "setup-owner@example.com")
	siteID := createTestSite(t, db, "setup-site-key", "setup.example", "Setup")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	firstBody := map[string]any{"role": "publisher", "install_path": "cli", "mode": "auto", "mode_shown": "auto"}
	first, firstJSON := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", firstBody)
	if first.Code != http.StatusOK || firstJSON["mode"] != "auto" {
		t.Fatalf("first save: %d %s", first.Code, first.Body.String())
	}
	api.Claims = entry.StubClaimChecker{}
	againBody := map[string]any{"role": "publisher", "install_path": "cli", "mode": "manual", "mode_shown": "manual"}
	again, againJSON := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", againBody)
	if again.Code != http.StatusOK || againJSON["mode"] != "manual" || againJSON["approval_reason"] != entry.ClaimCheckReason || againJSON["approval_message"] != entry.ClaimCheckMessage {
		t.Fatalf("stub save: %d %s", again.Code, again.Body.String())
	}
	var stored, policy string
	if err := db.Pool.QueryRow(context.Background(), `SELECT mode FROM owner_setup WHERE owner_id=$1 AND role='publisher'`, owner.ID).Scan(&stored); err != nil || stored != "auto" {
		t.Fatalf("stored setup mode %s %v", stored, err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT mode FROM approval_policies WHERE site_id=$1`, siteID).Scan(&policy); err != nil || policy != "auto" {
		t.Fatalf("stored policy mode %s %v", policy, err)
	}
}

func TestPauseAndResumeReturnEffectiveMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "pause-owner", "pause-owner@example.com")
	siteID := createTestSite(t, db, "pause-site-key", "pause.example", "Pause")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if opened.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", opened.Code, opened.Body.String())
	}
	api.Claims = entry.StubClaimChecker{}
	paused, pausedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/pause", map[string]any{"scope": "site", "id": siteID})
	pausedRow := appliedRow(t, pausedBody)
	if paused.Code != http.StatusOK || pausedRow["effective_mode"] != "manual" || pausedRow["approval_reason"] != entry.ClaimCheckReason || pausedRow["approval_message"] != entry.ClaimCheckMessage {
		t.Fatalf("pause: %d %s", paused.Code, paused.Body.String())
	}
	resumed, resumedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account/resume", map[string]any{"scope": "site", "id": siteID})
	resumedRow := appliedRow(t, resumedBody)
	if resumed.Code != http.StatusOK || resumedRow["mode"] != "yolo" || resumedRow["effective_mode"] != "manual" || resumedRow["approval_reason"] != entry.ClaimCheckReason || resumedRow["approval_message"] != entry.ClaimCheckMessage {
		t.Fatalf("resume: %d %s", resumed.Code, resumed.Body.String())
	}
}

func TestDigestProjectTotalItems(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	prev := database.DigestPageSize
	database.DigestPageSize = 1
	t.Cleanup(func() { database.DigestPageSize = prev })
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "total-owner", "total-owner@example.com")
	siteID := createTestSite(t, db, "total-site-key", "total.example", "Total")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "total-maker", "total-maker@example.com", "Total Product")
	first := insertPlacement(t, db, siteID, offerID, "https://total.example/one", "Total Product one.")
	second := insertPlacement(t, db, siteID, offerID, "https://total.example/two", "Total Product two.")
	markPlacementLive(t, db, siteID, offerID, first, "total-go", "https://total.example/one")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), campaign_id=(SELECT campaign_id FROM placement_proposals WHERE id=$2), approved_at=NOW() WHERE id=$1`, second, first); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/account/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	var found bool
	for _, raw := range body["projects"].([]any) {
		project := raw.(map[string]any)
		if project["kind"] != "site" {
			if _, ok := project["total_items"]; !ok {
				t.Fatalf("project missing total_items: %s", rec.Body.String())
			}
			continue
		}
		found = true
		items := project["items"].([]any)
		if len(items) != 1 || project["total_items"] != float64(2) {
			t.Fatalf("page: items %d total %v", len(items), project["total_items"])
		}
	}
	if !found {
		t.Fatalf("site project missing: %s", rec.Body.String())
	}
}

// TestPluginAckMatchesMergedPluginPayloads replays the bodies Affilio_Plugin_Ack
// sends: header X-Plugin-Key, action plus rel_sponsored, and reason only on failed.
// The receipt body is the one Affilio_Distribution::publish posts, with a site-local /go/ link.
func TestPluginAckMatchesMergedPluginPayloads(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	r.POST("/v1/placements/:id/receipt", middleware.APIKeyAuth(db, nil), (&PlacementsHandler{DB: db}).RecordReceipt)
	owner, cookie := testOwnerSession(t, h, "contract-owner", "contract-owner@example.com")
	siteID := createTestSite(t, db, "contract-site-key", "blog.example", "Blog")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "contract-maker", "contract-maker@example.com", "Contract Product")
	pluginKey := issuePluginKey(t, db, siteID, "plugin-only-key")
	siteKey := map[string]string{"X-API-Key": "contract-site-key"}
	projectKey := "or_contract_project"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_cont',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAgentAccess(context.Background(), siteID, services.HashAPIKey("contract-agent-key")); err != nil {
		t.Fatal(err)
	}

	ack := func(id, raw string, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/placements/"+id+"/plugin-ack", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec, body
	}
	fresh := func(surface string) string {
		t.Helper()
		id := insertPlacement(t, db, siteID, offerID, surface, "Contract Product note.")
		if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	waitOf := func(id string) string {
		t.Helper()
		var wait string
		if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, id).Scan(&wait); err != nil {
			t.Fatal(err)
		}
		return wait
	}

	placedID := fresh("https://blog.example/placed")
	markPlacementLive(t, db, siteID, offerID, placedID, "contract-go", "https://blog.example/placed")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='' WHERE id=$1`, placedID); err != nil {
		t.Fatal(err)
	}
	placed, placedBody := ack(placedID, `{"action":"placed","rel_sponsored":true}`, pluginKey)
	if placed.Code != http.StatusOK || placedBody["plugin_state"] != "placed" || waitOf(placedID) != "placed" {
		t.Fatalf("placed ack: %d %s wait %s", placed.Code, placed.Body.String(), waitOf(placedID))
	}
	var live bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM placement_counts_as_live WHERE id=$1)`, placedID).Scan(&live); err != nil || !live {
		t.Fatalf("upgrade ack did not make the existing box live: %v %v", live, err)
	}

	deniedID := fresh("https://blog.example/denied")
	denied, deniedBody := ack(deniedID, `{"action":"placed","rel_sponsored":false}`, pluginKey)
	if denied.Code != http.StatusUnprocessableEntity || deniedBody["check_id"] != "rel_sponsored" || waitOf(deniedID) != "" {
		t.Fatalf("placed without rel: %d %s wait %q", denied.Code, denied.Body.String(), waitOf(deniedID))
	}

	removedID := fresh("https://blog.example/removed")
	revoked, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+removedID+"/revoke", nil, siteKey)
	if revoked.Code != http.StatusOK || waitOf(removedID) != "remove" {
		t.Fatalf("revoke: %d %s", revoked.Code, revoked.Body.String())
	}
	removed, removedBody := ack(removedID, `{"action":"removed","rel_sponsored":false}`, pluginKey)
	if removed.Code != http.StatusOK || removedBody["plugin_state"] != "removed" {
		t.Fatalf("removed ack: %d %s", removed.Code, removed.Body.String())
	}

	restoredID := fresh("https://blog.example/restored")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='restore', plugin_wait_at=NOW() WHERE id=$1`, restoredID); err != nil {
		t.Fatal(err)
	}
	restored, restoredBody := ack(restoredID, `{"action":"restored","rel_sponsored":true}`, pluginKey)
	if restored.Code != http.StatusOK || restoredBody["plugin_state"] != "placed" || waitOf(restoredID) != "placed" {
		t.Fatalf("restored ack: %d %s", restored.Code, restored.Body.String())
	}

	failedID := fresh("https://blog.example/failed")
	failed, failedBody := ack(failedID, `{"action":"failed","rel_sponsored":false,"reason":"re-read failed"}`, pluginKey)
	if failed.Code != http.StatusOK || failedBody["plugin_state"] != "failed" || failedBody["plugin_failed_reason"] != "re-read failed" || failedBody["failed_action"] != "restored" || waitOf(failedID) != "" {
		t.Fatalf("failed ack: %d %s wait %q", failed.Code, failed.Body.String(), waitOf(failedID))
	}

	refusedID := fresh("https://blog.example/refused")
	refusals := []map[string]string{
		{"X-API-Key": "contract-site-key"},
		{"X-API-Key": projectKey},
		{"X-API-Key": "contract-agent-key"},
		{"X-API-Key": "plugin-only-key"},
		{"Authorization": "Bearer contract-agent-key"},
		{"X-Maker-Key": "contract-maker"},
		{"X-Plugin-Key": "plugin-only-key", "X-API-Key": "contract-site-key"},
		{},
	}
	for _, headers := range refusals {
		rec, _ := ack(refusedID, `{"action":"placed","rel_sponsored":true}`, headers)
		if rec.Code != http.StatusForbidden || waitOf(refusedID) != "" {
			t.Fatalf("refusal %v: %d %s wait %q", headers, rec.Code, rec.Body.String(), waitOf(refusedID))
		}
	}
	session, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+refusedID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true})
	if session.Code != http.StatusForbidden || waitOf(refusedID) != "" {
		t.Fatalf("session ack: %d %s", session.Code, session.Body.String())
	}

	receiptID := fresh("https://blog.example/post")
	copy := "Contract Product note."
	disclosure := "(link sponsorizzato)"
	receipt := `{"tracked_link":"/v1/go/` + receiptID + `","publication_status":"awaiting_evidence"}`
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', receipt=$2::jsonb WHERE id=$1`, receiptID, receipt); err != nil {
		t.Fatal(err)
	}
	published := `<p class="affilio-placement-` + receiptID + `">` + copy + ` <a href="https://blog.example/go/` + receiptID + `" rel="sponsored nofollow">Vedi il prodotto</a> (link sponsorizzato)</p>` + "\n" + disclosure
	rawReceipt := `{"surface_url":"https://blog.example/post","published_text":` + jsonString(published) + `,"evidence":"WordPress administrator inserted the approved placement in published post 7"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/placements/"+receiptID+"/receipt", strings.NewReader(rawReceipt))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "contract-site-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("site-local receipt: %d %s", rec.Code, rec.Body.String())
	}
	var tracked string
	if err := db.Pool.QueryRow(context.Background(), `SELECT receipt->>'tracked_link' FROM placement_proposals WHERE id=$1`, receiptID).Scan(&tracked); err != nil || tracked != "/v1/go/"+receiptID {
		t.Fatalf("stored tracked link %q %v", tracked, err)
	}
}

func jsonString(s string) string {
	raw, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(raw)
}
