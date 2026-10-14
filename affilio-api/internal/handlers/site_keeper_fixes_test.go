package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.StubClaimChecker{}
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db, HomeURL: "https://offerrails.example/"}).Go)
	owner, cookie := testOwnerSession(t, h, "redo-click-owner", "redo-click@example.com")
	siteID := createTestSite(t, db, "redo-click-key", "redo-click.example", "Redo")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "redo-click-maker", "redo-click-maker@example.com", "Redo Product")
	id := insertPlacement(t, db, siteID, offerID, "https://redo-click.example/post", "Honest note about the product. The offer pays 30%.")
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
	var pluginWait string
	if err := db.Pool.QueryRow(context.Background(), `SELECT plugin_wait FROM placement_proposals WHERE id=$1`, id).Scan(&pluginWait); err != nil {
		t.Fatal(err)
	}
	if polled.Code != http.StatusOK || pollBody["plugin_state"] != "placed" || pluginWait != "placed" {
		t.Fatalf("redo before ack: %d wait %s %s", polled.Code, pluginWait, polled.Body.String())
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

func TestStubUntrustedNinetyPercentNeverGoesLive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	_, makerKey := createTestMaker(t, db)
	makers := newMakerRouterWithClaims(db, config.ClaimsStub)
	productID, heldID := createDraftOffer(t, makers, makerKey, "InboxPilot", "https://inboxpilot.example.com", "Readers save 90% of their time.", 20)
	markProductReady(t, db, productID)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET name='InboxPilot', primary_category='productivity', keywords=$2 WHERE id=$1`, productID, []string{"email", "inbox", "productivity"}); err != nil {
		t.Fatal(err)
	}
	submitted, submittedBody := doJSON(t, makers, http.MethodPost, "/v1/maker/offers/"+heldID+"/submit", makerKey, nil)
	if submitted.Code != http.StatusOK || submittedBody["status"] != "submitted" || submittedBody["review_reason"] != "claims" {
		t.Fatalf("stub submit: %d %v", submitted.Code, submittedBody)
	}
	blocked, blockedBody := adminJSON(t, makers, http.MethodPost, "/v1/offer-review/"+heldID+"/approve", nil)
	if blocked.Code != http.StatusConflict || blockedBody["review_reason"] != "claims" || blockedBody["status"] != "submitted" {
		t.Fatalf("publish 90%%: %d %v", blocked.Code, blockedBody)
	}
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM offers WHERE id=$1`, heldID).Scan(&status); err != nil || status != "submitted" {
		t.Fatalf("90%% offer listed: %s %v", status, err)
	}

	honestProduct, honestID := createDraftOffer(t, makers, makerKey, "InboxPilot Notes", "https://inboxpilot.example.com", "InboxPilot pays 20 percent to publishers.", 20)
	markProductReady(t, db, honestProduct)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET name='InboxPilot', primary_category='productivity', keywords=$2 WHERE id=$1`, honestProduct, []string{"email", "inbox", "productivity"}); err != nil {
		t.Fatal(err)
	}
	held, heldBody := doJSON(t, makers, http.MethodPost, "/v1/maker/offers/"+honestID+"/submit", makerKey, nil)
	if held.Code != http.StatusOK || heldBody["status"] != "submitted" {
		t.Fatalf("honest stub submit: %d %v", held.Code, heldBody)
	}
	published, publishedBody := adminJSON(t, makers, http.MethodPost, "/v1/offer-review/"+honestID+"/approve", nil)
	if published.Code != http.StatusOK || publishedBody["status"] != "listed" {
		t.Fatalf("honest publish: %d %v", published.Code, publishedBody)
	}

	siteKey := "site-key-stub-90"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkTestOwner(t, db, siteID, "stub-90-owner", "stub-90@example.com")
	publishers := publisherLegRouter(db, false)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	article := "https://publisher.example.com/stub-claims"
	unlisted, unlistedBody := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": heldID, "surface_url": article, "context": contextText, "copy": "InboxPilot improves this workflow by 90%.",
	})
	if unlisted.Code != http.StatusConflict || unlistedBody["reason"] != "offer_ineligible" {
		t.Fatalf("unlisted 90%% offer: %d %v", unlisted.Code, unlistedBody)
	}
	copyBlocked, copyBody := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": honestID, "surface_url": article, "context": contextText, "copy": "InboxPilot improves this workflow by 90%.",
	})
	if copyBlocked.Code != http.StatusUnprocessableEntity || copyBody["check_id"] != "claims_from_offer" {
		t.Fatalf("prepare listed 90%%: %d %v", copyBlocked.Code, copyBody)
	}
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1`, siteID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("90%% copy was stored: %d %v", n, err)
	}
	placed, prepared := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": honestID, "surface_url": article, "context": contextText, "copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if placed.Code != http.StatusCreated {
		t.Fatalf("honest place: %d %s", placed.Code, placed.Body.String())
	}
	id := prepared["placement"].(map[string]interface{})["id"].(string)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET copy='InboxPilot improves this workflow by 90%.' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	approved, _ := adminJSON(t, publishers, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	commit, commitBody := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements/"+id+"/commit", siteKey, nil)
	if commit.Code != http.StatusUnprocessableEntity || commitBody["check_id"] != "claims_from_offer" {
		t.Fatalf("commit 90%%: %d %v", commit.Code, commitBody)
	}
	var placementStatus string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, id).Scan(&placementStatus); err != nil || placementStatus != "approved" {
		t.Fatalf("commit wrote the 90%% copy live: %s %v", placementStatus, err)
	}
}

