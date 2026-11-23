package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/dcp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func TestD097SuggestMonetizationAIDraft(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry, r, srv := mountMonetization(t, db)
	token, ownerID := mintJoin(t, r)
	draft := "I have been testing an AI writing tool that drafts posts for founders. Reach ada@secret.example or @ada_private at +1 415 555 0199."
	rec, body := suggestDraft(t, r, token, map[string]any{"text": draft, "surface": "x", "language": "en"})
	if rec.Code != http.StatusOK {
		t.Fatalf("suggest %d %s", rec.Code, rec.Body.String())
	}
	offers := suggestOffers(t, body)
	if len(offers) < 1 || len(offers) > 3 {
		t.Fatalf("offers %d %s", len(offers), rec.Body.String())
	}
	if body["human_prompt"] != monetizationHumanPrompt || body["needs_human_confirm"] != true || body["mode"] != "ask" || body["step"] != "offer_match" || body["reason"] != "" {
		t.Fatalf("prompt body %s", rec.Body.String())
	}
	first := offers[0]
	if first["offer_id"] == "" || first["title"] == "" || first["why_match"] == "" || first["disclosure_line"] != monetizationDisclosure || first["needs_human_confirm"] != true {
		t.Fatalf("offer %#v", first)
	}
	link, _ := first["tracked_link"].(string)
	if !strings.Contains(link, "/v1/m/sg") || !strings.HasPrefix(link, srv.URL) {
		t.Fatalf("tracked link %q", link)
	}
	for _, secret := range []string{"ada@secret.example", "secret.example", "ada_private", "5550199", "+1 415 555 0199"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response kept %q", secret)
		}
	}
	assertNoDraftPII(t, db, draft)
	if countRows(t, db, `SELECT COUNT(*) FROM placement_proposals`) != 0 || countRows(t, db, `SELECT COUNT(*) FROM campaigns`) != 0 {
		t.Fatal("suggest published a placement or campaign")
	}
	path := link[strings.LastIndex(link, "/v1/m/"):]
	redir := httptest.NewRecorder()
	r.ServeHTTP(redir, httptest.NewRequest(http.MethodGet, path, nil))
	if redir.Code != http.StatusFound || !strings.HasPrefix(redir.Header().Get("Location"), "https://") {
		t.Fatalf("redirect %d %s", redir.Code, redir.Header().Get("Location"))
	}
	code := path[strings.LastIndex(path, "/")+1:]
	clicks, err := db.MonetizationLinkClicks(context.Background(), code)
	if err != nil || clicks != 1 {
		t.Fatalf("clicks %d %v", clicks, err)
	}
	var surface, lang string
	if err = db.Pool.QueryRow(context.Background(), `SELECT surface, lang FROM monetization_suggestions WHERE owner_id=$1`, ownerID).Scan(&surface, &lang); err != nil || surface != "x" || lang != "en" {
		t.Fatalf("stored surface %q lang %q %v", surface, lang, err)
	}

	client := dcp.NewClient(srv.URL, "")
	client.Bearer = token
	out, err := dcp.CallTool(client, "suggest_monetization", map[string]interface{}{"text": draft, "surface": "x", "language": "en"})
	if err != nil {
		t.Fatal(err)
	}
	toolOffers, _ := out["offers"].([]any)
	if len(toolOffers) < 1 || out["human_prompt"] != monetizationHumanPrompt {
		t.Fatalf("tool %#v", out)
	}
	if entry == nil {
		t.Fatal("entry")
	}
}

