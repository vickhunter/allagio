package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func markPlacementLive(t *testing.T, db *database.DB, siteID, offerID, placementID, code, post string) {
	t.Helper()
	var campaignID string
	err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status)
VALUES($1,$2,$3,true,'active')
ON CONFLICT (offer_id, site_id) DO UPDATE SET status='active', requires_placement=true
RETURNING id`, offerID, siteID, code).Scan(&campaignID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), plugin_failed_reason='', failed_action='', restore_unconfirmed=false,
campaign_id=$2, expires_at=NOW()+INTERVAL '2 days', post_url=$3, approved_at=COALESCE(approved_at, NOW()),
matched_rule=COALESCE(NULLIF(matched_rule,''), 'hard_checks')
WHERE id=$1`, placementID, campaignID, post)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewYoloEnforcesCapMinAndIgnoresCaller(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "review-policy", "review-policy@example.com")
	siteID := createTestSite(t, db, "review-policy-key", "review-policy.example", "Review")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='cli' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "review-policy-maker", "review-policy-maker@example.com", "Review Product")
	siteKey := map[string]string{"X-API-Key": "review-policy-key"}

	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "min_relevance": 80})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	low := insertPlacement(t, db, siteID, offerID, "https://review-policy.example/low", "Honest note about the product.")
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+low+"/decide", map[string]any{"relevance": 100, "position": "end", "rel_sponsored": true, "offer_data": "caller offer"})
	if rec.Code != http.StatusUnprocessableEntity || body["check_id"] != "relevance_min" {
		t.Fatalf("yolo min relevance: %d %s", rec.Code, rec.Body.String())
	}

	cappedMode, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "min_relevance": 40, "max_placements_per_30_days": 0})
	if cappedMode.Code != http.StatusOK {
		t.Fatalf("cap mode: %d %s", cappedMode.Code, cappedMode.Body.String())
	}
	capped := insertPlacement(t, db, siteID, offerID, "https://review-policy.example/cap", "Review Product is worth a look.")
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+capped+"/decide", map[string]any{"relevance": 100, "position": "end", "rel_sponsored": true})
	if rec.Code != http.StatusUnprocessableEntity || body["check_id"] != "cap" {
		t.Fatalf("yolo cap: %d %s", rec.Code, rec.Body.String())
	}

	opened, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo", "min_relevance": 40, "max_placements_per_30_days": 3})
	if opened.Code != http.StatusOK {
		t.Fatalf("reopen: %d %s", opened.Code, opened.Body.String())
	}
	ignored := insertPlacement(t, db, siteID, offerID, "https://review-policy.example/ignore", "Review Product is worth a look.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET disclosure='See the product.' WHERE id=$1`, ignored); err != nil {
		t.Fatal(err)
	}
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+ignored+"/decide", map[string]any{"relevance": 1, "position": "end", "rel_sponsored": false, "offer_data": "not the offer"})
	if rec.Code != http.StatusOK || body["approved_by"] != "policy:yolo" {
		t.Fatalf("disclosure words do not decide rel_sponsored: %d %s", rec.Code, rec.Body.String())
	}

	api.Claims = entry.StubClaimChecker{}
	setup, setupBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher&install_path=chatgpt_dir", nil)
	if setup.Code != http.StatusOK || setupBody["mode_preselected"] != "yolo" || setupBody["mode"] != "manual" || setupBody["approval_reason"] != entry.ClaimCheckReason {
		t.Fatalf("stub setup: %d %s", setup.Code, setup.Body.String())
	}
	modeRec, modeBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/sites/"+siteID+"/mode", nil)
	if modeRec.Code != http.StatusOK || modeBody["mode"] != "manual" || modeBody["approval_reason"] != entry.ClaimCheckReason || modeBody["approval_message"] == "" {
		t.Fatalf("effective mode: %d %s", modeRec.Code, modeRec.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT mode FROM approval_policies WHERE site_id=$1`, siteID).Scan(&stored); err != nil || stored != "yolo" {
		t.Fatalf("stored mode %s %v", stored, err)
	}
	auto := insertPlacement(t, db, siteID, offerID, "https://review-policy.example/auto", "Another Review Product note.")
	rec, body = serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+auto+"/decide", map[string]any{"relevance": 90, "position": "end", "rel_sponsored": true})
	if rec.Code != http.StatusOK || body["decision"] != "needs_owner" || body["reason"] != entry.ClaimCheckReason || body["message"] != entry.ClaimCheckMessage {
		t.Fatalf("stub decide: %d %s", rec.Code, rec.Body.String())
	}
	digestRec, digest := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if digestRec.Code != http.StatusOK || digest["approval_reason"] != entry.ClaimCheckReason || digest["effective_mode"] != "manual" {
		t.Fatalf("stub digest: %d %s", digestRec.Code, digestRec.Body.String())
	}

	api.Claims = entry.LiveClaimChecker{}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed' WHERE id=$1`, ignored); err != nil {
		t.Fatal(err)
	}
	undo, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+ignored+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undo.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undo.Code, undo.Body.String())
	}
	restored, restoredBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+ignored+"/restore", nil)
	if restored.Code != http.StatusOK || restoredBody["status"] == "" {
		t.Fatalf("restore ignores disclosure words: %d %s", restored.Code, restored.Body.String())
	}
	pluginKey := issuePluginKey(t, db, siteID, "review-policy-plugin")
	relAck, relBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+ignored+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": false}, pluginKey)
	if relAck.Code != http.StatusUnprocessableEntity || relBody["check_id"] != "rel_sponsored" {
		t.Fatalf("ack rel: %d %s", relAck.Code, relAck.Body.String())
	}
	var wait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, ignored).Scan(&wait); err != nil || wait != "restore" {
		t.Fatalf("ack without rel stored %q %v", wait, err)
	}
}

