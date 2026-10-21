package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
)

func TestVerifiedSiteWithoutRowAsksAndSavedAutoPublishes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, live := newStepModeRouter(db, entry.LiveClaimChecker{})
	ownerID, siteID, key := signupPublisher(t, live, "d093b-missing@example.com", "d093b-missing.example")
	verifySite(t, db, siteID, "d093b-missing.example")
	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM agent_step_modes WHERE site_id=$1 OR owner_id=$2`, siteID, ownerID); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 OR owner_id=$2`, siteID, ownerID) != 0 {
		t.Fatal("step row remained")
	}
	_, offerID := seedListedOffer(t, db, "d093b-missing-maker", "d093b-missing-maker@example.com", "CodeLens")

	server := httptest.NewServer(live)
	defer server.Close()
	gin.SetMode(gin.TestMode)
	mcpRouter := gin.New()
	mcpRouter.POST("/v1/mcp", (&MCPHandler{LoopbackURL: server.URL}).Receive)
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "get_step_modes", "arguments": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(string(raw)))
	req.Header.Set("X-API-Key", key)
	mcpRec := httptest.NewRecorder()
	mcpRouter.ServeHTTP(mcpRec, req)
	var mcpBody map[string]any
	if err = json.Unmarshal(mcpRec.Body.Bytes(), &mcpBody); err != nil || mcpRec.Code != http.StatusOK {
		t.Fatalf("get_step_modes: %d %s", mcpRec.Code, mcpRec.Body.String())
	}
	result, _ := mcpBody["result"].(map[string]any)
	content, _ := result["content"].([]any)
	var text string
	if len(content) > 0 {
		text, _ = content[0].(map[string]any)["text"].(string)
	}
	if result["isError"] != false || !strings.Contains(text, `"placement_publish": "ask"`) || !strings.Contains(text, `"offer_match": "auto"`) || !strings.Contains(text, `"payout": "auto"`) || !strings.Contains(text, `"site_signup": "auto"`) {
		t.Fatalf("get_step_modes: %d %s", mcpRec.Code, mcpRec.Body.String())
	}

	held, heldBody := preparePlacement(t, live, key, offerID, "https://d093b-missing.example/posts/held", 1)
	if held.Code != http.StatusCreated || heldBody["status"] != "awaiting_approval" || heldBody["hold_reason"] != "step_ask" || heldBody["error"] != nil || placementBody(t, heldBody)["status"] != "pending" {
		t.Fatalf("missing row published: %d %s", held.Code, held.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 OR owner_id=$2`, siteID, ownerID) != 0 {
		t.Fatal("holding a placement wrote a step row")
	}

	saved := putStepModes(t, live, key, map[string]any{"placement_publish": "auto"})
	savedModes := modeMap(t, saved)
	if savedModes["placement_publish"] != "auto" || savedModes["offer_match"] != "auto" || saved["placement_publish_effective"] != "auto" {
		t.Fatalf("agent auto: %s", saved)
	}
	published, publishedBody := preparePlacement(t, live, key, offerID, "https://d093b-missing.example/posts/auto", 2)
	publishedPlacement := placementBody(t, publishedBody)
	if published.Code != http.StatusCreated || publishedBody["status"] != "published" || publishedPlacement["status"] != "committed" || publishedPlacement["approved_by"] != "policy:auto" {
		t.Fatalf("agent auto publish: %d %s", published.Code, published.Body.String())
	}
}

func TestPersonAndOwnerWideAutoStillPublish(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, live := newStepModeRouter(db, entry.LiveClaimChecker{})
	ctx := context.Background()

	owner, cookie := testOwnerSession(t, h, "d093b-person", "d093b-person@example.com")
	siteID := createTestSite(t, db, "d093b-person-key", "d093b-person.example", "Person Blog")
	if _, err := db.Pool.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM agent_step_modes WHERE site_id=$1 OR owner_id=$2`, siteID, owner.ID); err != nil {
		t.Fatal(err)
	}
	verifySite(t, db, siteID, "d093b-person.example")
	_, offerID := seedListedOffer(t, db, "d093b-person-maker", "d093b-person-maker@example.com", "CodeLens")
	putOwnerModes(t, live, h, cookie, owner.CSRF, map[string]any{"placement_publish": "auto"})
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='auto'`, siteID) != 1 {
		t.Fatal("person auto was not stored")
	}
	published, publishedBody := preparePlacement(t, live, "d093b-person-key", offerID, "https://d093b-person.example/posts/auto", 1)
	publishedPlacement := placementBody(t, publishedBody)
	if published.Code != http.StatusCreated || publishedBody["status"] != "published" || publishedPlacement["status"] != "committed" || publishedPlacement["approved_by"] != "policy:auto" {
		t.Fatalf("person auto publish: %d %s", published.Code, published.Body.String())
	}

	wideOwner, wideSite, wideKey := signupPublisher(t, live, "d093b-wide@example.com", "d093b-wide.example")
	verifySite(t, db, wideSite, "d093b-wide.example")
	if _, err := db.Pool.Exec(ctx, `DELETE FROM agent_step_modes WHERE site_id=$1 OR owner_id=$2`, wideSite, wideOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, owner_id, placement_publish) VALUES('owner:' || $1::text, $1::uuid, 'auto')`, wideOwner); err != nil {
		t.Fatal(err)
	}
	got, gotBody := postJSON(t, live, http.MethodGet, "/v1/step-modes", nil, map[string]string{"X-API-Key": wideKey})
	wideModes := modeMap(t, gotBody)
	if got.Code != http.StatusOK || wideModes["placement_publish"] != "auto" || gotBody["placement_publish_effective"] != "auto" {
		t.Fatalf("owner-wide auto read: %d %s", got.Code, got.Body.String())
	}
	widePublished, wideBody := preparePlacement(t, live, wideKey, offerID, "https://d093b-wide.example/posts/auto", 2)
	widePlacement := placementBody(t, wideBody)
	if widePublished.Code != http.StatusCreated || wideBody["status"] != "published" || widePlacement["status"] != "committed" || widePlacement["approved_by"] != "policy:auto" {
		t.Fatalf("owner-wide auto publish: %d %s", widePublished.Code, widePublished.Body.String())
	}
}