func TestOneBoxPrepareIsAtomic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-box-race"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkTestOwner(t, db, siteID, "box-race-owner", "box-race@example.com")
	r := publisherLegRouter(db, false)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	article := "https://publisher.example.com/race"
	var wg sync.WaitGroup
	codes := make([]int, 2)
	reasons := make([]string, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
				"offer_id": offerID, "surface_url": article, "context": contextText,
				"copy": "Consider InboxPilot for an email inbox productivity workflow.",
			})
			codes[i] = rec.Code
			if reason, _ := body["reason"].(string); reason != "" {
				reasons[i] = reason
			}
		}(i)
	}
	close(start)
	wg.Wait()
	created, conflict := 0, 0
	for i := 0; i < 2; i++ {
		switch codes[i] {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			if reasons[i] == "one_per_post" {
				conflict++
			}
		}
	}
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1`, siteID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if created != 1 || conflict != 1 || n != 1 {
		t.Fatalf("concurrent prepare created=%d conflict=%d rows=%d codes=%v reasons=%v", created, conflict, n, codes, reasons)
	}
}

func itemsFromDigest(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	var out []map[string]any
	if raw, ok := body["items"].([]any); ok {
		for _, item := range raw {
			out = append(out, item.(map[string]any))
		}
		return out
	}
	projects, _ := body["projects"].([]any)
	for _, raw := range projects {
		project := raw.(map[string]any)
		items, _ := project["items"].([]any)
		for _, item := range items {
			out = append(out, item.(map[string]any))
		}
	}
	return out
}

func requireDigestTitleAndSurface(t *testing.T, items []map[string]any) {
	t.Helper()
	if len(items) == 0 {
		t.Fatal("digest has no items")
	}
	for _, item := range items {
		title, _ := item["post_title"].(string)
		surface, _ := item["surface_url"].(string)
		position, _ := item["position"].(string)
		if title == "" || surface == "" || position == "" {
			t.Fatalf("item missing post_title, surface_url, or position: %v", item)
		}
	}
}

func findDigestItem(items []map[string]any, id string) map[string]any {
	for _, item := range items {
		if item["id"] == id {
			return item
		}
	}
	return nil
}

func TestDigestTitleAndSurfaceFromRealPlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "place-title-owner", "place-title@example.com")
	siteID := createTestSite(t, db, "place-title-site", "digest-place.example", "Digest Place")
	makerID, offerID := seedListedOffer(t, db, "place-title-maker", "place-title-maker@example.com", "Place Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	const surface = "https://digest-place.example/guide"
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO articles(site_id, wp_post_id, title, url) VALUES($1, 41, 'Guide to shipping', $2)`, siteID, surface+"/"); err != nil {
		t.Fatal(err)
	}
	created, err := db.CreatePlacement(context.Background(), database.Placement{
		SiteID: siteID, OfferID: offerID, SurfaceURL: surface,
		Context:    "A section about shipping the box to readers today.",
		Copy:       "Honest note about the product.",
		Disclosure: "Affiliate disclosure: I may earn a commission if you buy through this link.",
		Reason:     "The product fits this guide.",
		ExpiresAt:  time.Now().Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	var storedTitle string
	if err = db.Pool.QueryRow(context.Background(), `SELECT post_title FROM placement_proposals WHERE id=$1`, created.ID).Scan(&storedTitle); err != nil || storedTitle != "Guide to shipping" {
		t.Fatalf("placement stored %q (%v); the test did not set the title", storedTitle, err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET status='approved', approved_at=NOW(), approved_by_kind='owner' WHERE id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.CommitPlacement(context.Background(), siteID, created.ID, "place-title-go", entry.StubClaimChecker{}); err != nil {
		t.Fatal(err)
	}
	var receiptTitle, receiptSurface string
	if err = db.Pool.QueryRow(context.Background(), `SELECT receipt->>'post_title', receipt->>'surface_url' FROM placement_proposals WHERE id=$1`, created.ID).Scan(&receiptTitle, &receiptSurface); err != nil || receiptTitle != "Guide to shipping" || receiptSurface != surface {
		t.Fatalf("receipt title %q surface %q (%v)", receiptTitle, receiptSurface, err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET post_title='' WHERE id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `DELETE FROM articles WHERE site_id=$1`, siteID); err != nil {
		t.Fatal(err)
	}

	check := func(path string) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		items := itemsFromDigest(t, body)
		requireDigestTitleAndSurface(t, items)
		item := findDigestItem(items, created.ID)
		if item == nil || item["post_title"] != "Guide to shipping" || item["surface_url"] != surface || item["position"] != "end" {
			t.Fatalf("%s placement: %v", path, item)
		}
		offer := findDigestItem(items, offerID)
		if offer == nil || offer["post_title"] != "Guide to shipping" || offer["surface_url"] != surface || offer["position"] != "end" {
			t.Fatalf("%s offer: %v", path, offer)
		}
	}
	check("/v1/digest")
	check("/v1/account/digest")
}

func TestAccountDigestKeepsHeldOfferAcrossOpens(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.ClaimsCheck = config.ClaimsStub
	owner, cookie := testOwnerSession(t, h, "held-digest-owner", "held-digest@example.com")
	makerID, listedID := seedListedOffer(t, db, "held-digest-maker", "held-digest-maker@example.com", "Held Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET maker_id=$2 WHERE id=$1`, owner.ID, makerID); err != nil {
		t.Fatal(err)
	}
	var heldID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'submitted',commission_type,commission_value,landing_url FROM offers WHERE id=$1 RETURNING id`, listedID).Scan(&heldID); err != nil {
		t.Fatal(err)
	}
	past := "2026-06-01 08:00:00+00"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET updated_at = ($2::timestamptz AT TIME ZONE current_setting('TimeZone')) WHERE id=$1 OR id=$3`, listedID, past, heldID); err != nil {
		t.Fatal(err)
	}

	open := func() {
		t.Helper()
		rec, _ := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPost, "/v1/events", map[string]any{"kind": "digest_opened"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
		}
	}
	assertHeld := func(path string, wantListed bool) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		items := itemsFromDigest(t, body)
		requireDigestTitleAndSurface(t, items)
		held := findDigestItem(items, heldID)
		if held == nil || held["status"] != "submitted" || held["post_title"] != "Held Product" || held["surface_url"] != "https://example.com" || held["position"] != "end" {
			t.Fatalf("%s held: %v", path, held)
		}
		listed := findDigestItem(items, listedID)
		if wantListed && listed == nil {
			t.Fatalf("%s missing listed offer", path)
		}
		if !wantListed && listed != nil {
			t.Fatalf("%s still lists the resolved window: %v", path, listed)
		}
	}

	assertHeld("/v1/account/digest", true)
	assertHeld("/v1/digest", true)
	open()
	assertHeld("/v1/account/digest", true)
	assertHeld("/v1/digest", false)
	open()
	assertHeld("/v1/account/digest", true)
	assertHeld("/v1/digest", false)
}

func TestRefusedReceiptReopensPrepare(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-refused-receipt"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	surface := "https://publisher.example.com/tutorial"
	copy := "Consider InboxPilot for an email inbox productivity workflow."
	prepare := func() (int, map[string]interface{}) {
		t.Helper()
		rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
			"offer_id": offerID, "surface_url": surface, "context": "InboxPilot email inbox productivity tutorial for developers", "copy": copy,
		})
		return rec.Code, body
	}
	code, prepared := prepare()
	if code != http.StatusCreated {
		t.Fatalf("prepare: %d %v", code, prepared)
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	approved, _ := adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, committedBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %v", committed.Code, committedBody)
	}
	refused, refusedBody := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/receipt", siteKey, map[string]interface{}{
		"surface_url": surface, "published_text": "This post does not contain the approved box.", "evidence": "The publisher refused this receipt.",
	})
	if refused.Code != http.StatusConflict || refusedBody["reason"] != "receipt_mismatch" {
		t.Fatalf("refuse: %d %v", refused.Code, refusedBody)
	}
	var status, publication string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, COALESCE(receipt->>'publication_status','') FROM placement_proposals WHERE id=$1`, placementID).Scan(&status, &publication); err != nil || status != "pending" || publication != "refused" {
		t.Fatalf("after refuse status=%s publication=%s err=%v", status, publication, err)
	}
	again, againBody := prepare()
	if again != http.StatusCreated {
		t.Fatalf("prepare after refuse: %d %v", again, againBody)
	}
	nextID, _ := againBody["placement"].(map[string]interface{})["id"].(string)
	if nextID == "" || nextID == placementID || againBody["reason"] == "one_per_post" {
		t.Fatalf("expected a new box, got %v", againBody)
	}
}

func assertYoloHeld(t *testing.T, rec *httptest.ResponseRecorder, body map[string]any) {
	t.Helper()
	if rec.Code != http.StatusOK || body["mode"] != "manual" || body["mode_preselected"] != "yolo" || body["approval_reason"] != entry.YoloAfterFirstReason || body["approval_message"] != entry.YoloAfterFirstMessage {
		t.Fatalf("yolo hold: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOwnerSetupSavesYoloUntilFirstPlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.StubClaimChecker{}
	owner, cookie := testOwnerSession(t, h, "yolo-setup-route", "yolo-setup-route@example.com")
	siteID := createTestSite(t, db, "yolo-setup-route-key", "yolo-setup-route.example", "Yolo Setup")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='direct' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	saved, savedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/setup/settings", map[string]any{
		"role": "publisher", "install_path": "cli", "mode": "yolo", "mode_shown": "yolo",
	})
	assertYoloHeld(t, saved, savedBody)
	got, gotBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/setup/settings?role=publisher", nil)
	assertYoloHeld(t, got, gotBody)
}

func TestSiteModeSavesYoloUntilFirstPlacement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.Claims = entry.StubClaimChecker{}
	owner, cookie := testOwnerSession(t, h, "yolo-site-route", "yolo-site-route@example.com")
	siteID := createTestSite(t, db, "yolo-site-route-key", "yolo-site-route.example", "Yolo Site")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, source='direct' WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	saved, savedBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodPut, "/v1/sites/"+siteID+"/mode", map[string]any{"mode": "yolo"})
	assertYoloHeld(t, saved, savedBody)
	got, gotBody := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/sites/"+siteID+"/mode", nil)
	assertYoloHeld(t, got, gotBody)
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT mode FROM approval_policies WHERE site_id=$1`, siteID).Scan(&stored); err != nil || stored != "yolo" {
		t.Fatalf("stored mode %s %v", stored, err)
	}
}

