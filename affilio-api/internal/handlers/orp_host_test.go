package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

func TestORPDiscoveryAndDCP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entry := &AgentEntry{BaseURL: "http://127.0.0.1:8080", ClaimsCheck: "stub"}
	host := &ORPHost{Entry: entry}
	r := gin.New()
	r.Use(middleware.CORS("https://allowed.example"))
	r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}) })
	host.Mount(r)
	r.GET("/.well-known/dcp.json", ServeDCPDiscovery(""))

	req := httptest.NewRequest(http.MethodGet, "/.well-known/orp.json", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("orp.json %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("ACAO %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache %q", rec.Header().Get("Cache-Control"))
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "In testing" {
		t.Fatalf("name %v", doc["name"])
	}
	if doc["docs"] != "http://127.0.0.1:8080"+orp.DocsPath {
		t.Fatalf("docs %v", doc["docs"])
	}
	desc, _ := doc["description"].(string)
	if !strings.Contains(desc, orp.ProofPageTemplate) || !strings.Contains(desc, orp.DocsPath) {
		t.Fatalf("description %s", desc)
	}
	if _, ok := doc["conformance"]; ok {
		t.Fatalf("conformance claimed: %s", rec.Body.String())
	}
	checks := doc["capabilities"].(map[string]any)["checks"].(map[string]any)
	if checks["claims"] != "stub" {
		t.Fatalf("claims %v", checks["claims"])
	}
	legacy := doc["legacy"].(map[string]any)["dcp"].(map[string]any)
	if legacy["version"] != "0.1" || legacy["discovery"] != "/.well-known/dcp.json" {
		t.Fatalf("legacy %v", legacy)
	}
	if err := orp.ValidateAgainst(filepath.Join(orpRepoRoot(t), "docs", "protocol", "schema", "0.4", "discovery.schema.json"), doc); err != nil {
		t.Fatal(err)
	}

	opt := httptest.NewRequest(http.MethodOptions, "/.well-known/orp.json", nil)
	optRec := httptest.NewRecorder()
	r.ServeHTTP(optRec, opt)
	if optRec.Code != http.StatusNoContent {
		t.Fatalf("options %d", optRec.Code)
	}
	if optRec.Header().Get("Access-Control-Allow-Origin") != "*" || optRec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("options headers %v", optRec.Header())
	}

	dcpReq := httptest.NewRequest(http.MethodGet, "/.well-known/dcp.json", nil)
	dcpReq.Header.Set("Origin", "https://evil.example")
	dcpRec := httptest.NewRecorder()
	r.ServeHTTP(dcpRec, dcpReq)
	if dcpRec.Code != http.StatusOK {
		t.Fatalf("dcp.json %d %s", dcpRec.Code, dcpRec.Body.String())
	}
	var dcp map[string]any
	if err := json.Unmarshal(dcpRec.Body.Bytes(), &dcp); err != nil {
		t.Fatal(err)
	}
	if dcp["dcp_version"] != "0.1" || dcp["http_base"] != "/v1" {
		t.Fatalf("dcp body %s", dcpRec.Body.String())
	}
	if got := dcpRec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("dcp.json allowed every origin")
	}
	if landingMiss("/orp") || landingMiss("/orp/suggest") {
		t.Fatal("orp paths would render the landing 404")
	}
}