func TestPlacementPublishAskBackfill(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join("..", "migrations", "047_d093b_placement_publish_ask.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	if strings.Contains(sqlText, "BEGIN") || strings.Contains(sqlText, "COMMIT") || strings.Contains(strings.ToUpper(sqlText), "UPDATE") {
		t.Fatal("backfill rewrites rows or owns a transaction")
	}
	ctx := context.Background()
	insertSite := func(domain, hash string) string {
		t.Helper()
		var id string
		if err := db.Pool.QueryRow(ctx, `INSERT INTO sites(api_key_hash, api_key_prefix, domain, name) VALUES($1,$2,$3,$3) RETURNING id::text`, hash, hash[:6], domain).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	missing := insertSite("d093b-backfill-missing.example", "d093bmisshash")
	explicit := insertSite("d093b-backfill-auto.example", "d093bautohash")
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, site_id, site_signup, offer_match, placement_publish, link_change, payout) VALUES('site:' || $1::text, $1::uuid, 'auto', 'off', 'auto', 'ask', 'off')`, explicit); err != nil {
		t.Fatal(err)
	}
	asked := insertSite("d093b-backfill-ask.example", "d093baskhash0")
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, site_id, placement_publish) VALUES('site:' || $1::text, $1::uuid, 'ask')`, asked); err != nil {
		t.Fatal(err)
	}
	off := insertSite("d093b-backfill-off.example", "d093boffhash0")
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, site_id, placement_publish) VALUES('site:' || $1::text, $1::uuid, 'off')`, off); err != nil {
		t.Fatal(err)
	}
	wideSite := insertSite("d093b-backfill-wide.example", "d093bwidehash")
	var wideOwner string
	if err = db.Pool.QueryRow(ctx, `INSERT INTO owners(issuer, subject, email, name, site_id) VALUES('d093b','wide-backfill','d093b-wide-backfill@example.com','Wide',$1::uuid) RETURNING id::text`, wideSite).Scan(&wideOwner); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, owner_id, placement_publish) VALUES('owner:' || $1::text, $1::uuid, 'auto')`, wideOwner); err != nil {
		t.Fatal(err)
	}

	apply := func() {
		t.Helper()
		if _, err := db.Pool.Exec(ctx, sqlText); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	apply()

	var signup, match, publish, link, payout, ownerID, scope string
	if err = db.Pool.QueryRow(ctx, `SELECT site_signup, offer_match, placement_publish, link_change, payout, COALESCE(owner_id::text,''), scope_key FROM agent_step_modes WHERE site_id=$1`, missing).Scan(&signup, &match, &publish, &link, &payout, &ownerID, &scope); err != nil {
		t.Fatal(err)
	}
	if signup != "auto" || match != "auto" || publish != "ask" || link != "auto" || payout != "auto" || ownerID != "" || scope != "site:"+missing {
		t.Fatalf("backfill row %s %s %s %s %s owner=%s scope=%s", signup, match, publish, link, payout, ownerID, scope)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1`, missing) != 1 {
		t.Fatal("backfill duplicated the missing site")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND offer_match='off' AND placement_publish='auto' AND link_change='ask' AND payout='off'`, explicit) != 1 {
		t.Fatal("backfill changed a stored auto row")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='ask'`, asked) != 1 {
		t.Fatal("backfill changed a stored ask row")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='off'`, off) != 1 {
		t.Fatal("backfill changed a stored off row")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1`, wideSite) != 0 {
		t.Fatal("backfill added a site row over an owner-wide auto")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1 AND site_id IS NULL AND placement_publish='auto'`, wideOwner) != 1 {
		t.Fatal("backfill changed the owner-wide auto row")
	}
	loaded, err := db.LoadStepModes(ctx, wideOwner, wideSite)
	if err != nil || loaded.PlacementPublish != "auto" || loaded.OfferMatch != "auto" {
		t.Fatalf("owner-wide read after backfill: %+v %v", loaded, err)
	}
	bare, err := db.LoadStepModes(ctx, "", missing)
	if err != nil || bare.PlacementPublish != "ask" || bare.SiteSignup != "auto" || bare.Payout != "auto" {
		t.Fatalf("backfilled read: %+v %v", bare, err)
	}
}
