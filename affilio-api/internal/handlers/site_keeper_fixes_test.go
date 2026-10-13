package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
)

func testOneBoxVariant(t *testing.T, mutate func(article string) string) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-box-variant"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkTestOwner(t, db, siteID, "box-owner", "box-owner@example.com")
	r := publisherLegRouter(db, false)
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

	variant := mutate(article)
	secondPlace, secondBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": variant, "context": contextText, "copy": "InboxPilot is one tool for this inbox workflow.",
	})
	if secondPlace.Code != http.StatusConflict || secondBody["reason"] != "one_per_post" {
		t.Fatalf("second place %s: %d %v", variant, secondPlace.Code, secondBody)
	}

	var sibling, status string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO placement_proposals(site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,expires_at,post_url,status,approved_by_kind,approved_at)
SELECT site_id,offer_id,$2,context,$3,disclosure,reason,terms_snapshot,expires_at,$2,'approved','owner',NOW()
FROM placement_proposals WHERE id=$1 RETURNING id`, first, variant, "InboxPilot is a second box for this same article.").Scan(&sibling); err != nil {
		t.Fatal(err)
	}
	blocked, blockedBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+sibling+"/commit", siteKey, nil)
	if blocked.Code != http.StatusConflict || blockedBody["reason"] != "one_per_post" {
		t.Fatalf("variant commit %s: %d %v", variant, blocked.Code, blockedBody)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, sibling).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("variant commit wrote through: %s %v", status, err)
	}
}

func linkTestOwner(t *testing.T, db *database.DB, siteID, subject, email string) {
	t.Helper()
	h := &HostedHandler{DB: db, BaseURL: "http://127.0.0.1:8080"}
	owner, _ := testOwnerSession(t, h, subject, email)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
}

func ginSetTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

func TestOneBoxIgnoresTrailingSlash(t *testing.T) {
	testOneBoxVariant(t, func(article string) string { return article + "/" })
}

func TestOneBoxIgnoresQuery(t *testing.T) {
	testOneBoxVariant(t, func(article string) string { return article + "?utm=x" })
}

func TestOneBoxIgnoresFragment(t *testing.T) {
	testOneBoxVariant(t, func(article string) string { return article + "#section" })
}

func TestOneBoxIgnoresHostCase(t *testing.T) {
	testOneBoxVariant(t, func(article string) string {
		return strings.Replace(article, "https://publisher.example.com", "https://Publisher.Example.com", 1)
	})
}

func TestOneBoxMatchesWordPressPostID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-post-id"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkTestOwner(t, db, siteID, "post-id-owner", "post-id-owner@example.com")
	r := publisherLegRouter(db, false)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	copy := "Consider InboxPilot for an email inbox productivity workflow."
	rec, _ := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": contextText, "copy": copy, "post_id": 77,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("place: %d %s", rec.Code, rec.Body.String())
	}
	again, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/other-article", "context": contextText,
		"copy": "InboxPilot is one tool for this inbox workflow.", "wp_post_id": 77,
	})
	if again.Code != http.StatusConflict || body["reason"] != "one_per_post" {
		t.Fatalf("same post id: %d %v", again.Code, body)
	}
}

func TestClaimsCheckedOnPrepareAndCommit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-claims-copy"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkTestOwner(t, db, siteID, "claims-copy-owner", "claims-copy@example.com")
	r := publisherLegRouter(db, false)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	article := "https://publisher.example.com/claims"
	blocked, blockedBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": article, "context": contextText, "copy": "InboxPilot improves this workflow by 90%.",
	})
	if blocked.Code != http.StatusUnprocessableEntity || blockedBody["check_id"] != "claims_from_offer" || blockedBody["check_text"] != "Use only claims from the offer." {
		t.Fatalf("prepare 90%%: %d %v", blocked.Code, blockedBody)
	}
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1`, siteID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("90%% copy was stored: %d %v", n, err)
	}

	placed, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": article, "context": contextText, "copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if placed.Code != http.StatusCreated {
		t.Fatalf("honest place: %d %s", placed.Code, placed.Body.String())
	}
	id := prepared["placement"].(map[string]interface{})["id"].(string)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET copy='InboxPilot improves this workflow by 90%.' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	approved, _ := adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	commit, commitBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/commit", siteKey, nil)
	if commit.Code != http.StatusUnprocessableEntity || commitBody["check_id"] != "claims_from_offer" {
		t.Fatalf("commit 90%%: %d %v", commit.Code, commitBody)
	}
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, id).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("commit wrote the 90%% copy live: %s %v", status, err)
	}
}

