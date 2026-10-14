package handlers

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
)

func TestLiveGoLinkRecordsClickAndKeepsRef(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "live-go-site", "live-go.example", "Live Go")
	_, offerID := seedListedOffer(t, db, "live-go-maker", "live-go-maker@example.com", "Live Go Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url=$2, ref_param='via' WHERE id=$1`, offerID, "https://shop.example/buy?ref=partner"); err != nil {
		t.Fatal(err)
	}
	article := "https://live-go.example/the-article"
	id := insertPlacement(t, db, siteID, offerID, article, "Live Go Product is worth a look.")
	markPlacementLive(t, db, siteID, offerID, id, "live-go-camp", article)

	prev := landingReachable
	landingReachable = func(string) bool { return false }
	t.Cleanup(func() { landingReachable = prev })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db, IPSaltSecret: "live-go-salt"}).Go)
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+id, nil)
	req.RemoteAddr = "203.0.113.50:1000"
	req.Header.Set("User-Agent", "Mozilla/5.0 reader")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	requireGoLinkHeaders(t, rec)
	loc := rec.Header().Get("Location")
	if loc == article || strings.Contains(loc, "live-go.example") {
		t.Fatalf("redirected to the article: %s", loc)
	}
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != "https://shop.example/buy" {
		t.Fatalf("destination %s", loc)
	}
	if parsed.Query().Get("ref") != "partner" {
		t.Fatalf("ref was dropped: %s", loc)
	}
	var clickID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text FROM clicks WHERE placement_id=$1`, id).Scan(&clickID); err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("via") != clickID {
		t.Fatalf("via %q click %s location %s", parsed.Query().Get("via"), clickID, loc)
	}
	if n := clickCount(t, db); n != 1 {
		t.Fatalf("clicks %d", n)
	}
}

func TestConsistentClaimsPassRestore(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.StubClaimChecker{}
	current := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	api.Now = func() time.Time { return current }
	owner, cookie := testOwnerSession(t, h, "consistent-owner", "consistent-owner@example.com")
	siteID := createTestSite(t, db, "consistent-site-key", "consistent.example", "Consistent")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "consistent-maker", "consistent-maker@example.com", "Claims Product")
	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "manual", "min_relevance": 25})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	siteKey := map[string]string{"X-API-Key": "consistent-site-key"}
	placementID := insertPlacement(t, db, siteID, offerID, "https://consistent.example/post", "Claims Product is a solid fit.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_by_kind='owner' WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("consistent restore: %d %s", restored.Code, restored.Body.String())
	}

	badID := insertPlacement(t, db, siteID, offerID, "https://consistent.example/other", "Claims Product is a solid fit. 99%")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_by_kind='owner' WHERE id=$1`, badID); err != nil {
		t.Fatal(err)
	}
	undone, _ = postJSON(t, r, http.MethodPost, "/v1/placements/"+badID+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("bad undo: %d %s", undone.Code, undone.Body.String())
	}
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })
	current = current.Add(time.Second)
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+badID+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "claims_from_offer" {
		t.Fatalf("inconsistent restore: %d %s", blocked.Code, blocked.Body.String())
	}
	if !strings.Contains(logs.String(), "reason="+entry.ClaimReasonNumberNotInOffer) {
		t.Fatalf("log: %s", logs.String())
	}
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, badID).Scan(&status); err != nil || status != "revoked" {
		t.Fatalf("inconsistent restore wrote through: %s %v", status, err)
	}
}

func TestPublishRejectsSecondBoxOnPost(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-one-box"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	h := &HostedHandler{DB: db, BaseURL: "http://127.0.0.1:8080"}
	owner, cookie := testOwnerSession(t, h, "one-box-owner", "publisher@example.com")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	r := publisherLegRouter(db, false)
	current := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	api := &AgentEntry{DB: db, BaseURL: h.BaseURL, Owner: h.RequireOwnerSession, Claims: entry.LiveClaimChecker{}, Now: func() time.Time { return current }}
	api.Mount(r)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	copy := "Consider InboxPilot for an email inbox productivity workflow."
	article := "https://publisher.example.com/tutorial"
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": article, "context": contextText, "copy": copy,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("place: %d %s", rec.Code, rec.Body.String())
	}
	first := prepared["placement"].(map[string]interface{})["id"].(string)
	approved, _ := adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+first+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, _ := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+first+"/commit", siteKey, nil)
	if committed.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", committed.Code, committed.Body.String())
	}

	secondPlace, secondBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": article, "context": contextText, "copy": "InboxPilot is one tool for this inbox workflow.",
	})
	if secondPlace.Code != http.StatusConflict || secondBody["reason"] != "one_per_post" || secondBody["message"] != "Keep one box on this post." {
		t.Fatalf("second place: %d %v", secondPlace.Code, secondBody)
	}

	var sibling string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO placement_proposals(site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,expires_at,post_url,status,approved_by_kind,approved_at)