func TestAdminApproveMatchesOfferClaimsText(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ginSetTest(t)
	_, makerKey := createTestMaker(t, db)
	makers := newMakerRouterWithClaims(db, config.ClaimsStub)
	productID, heldID := createDraftOffer(t, makers, makerKey, "InboxPilot", "https://inboxpilot.example.com", "Readers save 90% of their time.", 20)
	markProductReady(t, db, productID)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET name='InboxPilot', primary_category='productivity', keywords=$2 WHERE id=$1`, productID, []string{"email", "inbox", "productivity"}); err != nil {
		t.Fatal(err)
	}
	submitted, submittedBody := doJSON(t, makers, http.MethodPost, "/v1/maker/offers/"+heldID+"/submit", makerKey, nil)
	if submitted.Code != http.StatusOK || submittedBody["status"] != "submitted" {
		t.Fatalf("submit: %d %v", submitted.Code, submittedBody)
	}
	row, rowBody := doJSON(t, makers, http.MethodGet, "/v1/maker/offers/"+heldID, makerKey, nil)
	if row.Code != http.StatusOK || rowBody["message"] != reviewClaimsNotLive || rowBody["review_reason"] != reviewReasonClaims {
		t.Fatalf("offer row: %d %v", row.Code, rowBody)
	}
	blocked, blockedBody := adminJSON(t, makers, http.MethodPost, "/v1/offer-review/"+heldID+"/approve", nil)
	if blocked.Code != http.StatusConflict || blockedBody["message"] != rowBody["message"] || blockedBody["review_reason"] != rowBody["review_reason"] || blockedBody["status"] != "submitted" {
		t.Fatalf("approve: %d %v row %v", blocked.Code, blockedBody, rowBody["message"])
	}
}

func TestDigestPostTitleIsArticleNotCopy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "article-title-owner", "article-title@example.com")
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-article-title"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2, maker_id=$3 WHERE id=$1`, owner.ID, siteID, makerID); err != nil {
		t.Fatal(err)
	}
	publishers := publisherLegRouter(db, true)
	const (
		surface = "https://publisher.example.com/how-to-ship"
		title   = "How to ship a WordPress post"
		copy    = "Consider InboxPilot for an email inbox productivity workflow."
		postID  = 88
	)
	prepared, preparedBody := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements", siteKey, map[string]any{
		"offer_id": offerID, "surface_url": surface, "context": "InboxPilot email inbox productivity tutorial for developers",
		"copy": copy, "post_title": title, "post_id": postID,
	})
	if prepared.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %s", prepared.Code, prepared.Body.String())
	}
	placementID := preparedBody["placement"].(map[string]any)["id"].(string)
	var storedTitle string
	var storedPost int64
	if err := db.Pool.QueryRow(context.Background(), `SELECT post_title, wp_post_id FROM placement_proposals WHERE id=$1`, placementID).Scan(&storedTitle, &storedPost); err != nil || storedTitle != title || storedPost != postID {
		t.Fatalf("stored title %q post %d (%v)", storedTitle, storedPost, err)
	}
	approved, _ := adminJSON(t, publishers, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, _ := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", committed.Code, committed.Body.String())
	}
	check := func(path string) {
		t.Helper()
		rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		item := findDigestItem(itemsFromDigest(t, body), placementID)
		if item == nil || item["post_title"] != title || item["message"] != copy || item["post_title"] == copy {
			t.Fatalf("%s placement: %v", path, item)
		}
		offer := findDigestItem(itemsFromDigest(t, body), offerID)
		if offer == nil || offer["post_title"] != title {
			t.Fatalf("%s offer: %v", path, offer)
		}
	}
	check("/v1/digest")
	check("/v1/account/digest")

	receipt, receiptBody := doSiteJSON(t, publishers, http.MethodPost, "/v1/placements/"+placementID+"/receipt", siteKey, map[string]any{
		"surface_url": surface, "post_title": title, "post_id": postID,
		"published_text": copy + "\n\nAffiliate disclosure: I may earn a commission if you buy through this link.\n/go/" + placementID,
		"evidence":       "The plugin published this article.",
	})
	if receipt.Code != http.StatusOK {
		t.Fatalf("receipt: %d %v", receipt.Code, receiptBody)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT post_title, wp_post_id FROM placement_proposals WHERE id=$1`, placementID).Scan(&storedTitle, &storedPost); err != nil || storedTitle != title || storedPost != postID {
		t.Fatalf("receipt stored title %q post %d (%v)", storedTitle, storedPost, err)
	}
	check("/v1/digest")
	check("/v1/account/digest")
}

func TestMakerDigestReadsOwnOffersOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, api, r := newEntryTest(t, db)
	api.ClaimsCheck = config.ClaimsStub
	_, listedID := seedListedOffer(t, db, "maker-digest-key", "maker-digest@example.com", "Live Product")
	var heldID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id,status,commission_type,commission_value,landing_url)
SELECT product_id,'submitted',commission_type,commission_value,landing_url FROM offers WHERE id=$1 RETURNING id`, listedID).Scan(&heldID); err != nil {
		t.Fatal(err)
	}
	_, otherID := seedListedOffer(t, db, "other-maker-digest-key", "other-maker-digest@example.com", "Other Product")
	own, ownBody := postJSON(t, r, http.MethodGet, "/v1/maker/digest", nil, map[string]string{"X-Maker-Key": "maker-digest-key"})
	if own.Code != http.StatusOK {
		t.Fatalf("own digest: %d %s", own.Code, own.Body.String())
	}
	items := itemsFromDigest(t, ownBody)
	live := findDigestItem(items, listedID)
	held := findDigestItem(items, heldID)
	if live == nil || live["status"] != "listed" || findDigestItem(items, otherID) != nil {
		t.Fatalf("own live: %v", items)
	}
	if held == nil || held["status"] != "submitted" || held["review_reason"] != "claims" {
		t.Fatalf("own held: %v", held)
	}
	other, otherBody := postJSON(t, r, http.MethodGet, "/v1/maker/digest?offer_id="+listedID, nil, map[string]string{"X-Maker-Key": "other-maker-digest-key"})
	if other.Code != http.StatusForbidden || otherBody["message"] != "This key can read only its own offers." {
		t.Fatalf("other maker: %d %s", other.Code, other.Body.String())
	}
	otherList, otherListBody := postJSON(t, r, http.MethodGet, "/v1/maker/digest", nil, map[string]string{"X-Maker-Key": "other-maker-digest-key"})
	if otherList.Code != http.StatusOK || findDigestItem(itemsFromDigest(t, otherListBody), listedID) != nil || findDigestItem(itemsFromDigest(t, otherListBody), heldID) != nil {
		t.Fatalf("other list: %d %s", otherList.Code, otherList.Body.String())
	}
	ownerRoute, ownerRouteBody := postJSON(t, r, http.MethodGet, "/v1/digest", nil, map[string]string{"X-Maker-Key": "maker-digest-key"})
	if ownerRoute.Code != http.StatusForbidden || ownerRouteBody["reason"] != "forbidden_key_type" || strings.Contains(ownerRoute.Body.String(), "invalid return path") {
		t.Fatalf("owner digest with maker key: %d %s", ownerRoute.Code, ownerRoute.Body.String())
	}
	_ = h
}