func TestUndoRedoBeforePluginAckKeepsGoClicks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db, HomeURL: "https://offerrails.example/"}).Go)
	owner, cookie := testOwnerSession(t, h, "redo-click-owner", "redo-click@example.com")
	siteID := createTestSite(t, db, "redo-click-key", "redo-click.example", "Redo")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "redo-click-maker", "redo-click-maker@example.com", "Redo Product")
	id := insertPlacement(t, db, siteID, offerID, "https://redo-click.example/post", "Honest note about the product.")
	markPlacementLive(t, db, siteID, offerID, id, "redo-click-go", "https://redo-click.example/post")
	siteKey := map[string]string{"X-API-Key": "redo-click-key"}
	path := "/v1/go/" + id

	live := hitGo(t, r, http.MethodGet, path, "LiveReader/1")
	if live.Code != http.StatusFound || !strings.Contains(live.Header().Get("Location"), "via=") || placementClickCount(t, db, id) != 1 {
		t.Fatalf("live click: %d %s count %d", live.Code, live.Header().Get("Location"), placementClickCount(t, db, id))
	}
	undone, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", map[string]any{"reason": "undo"}, siteKey)
	if undone.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", undone.Code, undone.Body.String())
	}
	stopped := hitGo(t, r, http.MethodGet, path, "UndoReader/1")
	if stopped.Code != http.StatusFound || strings.Contains(stopped.Header().Get("Location"), "via=") || placementClickCount(t, db, id) != 1 {
		t.Fatalf("undo still counted: %d %s", stopped.Code, stopped.Header().Get("Location"))
	}
	restored, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/placements/"+id+"/restore", nil)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	polled, pollBody := postJSON(t, r, http.MethodGet, "/v1/placements/"+id, nil, siteKey)
	if polled.Code != http.StatusOK || pollBody["plugin_state"] != "placed" {
		t.Fatalf("redo before ack: %d %s", polled.Code, polled.Body.String())
	}
	again := hitGo(t, r, http.MethodGet, path, "RedoReader/2")
	if again.Code != http.StatusFound || !strings.Contains(again.Header().Get("Location"), "via=") || placementClickCount(t, db, id) != 2 {
		t.Fatalf("redo click: %d %s count %d", again.Code, again.Header().Get("Location"), placementClickCount(t, db, id))
	}
}

func TestUnknownGoLinkKeepsPort(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	r := gin.New()
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db, HomeURL: "https://offerrails.example/"}).Go)
	req := httptest.NewRequest(http.MethodGet, "/v1/go/00000000-0000-4000-8000-000000000099", nil)
	req.Host = "127.0.0.1:8443"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
		t.Fatalf("unknown id: %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") || !strings.Contains(rec.Body.String(), "This link is not on Offer Rails.") {
		t.Fatalf("404 page: %q %s", rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("unknown id dropped go-link headers")
	}
	code := httptest.NewRequest(http.MethodGet, "/v1/go/not-a-placement", nil)
	code.Host = "127.0.0.1:8443"
	codeRec := httptest.NewRecorder()
	r.ServeHTTP(codeRec, code)
	if codeRec.Code != http.StatusFound || codeRec.Header().Get("Location") != "https://offerrails.example/" {
		t.Fatalf("unknown code: %d %s", codeRec.Code, codeRec.Header().Get("Location"))
	}
}

func TestSetupSiteURLKeepsPort(t *testing.T) {
	if got, ok := domainFromSiteURL("https://publisher.example:8443/blog"); !ok || got != "publisher.example:8443" {
		t.Fatalf("domain %q ok=%v", got, ok)
	}
	if !validDomain("publisher.example:8443") || validDomain("publisher.example:0") {
		t.Fatal("port was not accepted on a stored domain")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "port-owner", "port-owner@example.com")
	siteID := createTestSite(t, db, "port-site-key", "publisher.example:8443", "Ported")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	if rec.Code != http.StatusOK || body["site_url"] != "https://publisher.example:8443/" {
		t.Fatalf("site_url: %d %v", rec.Code, body["site_url"])
	}
}