func TestReviewProjectKeyAllowList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, _ := newEntryTest(t, db)
	owner, _ := testOwnerSession(t, h, "review-allow", "review-allow@example.com")
	siteID := createTestSite(t, db, "review-allow-site", "allow.example", "Allow")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	projectKey := "or_allow_project_key"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_allo',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	auth := middleware.APIKeyAuth(db, nil)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.POST("/v1/payouts/:id/confirm-received", auth, ok)
	r.POST("/v1/marketplace/offers/:id/join", auth, ok)
	r.POST("/v1/sites/owner-connection", auth, ok)
	r.POST("/v1/insert/confirm", auth, ok)
	paths := []string{
		"/v1/payouts/11111111-1111-1111-1111-111111111111/confirm-received",
		"/v1/marketplace/offers/11111111-1111-1111-1111-111111111111/join",
		"/v1/sites/owner-connection",
		"/v1/insert/confirm",
	}
	for _, path := range paths {
		rec, body := postJSON(t, r, http.MethodPost, path, map[string]any{}, map[string]string{"X-API-Key": projectKey})
		if rec.Code != http.StatusForbidden || !strings.Contains(body["error"].(string), "project key") {
			t.Fatalf("project %s: %d %s", path, rec.Code, rec.Body.String())
		}
		siteRec, _ := postJSON(t, r, http.MethodPost, path, map[string]any{}, map[string]string{"X-API-Key": "review-allow-site"})
		if siteRec.Code != http.StatusOK {
			t.Fatalf("site %s: %d %s", path, siteRec.Code, siteRec.Body.String())
		}
	}
}

func TestReviewPluginAckAndRevoke(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "review-plugin", "review-plugin@example.com")
	siteID := createTestSite(t, db, "review-plugin-site", "plugin.example", "Plugin")
	makerID, offerID := seedListedOffer(t, db, "review-plugin-maker", "review-plugin-maker@example.com", "Plugin Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	projectKey := "or_plugin_project_key"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,expires_at)
VALUES($1,'or_plug',$2,'site',$3,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(projectKey), owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAgentAccess(context.Background(), siteID, services.HashAPIKey("review-agent-key")); err != nil {
		t.Fatal(err)
	}
	created, createdBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/account-keys", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("account key: %d %s", created.Code, created.Body.String())
	}
	accountKey := createdBody["key"].(string)
	ackID := insertPlacement(t, db, siteID, offerID, "https://plugin.example/ack", "Plugin Product note.")
	for _, key := range []string{projectKey, accountKey, "review-agent-key", "review-plugin-site"} {
		rec, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+ackID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, map[string]string{"X-API-Key": key})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("ack %s: %d %s", key, rec.Code, rec.Body.String())
		}
	}
	pluginKey := issuePluginKey(t, db, siteID, "review-plugin-only")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed' WHERE id=$1`, ackID); err != nil {
		t.Fatal(err)
	}
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+ackID+"/plugin-ack", map[string]any{"action": "restored", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("site ack: %d %s", acked.Code, acked.Body.String())
	}

	plain := insertPlacement(t, db, siteID, offerID, "https://plugin.example/plain", "Plugin Product plain.")
	revoked, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+plain+"/revoke", nil, map[string]string{"X-API-Key": "review-plugin-site"})
	if revoked.Code != http.StatusOK {
		t.Fatalf("plain revoke: %d %s", revoked.Code, revoked.Body.String())
	}
	var wait, before, status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait, COALESCE(status_before_undo,''), status FROM placement_proposals WHERE id=$1`, plain).Scan(&wait, &before, &status); err != nil || wait != "remove" || before != "" || status != "revoked" {
		t.Fatalf("plain revoke row %s %s %s %v", wait, before, status, err)
	}

	var otherOffer string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'listed','percentage',30,'https://example.com' FROM offers WHERE id=$1 RETURNING id`, offerID).Scan(&otherOffer); err != nil {
		t.Fatal(err)
	}
	offerKey := "or_offer_scope_key"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,offer_id,maker_id,expires_at)