func TestPluginRefuseReopensPrepare(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-plugin-refuse"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	surface := "https://publisher.example.com/refused-locally"
	copy := "Consider InboxPilot for an email inbox productivity workflow."
	prepare := func() (int, map[string]any) {
		t.Helper()
		rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]any{
			"offer_id": offerID, "surface_url": surface, "context": "InboxPilot email inbox productivity tutorial for developers", "copy": copy,
		})
		return rec.Code, body
	}
	code, prepared := prepare()
	if code != http.StatusCreated {
		t.Fatalf("prepare: %d %v", code, prepared)
	}
	placementID := prepared["placement"].(map[string]any)["id"].(string)
	approved, _ := adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, _ := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", committed.Code, committed.Body.String())
	}
	refuse := func() (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		return doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/refuse", siteKey, map[string]any{"reason": "The plugin did not insert this box."})
	}
	first, firstBody := refuse()
	if first.Code != http.StatusOK || firstBody["status"] != "pending" || firstBody["publication_status"] != "refused" {
		t.Fatalf("refuse: %d %v", first.Code, firstBody)
	}
	again, againBody := refuse()
	if again.Code != http.StatusOK || againBody["publication_status"] != "refused" {
		t.Fatalf("second refuse: %d %v", again.Code, againBody)
	}
	var status, publication string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, COALESCE(receipt->>'publication_status','') FROM placement_proposals WHERE id=$1`, placementID).Scan(&status, &publication); err != nil || status != "pending" || publication != "refused" {
		t.Fatalf("after refuse status=%s publication=%s err=%v", status, publication, err)
	}
	nextCode, nextBody := prepare()
	if nextCode != http.StatusCreated || nextBody["reason"] == "one_per_post" {
		t.Fatalf("prepare after refuse: %d %v", nextCode, nextBody)
	}
	nextID := nextBody["placement"].(map[string]any)["id"].(string)
	if nextID == "" || nextID == placementID {
		t.Fatalf("expected a new box, got %v", nextBody)
	}
	boxes, err := db.CountPlacementBoxes(context.Background(), siteID, surface, "00000000-0000-0000-0000-000000000000", 0)
	if err != nil || boxes != 1 {
		t.Fatalf("boxes=%d %v", boxes, err)
	}
}