func TestORPSuggestJoinAndEnvelope(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	programID := insertProgram(t, db, "Zqnotionix Notes", "Zqnotionix", "zqnotionix-notes")
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "stub",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry, SuggestLimiter: NewIPRateLimiter(30, time.Minute)}
	r := mountORP(entry, host)

	before := dealCounts(t, db)
	owners := countRows(t, db, `SELECT COUNT(*) FROM owners`)
	rec, body := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{
		"text": "zqnotionix zqnotionix",
		"join": map[string]any{},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("suggest %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("suggest ACAO %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if err := orp.ValidateAgainst(schemaPath(t, "http/suggest-response.schema.json"), body); err != nil {
		t.Fatalf("suggest schema: %v\n%s", err, rec.Body.String())
	}
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("no pitches: %s", rec.Body.String())
	}
	first := items[0].(map[string]any)
	if first["pitch"] == "" {
		t.Fatalf("pitch missing: %s", rec.Body.String())
	}
	offer := first["offer"].(map[string]any)
	if offer["id"] != programID {
		t.Fatalf("offer id %v want %s", offer["id"], programID)
	}
	terms := offer["terms"].(map[string]any)
	if offer["terms_hash"] != orp.TermsHash(terms) {
		t.Fatalf("terms_hash %v", offer["terms_hash"])
	}
	joined := body["joined"].(map[string]any)
	token, _ := joined["token"].(string)
	if !strings.HasPrefix(token, "orat_") || len(token) < 20 {
		t.Fatalf("token %q", token)
	}
	ownerID, _ := joined["party"].(string)
	wantParty := "orp:127.0.0.1:8080/"
	if !strings.HasPrefix(ownerID, wantParty) {
		t.Fatalf("party %s", ownerID)
	}
	claim, _ := joined["claim"].(string)
	if !strings.HasSuffix(claim, "/owner") {
		t.Fatalf("claim %s", claim)
	}
	if joined["steps"].(map[string]any)["revoke"] != "auto" {
		t.Fatalf("steps %v", joined["steps"])
	}
	if !strings.HasSuffix(body["join"].(string), "/orp/join") {
		t.Fatalf("join url %v", body["join"])
	}
	listed, _ := postJSON(t, r, http.MethodGet, "/v1/agent-tokens", nil, bearerHeader(token))
	if listed.Code != http.StatusOK {
		t.Fatalf("agent-tokens %d %s", listed.Code, listed.Body.String())
	}

	badRec, badBody := postJSON(t, r, http.MethodPost, "/orp", envelope68(t), nil)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("#68 %d %s", badRec.Code, badRec.Body.String())
	}
	if badBody["type"] != "error" || badBody["body"].(map[string]any)["code"] != "invalid_message" {
		t.Fatalf("#68 body %s", badRec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/error.schema.json"), badBody); err != nil {
		t.Fatal(err)
	}
	if again := dealCounts(t, db); again != before {
		t.Fatalf("deals changed after invalid envelope: before %s after %s", before, again)
	}

	emailRec, _ := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"email": "a@b.c"}, nil)
	_, emailCodes := problemCodes(t, emailRec)
	if !sameCodes(emailCodes, []string{"join_personal"}) || !strings.Contains(emailRec.Body.String(), `"/email"`) {
		t.Fatalf("email join %d %s", emailRec.Code, emailRec.Body.String())
	}
	nameRec, _ := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"name": "Ada"}, nil)
	if nameRec.Code != http.StatusBadRequest || !strings.Contains(nameRec.Body.String(), "join_personal") || !strings.Contains(nameRec.Body.String(), `"/name"`) {
		t.Fatalf("name join %d %s", nameRec.Code, nameRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != owners+1 {
		t.Fatalf("owners changed on refused join")
	}

	acceptRec, acceptBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp":  "0.3",
		"type": "accept",
		"id":   "m_test_accept1",
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"from": ownerID,
		"body": map[string]any{},
	}, bearerHeader(token))
	if acceptRec.Code != http.StatusBadRequest || acceptBody["body"].(map[string]any)["code"] != "invalid_message" {
		t.Fatalf("accept %d %s", acceptRec.Code, acceptRec.Body.String())
	}
	if again := dealCounts(t, db); again != before {
		t.Fatalf("deals changed after accept: before %s after %s", before, again)
	}

	helloRec, helloBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp":  "0.3",
		"type": "hello",
		"id":   "m_test_hello01",
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"from": ownerID,
		"body": map[string]any{"versions": []any{"0.3"}},
	}, bearerHeader(token))
	if helloRec.Code != http.StatusOK || helloBody["type"] != "hello" {
		t.Fatalf("hello %d %s", helloRec.Code, helloRec.Body.String())
	}
	if helloBody["body"].(map[string]any)["party"] != ownerID {
		t.Fatalf("hello party %v", helloBody["body"])
	}
	versions := helloBody["body"].(map[string]any)["versions"].([]any)
	if len(versions) != 1 || versions[0] != "0.3" {
		t.Fatalf("versions %v", versions)
	}
	if err := orp.ValidateAgainst(schemaPath(t, "message.schema.json"), helloBody); err != nil {
		t.Fatal(err)
	}

	staleRec, staleBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp":  "0.3",
		"type": "hello",
		"id":   "m_test_stale01",
		"ts":   time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		"from": ownerID,
		"body": map[string]any{"versions": []any{"0.3"}},
	}, bearerHeader(token))
	if staleRec.Code != http.StatusBadRequest || staleBody["body"].(map[string]any)["code"] != "stale" {
		t.Fatalf("stale %d %s", staleRec.Code, staleRec.Body.String())
	}
}

func TestORPSuggestRateLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey, ClaimsCheck: "stub"}
	host := &ORPHost{Entry: entry, SuggestLimiter: NewIPRateLimiter(1, time.Minute)}
	r := mountORP(entry, host)
	payload := map[string]any{"text": "zqnotionix zqnotionix"}
	first, _ := postJSON(t, r, http.MethodPost, "/orp/suggest", payload, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first %d %s", first.Code, first.Body.String())
	}
	second, body := postJSON(t, r, http.MethodPost, "/orp/suggest", payload, nil)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second %d %s", second.Code, second.Body.String())
	}
	if second.Header().Get("Retry-After") != "60" || body["body"].(map[string]any)["code"] != "rate_limited" {
		t.Fatalf("rate limit %s", second.Body.String())
	}
}

func mountORP(entry *AgentEntry, host *ORPHost) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS("https://allowed.example"))
	r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}) })
	entry.Mount(r)
	host.Mount(r)
	(&A2A{Entry: entry, Engine: r}).Mount(r)
	r.GET("/.well-known/dcp.json", ServeDCPDiscovery(""))
	return r
}

func dealCounts(t *testing.T, db *database.DB) string {
	t.Helper()
	parts := make([]string, 0, 4)
	for _, table := range []string{"offers", "campaigns", "placement_proposals", "conversions"} {
		parts = append(parts, table+"="+strconv.Itoa(countRows(t, db, "SELECT COUNT(*) FROM "+table)))
	}
	return strings.Join(parts, ",")
}

func envelope68(t *testing.T) any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(orpRepoRoot(t), "docs", "protocol", "schema", "examples", "invalid", "envelope-68-shape.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wrapped struct {
		Instance any `json:"instance"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatal(err)
	}
	return wrapped.Instance
}

func schemaPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(orpRepoRoot(t), "docs", "protocol", "schema", "0.3", name)
}

func orpRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "protocol", "schema", "0.3", "defs.schema.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