func TestD097SuggestMonetizationCounts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := mountMonetization(t, db)
	token, _ := mintJoin(t, r)

	none, noneBody := suggestDraft(t, r, token, map[string]any{"text": "xqplumcake xqplumcake"})
	if none.Code != http.StatusOK || len(suggestOffers(t, noneBody)) != 0 || noneBody["reason"] != "no_match" || noneBody["human_prompt"] != "" {
		t.Fatalf("zero %d %s", none.Code, none.Body.String())
	}

	insertSuggestProgram(t, db, "Only Zqoneonly", "zqoneonly", "gadgets", []string{"zqoneonly"})
	one, oneBody := suggestDraft(t, r, token, map[string]any{"text": "zqoneonly zqoneonly", "surface": "blog"})
	oneOffers := suggestOffers(t, oneBody)
	if one.Code != http.StatusOK || len(oneOffers) != 1 || oneOffers[0]["title"] != "Only Zqoneonly" || oneBody["human_prompt"] != monetizationHumanPrompt {
		t.Fatalf("one %d %s", one.Code, one.Body.String())
	}

	for _, name := range []string{"Alpha Zqthreeonly", "Bravo Zqthreeonly", "Charlie Zqthreeonly", "Delta Zqthreeonly", "Echo Zqthreeonly"} {
		insertSuggestProgram(t, db, name, strings.ToLower(strings.ReplaceAll(name, " ", "-")), "gadgets", []string{"zqthreeonly"})
	}
	three, threeBody := suggestDraft(t, r, token, map[string]any{"draft": "zqthreeonly zqthreeonly", "platform": "x", "lang": "en"})
	threeOffers := suggestOffers(t, threeBody)
	if three.Code != http.StatusOK || len(threeOffers) != 3 {
		t.Fatalf("three %d %s", three.Code, three.Body.String())
	}
	if threeOffers[0]["title"] != "Alpha Zqthreeonly" || threeOffers[1]["title"] != "Bravo Zqthreeonly" || threeOffers[2]["title"] != "Charlie Zqthreeonly" {
		t.Fatalf("order %#v", threeOffers)
	}
}