VALUES($1,'or_offe',$2,'offer',$3,$4,NOW()+INTERVAL '1 hour')`, services.HashAPIKey(offerKey), owner.ID, offerID, makerID); err != nil {
		t.Fatal(err)
	}
	other := insertPlacement(t, db, siteID, otherOffer, "https://plugin.example/other", "Other product note.")
	own := insertPlacement(t, db, siteID, offerID, "https://plugin.example/own", "Plugin Product own.")
	blocked, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+other+"/revoke", nil, map[string]string{"X-Maker-Key": offerKey})
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("other offer revoke: %d %s", blocked.Code, blocked.Body.String())
	}
	ownRevoke, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+own+"/revoke", nil, map[string]string{"X-Maker-Key": offerKey})
	if ownRevoke.Code != http.StatusOK {
		t.Fatalf("own offer revoke: %d %s", ownRevoke.Code, ownRevoke.Body.String())
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, own).Scan(&wait); err != nil || wait != "remove" {
		t.Fatalf("maker revoke wait %s", wait)
	}
	r.GET("/v1/maker/earnings", middleware.MakerKeyAuth(db), (&PayoutsHandler{DB: db}).MakerEarnings)
	scoped, scopedBody := postJSON(t, r, http.MethodGet, "/v1/maker/earnings", nil, map[string]string{"X-Maker-Key": offerKey})
	if scoped.Code != http.StatusOK {
		t.Fatalf("scoped earnings: %d %s", scoped.Code, scoped.Body.String())
	}
	offers := scopedBody["offers"].([]any)
	if len(offers) != 1 || offers[0].(map[string]any)["offer_id"] != offerID {
		t.Fatalf("scoped offers: %s", scoped.Body.String())
	}
	full, fullBody := postJSON(t, r, http.MethodGet, "/v1/maker/earnings", nil, map[string]string{"X-Maker-Key": "review-plugin-maker"})
	if full.Code != http.StatusOK || len(fullBody["offers"].([]any)) != 2 {
		t.Fatalf("maker earnings: %d %s", full.Code, full.Body.String())
	}
}

func TestReviewDigestCountsAndPolicyRule(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	prev := database.DigestPageSize
	database.DigestPageSize = 1
	t.Cleanup(func() { database.DigestPageSize = prev })
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "review-digest", "review-digest@example.com")
	siteID := createTestSite(t, db, "review-digest-site", "digest-review.example", "Digest")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "review-digest-maker", "review-digest-maker@example.com", "Digest Product")
	first := insertPlacement(t, db, siteID, offerID, "https://digest-review.example/one", "Digest Product one.")
	second := insertPlacement(t, db, siteID, offerID, "https://digest-review.example/two", "Digest Product two.")
	markPlacementLive(t, db, siteID, offerID, first, "digest-review-go", "https://digest-review.example/one")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), campaign_id=(SELECT campaign_id FROM placement_proposals WHERE id=$2), expires_at=NOW()+INTERVAL '2 days', matched_rule='hard_checks', approved_at=NOW() WHERE id=$1`, second, first); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK || body["went_live"] != float64(2) || len(body["items"].([]any)) != 1 {
		t.Fatalf("digest page: %d %s", rec.Code, rec.Body.String())
	}
	item := body["items"].([]any)[0].(map[string]any)
	if item["policy_rule_id"] != "hard_checks_passed" || item["policy_rule_text"] != "Passed every hard check." {
		t.Fatalf("digest rule: %s", rec.Body.String())
	}
	statusRec, statusBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+first, nil, map[string]string{"X-API-Key": "review-digest-site"})
	if statusRec.Code != http.StatusOK || statusBody["policy_rule_id"] != "hard_checks_passed" || statusBody["policy_rule_text"] != "Passed every hard check." {
		t.Fatalf("placement read: %d %s", statusRec.Code, statusRec.Body.String())
	}
}