SELECT site_id,offer_id,surface_url,context,$2,disclosure,reason,terms_snapshot,expires_at,post_url,'approved','owner',NOW()
FROM placement_proposals WHERE id=$1 RETURNING id`, first, "InboxPilot is a second box for this same article.").Scan(&sibling); err != nil {
		t.Fatal(err)
	}
	publish, publishBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+sibling+"/commit", siteKey, nil)
	if publish.Code != http.StatusConflict || publishBody["reason"] != "one_per_post" || publishBody["message"] != "Keep one box on this post." {
		t.Fatalf("second publish: %d %v", publish.Code, publishBody)
	}
	var siblingStatus string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, sibling).Scan(&siblingStatus); err != nil || siblingStatus != "approved" {
		t.Fatalf("second publish wrote through: %s %v", siblingStatus, err)
	}
	pluginKey := issuePluginKey(t, db, siteID, "one-box-plugin")
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+sibling+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusUnprocessableEntity || ackBody["check_id"] != "one_per_post" {
		t.Fatalf("second place ack: %d %s", acked.Code, acked.Body.String())
	}

	other, otherBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/other", "context": contextText, "copy": copy,
	})
	if other.Code != http.StatusCreated {
		t.Fatalf("other article: %d %s", other.Code, other.Body.String())
	}
	otherID := otherBody["placement"].(map[string]interface{})["id"].(string)
	approved, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+otherID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("other approve: %d %s", approved.Code, approved.Body.String())
	}
	otherCommit, _ := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+otherID+"/commit", siteKey, nil)
	if otherCommit.Code != http.StatusOK {
		t.Fatalf("other publish: %d %s", otherCommit.Code, otherCommit.Body.String())
	}

	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+first+"/revoke", map[string]any{"reason": "undo"}, map[string]string{"X-API-Key": siteKey})
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	blocked, blockedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+first+"/restore", nil)
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "one_per_post" {
		t.Fatalf("restore while the second box exists: %d %s", blocked.Code, blocked.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='revoked' WHERE id=$1`, sibling); err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Second)
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+first+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore after the second box is gone: %d %s", restored.Code, restored.Body.String())
	}
}

func TestPlacementPayloadsIncludeSurfaceURL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.LiveClaimChecker{}
	owner, cookie := testOwnerSession(t, h, "surface-owner", "surface-owner@example.com")
	siteID := createTestSite(t, db, "surface-site-key", "surface.example", "Surface")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "surface-maker", "surface-maker@example.com", "Surface Product")
	article := "https://surface.example/the-post"
	placementID := insertPlacement(t, db, siteID, offerID, article, "Surface Product is a solid fit.")
	siteKey := map[string]string{"X-API-Key": "surface-site-key"}

	listed, listBody := postJSON(t, r, http.MethodGet, "/v1/placements", nil, siteKey)
	if listed.Code != http.StatusOK {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}
	items, _ := listBody["placements"].([]any)
	var item map[string]any
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == placementID {
			item = row
			break
		}
	}
	if item == nil || item["surface_url"] != article || item["status"] == nil || item["copy"] == nil || item["fallback_url"] == nil {
		t.Fatalf("list payload: %s", listed.Body.String())
	}

	pending, pendingBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+placementID, nil, siteKey)
	if pending.Code != http.StatusOK {
		t.Fatalf("pending: %d %s", pending.Code, pending.Body.String())
	}
	nested, _ := pendingBody["placement"].(map[string]any)
	for _, key := range []string{"id", "status", "plugin_state", "plugin_failed_reason", "failed_action", "fallback_url", "policy_rule_id", "policy_rule_text"} {
		if _, ok := pendingBody[key]; !ok {
			t.Fatalf("pending dropped %s: %s", key, pending.Body.String())
		}
	}
	if pendingBody["surface_url"] != article || nested["surface_url"] != article || pendingBody["plugin_state"] != "pending" {
		t.Fatalf("pending surface: %s", pending.Body.String())
	}

	saved, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	if saved.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", saved.Code, saved.Body.String())
	}
	decided, decision := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+placementID+"/decide", map[string]any{"relevance": 80, "position": "end", "rel_sponsored": true})
	if decided.Code != http.StatusOK || decision["surface_url"] != article || decision["decision"] != "approved" || decision["approved_by"] == nil || decision["policy_rule_id"] == nil || decision["policy_rule_text"] == nil || decision["policy_version"] == nil || decision["matched_rule"] == nil {
		t.Fatalf("decide: %d %s", decided.Code, decided.Body.String())
	}
}