func TestD097SuggestMonetizationModes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := mountMonetization(t, db)
	token, ownerID := mintJoin(t, r)
	insertSuggestProgram(t, db, "Mode Zqmodeoff", "zqmodeoff", "gadgets", []string{"zqmodeoff"})
	ctx := context.Background()
	draft := map[string]any{"text": "zqmodeoff zqmodeoff", "surface": "newsletter"}

	asked, askedBody := suggestDraft(t, r, token, draft)
	if asked.Code != http.StatusOK || len(suggestOffers(t, askedBody)) != 1 || askedBody["needs_human_confirm"] != true || askedBody["mode"] != "ask" {
		t.Fatalf("ask %d %s", asked.Code, asked.Body.String())
	}

	if err := db.SaveStepModes(ctx, ownerID, "", database.StepModes{PlacementPublish: database.ModeOff}); err != nil {
		t.Fatal(err)
	}
	off, offBody := suggestDraft(t, r, token, draft)
	if off.Code != http.StatusOK || len(suggestOffers(t, offBody)) != 0 || offBody["reason"] != "step_off" || offBody["step"] != "placement_publish" || offBody["mode"] != "off" || offBody["human_prompt"] != "" {
		t.Fatalf("off %d %s", off.Code, off.Body.String())
	}

	if err := db.SaveStepModes(ctx, ownerID, "", database.StepModes{OfferMatch: database.ModeOff, PlacementPublish: database.ModeAsk}); err != nil {
		t.Fatal(err)
	}
	matchOff, matchBody := suggestDraft(t, r, token, draft)
	if matchOff.Code != http.StatusOK || len(suggestOffers(t, matchBody)) != 0 || matchBody["reason"] != "step_off" || matchBody["step"] != "offer_match" {
		t.Fatalf("match off %d %s", matchOff.Code, matchOff.Body.String())
	}

	if err := db.SaveStepModes(ctx, ownerID, "", database.StepModes{OfferMatch: database.ModeAuto, PlacementPublish: database.ModeAuto}); err != nil {
		t.Fatal(err)
	}
	auto, autoBody := suggestDraft(t, r, token, draft)
	autoOffers := suggestOffers(t, autoBody)
	if auto.Code != http.StatusOK || len(autoOffers) != 1 || autoBody["mode"] != "auto" || autoBody["needs_human_confirm"] != false || autoOffers[0]["needs_human_confirm"] != false {
		t.Fatalf("auto %d %s", auto.Code, auto.Body.String())
	}
	if autoBody["human_prompt"] != monetizationHumanPrompt {
		t.Fatalf("auto prompt %s", auto.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM placement_proposals`) != 0 {
		t.Fatal("auto published")
	}
}

func TestD097SuggestMonetizationCategoryDeny(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := mountMonetization(t, db)
	token, ownerID := mintJoin(t, r)
	insertSuggestProgram(t, db, "Deny Zqdenyme", "zqdenyme", "gadgets", []string{"zqdenyme"})
	siteID := createTestSite(t, db, "d097-site-key", "d097.example", "D097")
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1`, ownerID, siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO approval_policies(scope, site_id, mode, deny_categories) VALUES('site', $1, 'manual', ARRAY['gadgets'])`, siteID); err != nil {
		t.Fatal(err)
	}
	rec, body := suggestDraft(t, r, token, map[string]any{"text": "zqdenyme zqdenyme", "surface": "email"})
	if rec.Code != http.StatusOK || len(suggestOffers(t, body)) != 0 || body["reason"] != "filtered" {
		t.Fatalf("deny %d %s", rec.Code, rec.Body.String())
	}

	if _, err := db.Pool.Exec(ctx, `UPDATE approval_policies SET deny_categories='{}', allow_categories=ARRAY['writing'] WHERE site_id=$1`, siteID); err != nil {
		t.Fatal(err)
	}
	allowed, allowedBody := suggestDraft(t, r, token, map[string]any{"text": "zqdenyme zqdenyme"})
	if allowed.Code != http.StatusOK || len(suggestOffers(t, allowedBody)) != 0 || allowedBody["reason"] != "filtered" {
		t.Fatalf("allow %d %s", allowed.Code, allowed.Body.String())
	}
}

func TestD097SuggestMonetizationAuth(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := mountMonetization(t, db)
	rec, body := suggestDraft(t, r, "", map[string]any{"text": "AI writing tool"})
	if rec.Code != http.StatusUnauthorized || body["error"] != "invalid bearer token" {
		t.Fatalf("auth %d %s", rec.Code, rec.Body.String())
	}
	token, _ := mintJoin(t, r)
	bad, _ := suggestDraft(t, r, token, map[string]any{"text": "AI writing tool", "surface": "tiktok"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("surface %d %s", bad.Code, bad.Body.String())
	}
}

func mountMonetization(t *testing.T, db *database.DB) (*AgentEntry, *gin.Engine, *httptest.Server) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &AgentEntry{DB: db, AddressKey: testAddressKey}
	host := &ORPHost{Entry: entry}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	host.Mount(r)
	r.GET("/v1/m/:code", (&RedirectsHandler{DB: db}).Monetization)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	entry.BaseURL = srv.URL
	return entry, r, srv
}

func mintJoin(t *testing.T, r http.Handler) (string, string) {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, rec, body)
	ownerID, _ := body["owner_id"].(string)
	if ownerID == "" {
		t.Fatal("owner id missing")
	}
	return token, ownerID
}

func suggestDraft(t *testing.T, r http.Handler, token string, payload map[string]any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	headers := map[string]string{}
	if token != "" {
		headers = bearerHeader(token)
	}
	return postJSON(t, r, http.MethodPost, MonetizationSuggestPath, payload, headers)
}

func suggestOffers(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["offers"].([]any)
	if !ok {
		t.Fatalf("offers %#v", body["offers"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		one, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("offer %#v", item)
		}
		out = append(out, one)
	}
	return out
}

func insertSuggestProgram(t *testing.T, db *database.DB, name, slug, category string, keywords []string) string {
	t.Helper()
	var id string
	err := db.Pool.QueryRow(context.Background(), `
INSERT INTO affiliate_programs (name, company, slug, description, website_url, commission_type, commission_value, commission_unit, primary_category, keywords, status)
VALUES ($1,$2,$3,'',$4,'percent',30,'percent',$5,$6,'active')
RETURNING id::text`, name, name, slug, "https://example.com/"+slug, category, keywords).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertNoDraftPII(t *testing.T, db *database.DB, draft string) {
	t.Helper()
	ctx := context.Background()
	var blob, stored string
	if err := db.Pool.QueryRow(ctx, `
SELECT COALESCE(string_agg(draft_sha256 || ' ' || keywords::text || ' ' || surface || ' ' || lang || ' ' || offer_ids::text || ' ' || mode, ' '), ''),
       COALESCE(max(draft_sha256), '')
FROM monetization_suggestions`).Scan(&blob, &stored); err != nil {
		t.Fatal(err)
	}
	var links string
	if err := db.Pool.QueryRow(ctx, `SELECT COALESCE(string_agg(code || ' ' || offer_id || ' ' || destination, ' '), '') FROM monetization_suggestion_links`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"ada@secret.example", "secret.example", "ada_private", "5550199", draft} {
		if strings.Contains(blob, secret) || strings.Contains(links, secret) {
			t.Fatalf("stored %q", secret)
		}
	}
	stripped := sha256.Sum256([]byte(services.StripDraftPII(draft)))
	raw := sha256.Sum256([]byte(draft))
	if stored != hex.EncodeToString(stripped[:]) || stored == hex.EncodeToString(raw[:]) {
		t.Fatalf("hash %s", stored)
	}
}