func TestReviewCampaignPathAndHeadProbe(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "review-go-site", "go-review.example", "Go")
	_, offerID := seedListedOffer(t, db, "review-go-maker", "review-go-maker@example.com", "Go Product")
	id := insertPlacement(t, db, siteID, offerID, "https://go-review.example/post", "Go Product is worth a look.")
	var campaignID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement,status) VALUES($1,$2,'review-camp',true,'active') RETURNING id`, offerID, siteID).Scan(&campaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', campaign_id=$2, expires_at=NOW()+INTERVAL '2 days', post_url='https://go-review.example/post' WHERE id=$1`, id, campaignID); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	redirects := &RedirectsHandler{DB: db, IPSaltSecret: "review-salt"}
	r.GET("/v1/go/:code", redirects.Go)

	hit := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "203.0.113.40:1000"
		req.Header.Set("User-Agent", testBrowserUA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	pending := hit("/v1/go/review-camp?placement=" + id)
	requirePlainRedirect(t, pending, "https://example.com")
	if n := clickCount(t, db); n != 0 {
		t.Fatalf("unplaced query recorded %d", n)
	}
	bare := hit("/v1/go/review-camp")
	requirePlainRedirect(t, bare, "https://example.com")
	markPlacementLive(t, db, siteID, offerID, id, "review-camp", "https://go-review.example/post")
	live := hit("/v1/go/review-camp")
	if live.Code != http.StatusFound || !strings.Contains(live.Header().Get("Location"), "via=") {
		t.Fatalf("live campaign: %d %s", live.Code, live.Header().Get("Location"))
	}
	requireGoLinkHeaders(t, live)
	if n := clickCount(t, db); n != 1 {
		t.Fatalf("live campaign clicks %d", n)
	}
	unknown := hit("/v1/go/missing-code")
	requirePlainRedirect(t, unknown, "https://offerrails.com/")
	if n := clickCount(t, db); n != 1 {
		t.Fatalf("unknown code clicks %d", n)
	}

	prev := landingReachable
	landingReachable = func(string) bool { return false }
	t.Cleanup(func() { landingReachable = prev })
	second := insertPlacement(t, db, siteID, offerID, "https://go-review.example/other", "Go Product other note.")
	var otherCampaign string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM campaigns WHERE tracking_code='review-camp'`).Scan(&otherCampaign); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='committed', plugin_wait='placed', plugin_wait_at=NOW(), campaign_id=$2, expires_at=NOW()+INTERVAL '2 days', post_url='https://go-review.example/other' WHERE id=$1`, second, otherCampaign); err != nil {
		t.Fatal(err)
	}
	failed := hit("/v1/go/" + second)
	loc := failed.Header().Get("Location")
	if failed.Code != http.StatusFound || !strings.HasPrefix(loc, "https://example.com?") || !strings.Contains(loc, "via=") || loc == "https://go-review.example/other" {
		t.Fatalf("live go link with a down probe: %d %s", failed.Code, loc)
	}
	requireGoLinkHeaders(t, failed)
	if n := clickCount(t, db); n != 2 {
		t.Fatalf("live go link clicks %d", n)
	}
}

func TestReviewClickSignalsPurge(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "review-purge-site", "purge.example", "Purge")
	var oldID, newID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO clicks(site_id,clicked_at,ip_hash,user_agent,referrer) VALUES($1,NOW()-INTERVAL '31 days','old-hash','OldAgent','https://old.example') RETURNING id`, siteID).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO clicks(site_id,clicked_at,ip_hash,user_agent,referrer) VALUES($1,NOW(),'new-hash','NewAgent','https://new.example') RETURNING id`, siteID).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go PurgeClickSignals(ctx, db)
	deadline := time.Now().Add(3 * time.Second)
	for {
		var hash *string
		if err := db.Pool.QueryRow(context.Background(), `SELECT ip_hash FROM clicks WHERE id=$1`, oldID).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if hash == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("old click signals were kept")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var agent, referrer *string
	if err := db.Pool.QueryRow(context.Background(), `SELECT user_agent, referrer FROM clicks WHERE id=$1`, oldID).Scan(&agent, &referrer); err != nil || agent != nil || referrer != nil {
		t.Fatalf("old signals agent %v referrer %v err %v", agent, referrer, err)
	}
	var keptHash, keptAgent, keptRef string
	if err := db.Pool.QueryRow(context.Background(), `SELECT ip_hash, user_agent, referrer FROM clicks WHERE id=$1`, newID).Scan(&keptHash, &keptAgent, &keptRef); err != nil || keptHash != "new-hash" || keptAgent != "NewAgent" || keptRef != "https://new.example" {
		t.Fatalf("new click %s %s %s %v", keptHash, keptAgent, keptRef, err)
	}
}
