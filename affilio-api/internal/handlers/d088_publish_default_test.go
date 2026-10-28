package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/dcp"
	"affilio-api/internal/entry"

	"github.com/gin-gonic/gin"
)

func TestFreshSetupHoldsPlacementForApproval(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, live := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, live, "fresh-hold@example.com", "fresh-hold.example")
	verifySite(t, db, siteID, "fresh-hold.example")
	_, offerID := seedListedOffer(t, db, "fresh-hold-maker", "fresh-hold-maker@example.com", "CodeLens")

	got, body := postJSON(t, live, http.MethodGet, "/v1/step-modes", nil, map[string]string{"X-API-Key": key})
	modes := modeMap(t, body)
	if got.Code != http.StatusOK || modes["placement_publish"] != "ask" || modes["offer_match"] != "auto" {
		t.Fatalf("fresh modes: %d %s", got.Code, got.Body.String())
	}

	rec, placed := preparePlacement(t, live, key, offerID, "https://fresh-hold.example/posts/first", 1)
	if rec.Code != http.StatusCreated || placed["status"] != "awaiting_approval" || placed["hold_reason"] != "step_ask" || placed["error"] != nil {
		t.Fatalf("fresh placement: %d %s", rec.Code, rec.Body.String())
	}
	if placementBody(t, placed)["status"] != "pending" {
		t.Fatalf("fresh placement row: %s", rec.Body.String())
	}

	_, stub := newStepModeRouter(db, entry.StubClaimChecker{})
	held, heldBody := preparePlacement(t, stub, key, offerID, "https://fresh-hold.example/posts/stub", 2)
	if held.Code != http.StatusCreated || heldBody["status"] != "awaiting_approval" || heldBody["hold_reason"] != "step_ask" || heldBody["error"] != nil {
		t.Fatalf("stub claims still publishes or uses another hold: %d %s", held.Code, held.Body.String())
	}
}