func TestSavedYoloKeepsPreselectedUntilFirstPlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.StubClaimChecker{}
	owner, cookie := testOwnerSession(t, h, "yolo-save-owner", "yolo-save@example.com")
	siteID := createTestSite(t, db, "yolo-save-key", "yolo-save.example", "Yolo")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='direct' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	saved, savedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", map[string]any{
		"role": "publisher", "install_path": "cli", "mode": "yolo", "mode_shown": "yolo",
	})
	if saved.Code != http.StatusOK || savedBody["mode"] != "manual" || savedBody["mode_preselected"] != "yolo" || savedBody["approval_reason"] != entry.YoloAfterFirstReason || savedBody["approval_message"] != entry.YoloAfterFirstMessage {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	got, gotBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	if got.Code != http.StatusOK || gotBody["mode"] != "manual" || gotBody["mode_preselected"] != "yolo" || gotBody["approval_reason"] != entry.YoloAfterFirstReason {
		t.Fatalf("get after save: %d %s", got.Code, got.Body.String())
	}
	_, offerID := seedListedOffer(t, db, "yolo-save-maker", "yolo-save-maker@example.com", "Yolo Product")
	id := insertPlacement(t, db, siteID, offerID, "https://yolo-save.example/post", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	after, afterBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	if after.Code != http.StatusOK || afterBody["mode"] != "manual" || afterBody["mode_preselected"] != "yolo" || afterBody["approval_reason"] != entry.ClaimCheckReason {
		t.Fatalf("after first placement: %d %s", after.Code, after.Body.String())
	}
}

func TestPluginAckForbiddenIncludesReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, _, r := newEntryTest(t, db)
	rec, body := postJSON(t, r, http.MethodPost, "/v1/placements/00000000-0000-4000-8000-0000000000aa/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-API-Key": "not-the-plugin-key"})
	if rec.Code != http.StatusForbidden || body["reason"] != "plugin_key_required" || body["message"] != "Send the plugin key in X-Plugin-Key." || body["error"] != "plugin key required" {
		t.Fatalf("api key: %d %s", rec.Code, rec.Body.String())
	}
	empty, emptyBody := postJSON(t, r, http.MethodPost, "/v1/placements/00000000-0000-4000-8000-0000000000aa/plugin-ack", map[string]any{"action": "placed"}, map[string]string{"X-Plugin-Key": "  "})
	if empty.Code != http.StatusForbidden || emptyBody["reason"] != "plugin_key_required" || emptyBody["message"] != "Send the plugin key in X-Plugin-Key." {
		t.Fatalf("empty plugin key: %d %s", empty.Code, empty.Body.String())
	}
}

func TestDigestPostTitleFromArticle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "title-owner", "title-owner@example.com")
	siteID := createTestSite(t, db, "title-site-key", "title.example", "Titles")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO articles(site_id, wp_post_id, title, url) VALUES($1, 9, 'How to ship', 'https://title.example/post/')`, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "title-maker", "title-maker@example.com", "Title Product")
	id := insertPlacement(t, db, siteID, offerID, "https://title.example/post", "Honest note about the product.")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), post_title='' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == id {
			if item["post_title"] != "How to ship" {
				t.Fatalf("post_title %v", item["post_title"])
			}
			return
		}
	}
	t.Fatal("placement missing from digest")
}

func TestDigestOfferIncludesReviewFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.ClaimsCheck = config.ClaimsStub
	owner, cookie := testOwnerSession(t, h, "digest-offer-owner", "digest-offer@example.com")
	makerID, listedID := seedListedOffer(t, db, "digest-offer-maker", "digest-offer-maker@example.com", "Listed Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	var submittedID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'submitted',commission_type,commission_value,landing_url FROM offers WHERE id=$1 RETURNING id`, listedID).Scan(&submittedID); err != nil {
		t.Fatal(err)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	seen := map[string]bool{}
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		if item["kind"] != "offer" {
			continue
		}
		if _, ok := item["status"]; !ok {
			t.Fatalf("status missing: %v", item)
		}
		if _, ok := item["review_reason"]; !ok {
			t.Fatalf("review_reason missing: %v", item)
		}
		if _, ok := item["message"]; !ok {
			t.Fatalf("message missing: %v", item)
		}
		if _, ok := item["plugin_failed_reason"]; ok {
			t.Fatalf("offer included plugin_failed_reason: %v", item)
		}
		id := item["id"].(string)
		seen[id] = true
		switch id {
		case listedID:
			if item["status"] != "listed" || item["review_reason"] != nil || item["message"] != nil {
				t.Fatalf("listed offer: %v", item)
			}
		case submittedID:
			if item["status"] != "submitted" || item["review_reason"] != "claims" || item["message"] != "The claims check is not live yet." {
				t.Fatalf("submitted offer: %v", item)
			}
		}
	}
	if !seen[listedID] || !seen[submittedID] {
		t.Fatalf("offers missing: %v", seen)
	}
}