func TestSiteWithNoStepRowStaysPendingWhenClaimsAreLive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, live := newStepModeRouter(db, entry.LiveClaimChecker{})
	_, siteID, key := signupPublisher(t, live, "legacy-ask@example.com", "legacy-ask.example")
	verifySite(t, db, siteID, "legacy-ask.example")
	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM agent_step_modes WHERE site_id=$1`, siteID); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "legacy-ask-maker", "legacy-ask-maker@example.com", "CodeLens")

	got, gotBody := postJSON(t, live, http.MethodGet, "/v1/step-modes", nil, map[string]string{"X-API-Key": key})
	modes := modeMap(t, gotBody)
	if got.Code != http.StatusOK || modes["placement_publish"] != "ask" || modes["site_signup"] != "auto" || modes["offer_match"] != "auto" || modes["link_change"] != "auto" || modes["payout"] != "auto" {
		t.Fatalf("missing row modes: %d %s", got.Code, got.Body.String())
	}
	if gotBody["placement_publish_effective"] != "ask" || gotBody["placement_publish_reason"] != "step_ask" {
		t.Fatalf("effective ask: %s", got.Body.String())
	}

	rec, body := preparePlacement(t, live, key, offerID, "https://legacy-ask.example/posts/kept", 1)
	placement := placementBody(t, body)
	if rec.Code != http.StatusCreated || body["status"] != "awaiting_approval" || body["hold_reason"] != "step_ask" || body["error"] != nil || placement["status"] != "pending" {
		t.Fatalf("legacy missing row: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterAgainDoesNotRewritePublishMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	id, err := db.RegisterInstallation(ctx, "d088-install-hash", "d088inst", "install-again.example", "Install")
	if err != nil || id == "" {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='ask'`, id) != 1 {
		t.Fatal("a new installation did not store ask")
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE agent_step_modes SET placement_publish='auto' WHERE site_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	again, err := db.RegisterInstallation(ctx, "d088-install-hash", "d088inst", "install-again.example", "Install")
	if err != nil || again != id {
		t.Fatalf("register again: %s %v", again, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='auto'`, id) != 1 {
		t.Fatal("registering an existing installation changed placement publish")
	}

	if _, err = db.Pool.Exec(ctx, `INSERT INTO sites(api_key_hash, api_key_prefix, domain, name) VALUES('d088-old-hash','d088old','old-site.example','Old')`); err != nil {
		t.Fatal(err)
	}
	oldID, err := db.RegisterInstallation(ctx, "d088-old-hash", "d088old", "old-site.example", "Old")
	if err != nil || oldID == "" {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1`, oldID) != 0 {
		t.Fatal("an existing site with no step row gained one")
	}
	modes, err := db.LoadStepModes(ctx, "", oldID)
	if err != nil || modes.PlacementPublish != "ask" || modes.OfferMatch != "auto" || modes.Payout != "auto" {
		t.Fatalf("old site read: %+v %v", modes, err)
	}
}

func TestNewSitePublishDefaultMigrationLeavesRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join("..", "migrations", "044_d088_new_site_publish_ask.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	if strings.Contains(sqlText, "BEGIN") || strings.Contains(sqlText, "COMMIT") || strings.Contains(strings.ToUpper(sqlText), "UPDATE ") {
		t.Fatal("migration is not an additive file that leaves rows alone")
	}
	ctx := context.Background()
	var siteID string
	if err = db.Pool.QueryRow(ctx, `INSERT INTO sites(api_key_hash, api_key_prefix, domain, name) VALUES('d088-mig-hash','d088mig','mig-default.example','Mig') RETURNING id::text`).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_step_modes(scope_key, site_id, placement_publish) VALUES($1, $2, 'auto')`, "site:"+siteID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, sqlText); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, sqlText); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE site_id=$1 AND placement_publish='auto'`, siteID) != 1 {
		t.Fatal("migration changed a stored auto row")
	}
	var columnDefault string
	if err = db.Pool.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_name='agent_step_modes' AND column_name='placement_publish'`).Scan(&columnDefault); err != nil || !strings.Contains(columnDefault, "ask") {
		t.Fatalf("column default %q %v", columnDefault, err)
	}
}

func TestLLMsAndDocsStateNewSitePublishDefault(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, path := range []string{
		filepath.Join(root, "affilio-wp", "admin", "build", "landing-v9", "llms-full.txt"),
		filepath.Join(root, "ui", "landing", "src", "publishers", "setup", "App.tsx"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), entry.NewSitePublishDefault) {
			t.Fatalf("%s does not state the new-site default", path)
		}
		if strings.Contains(path, "llms-full.txt") && !strings.Contains(string(body), entry.MissingRowPublishAsk) {
			t.Fatalf("%s does not state the missing-row publish default", path)
		}
		if strings.Contains(string(body), "still reads as auto") || strings.Contains(string(body), "still reads every step as auto") {
			t.Fatalf("%s still says a missing row reads as auto", path)
		}
	}
	var described bool
	for _, tool := range dcp.PublisherTools {
		if tool.Name == "get_step_modes" && strings.Contains(tool.Description, entry.NewSitePublishDefault) && strings.Contains(tool.Description, entry.MissingRowPublishAsk) && !strings.Contains(tool.Description, "still reads every step as auto") {
			described = true
		}
	}
	if !described {
		t.Fatal("get_step_modes does not state the new-site default")
	}
	modes, ok := agentDoc("modes.md")
	if !ok || !strings.Contains(modes, entry.NewSitePublishDefault) || !strings.Contains(modes, entry.MissingRowPublishAsk) {
		t.Fatal("modes.md does not state the publish default")
	}
	settings, ok := agentDoc("settings.md")
	if !ok || !strings.Contains(settings, entry.NewSitePublishDefault) || !strings.Contains(settings, entry.MissingRowPublishAsk) {
		t.Fatal("settings.md does not state the publish default")
	}
	if strings.Contains(modes, "still reads every step as") || strings.Contains(settings, "still reads that step as") {
		t.Fatal("agent docs still say a missing row reads as auto")
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{AssetsPath: filepath.Join(root, "affilio-wp", "admin", "build")}).Mount(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms-full.txt", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/plain") || !strings.Contains(rec.Body.String(), entry.NewSitePublishDefault) || !strings.Contains(rec.Body.String(), entry.MissingRowPublishAsk) {
		t.Fatalf("/llms-full.txt: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
}
