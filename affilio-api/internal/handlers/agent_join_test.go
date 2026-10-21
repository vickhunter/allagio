package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func mountJoinTokens(t *testing.T, db *database.DB, perMinute int) (*AgentEntry, *HostedHandler, *gin.Engine, *httptest.Server) {
	t.Helper()
	h, r, server := hostedTestServer(t, db)
	entry := &AgentEntry{DB: db, BaseURL: h.BaseURL}
	if perMinute > 0 {
		entry.TokenLimiter = NewIPRateLimiter(perMinute, time.Minute)
	}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	r.GET("/v1/agent-tokens", entry.ListAgentTokens)
	r.POST("/v1/agent-tokens/revoke", entry.RevokeAgentToken)
	r.POST("/v1/agent-tokens/:id/revoke", entry.RevokeAgentTokenByID)
	offers := &OffersHandler{DB: db}
	maker := r.Group("/v1/maker", middleware.MakerKeyAuth(db, h.resource()))
	maker.GET("/offers", offers.List)
	maker.POST("/offers", offers.Create)
	maker.POST("/offers/:id/submit", offers.Submit)
	return entry, h, r, server
}

func bearerHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func assertJoinIssued(t *testing.T, rec *httptest.ResponseRecorder, body map[string]any) string {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := body["role"]; ok {
		t.Fatalf("role was chosen: %s", rec.Body.String())
	}
	roles, ok := body["roles"].([]any)
	if !ok || len(roles) != 0 {
		t.Fatalf("roles: %#v", body["roles"])
	}
	if body["token_type"] != "Bearer" || body["scope"] != "agent" || body["agent_only"] != true || body["human_login"] != false {
		t.Fatalf("token body: %s", rec.Body.String())
	}
	secs, ok := body["expires_in"].(float64)
	if !ok || secs < 86300 || secs > 86400 {
		t.Fatalf("expires_in %v", body["expires_in"])
	}
	token, _ := body["access_token"].(string)
	if !strings.HasPrefix(token, "orat_") || len(token) < 20 {
		t.Fatalf("access_token %q", token)
	}
	if !strings.Contains(rec.Body.String(), `"roles":[]`) {
		t.Fatalf("roles encoded as %s", rec.Body.String())
	}
	return token
}

func TestAgentJoinTokenMintAndMCP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, h, r, server := mountJoinTokens(t, db, 8)

	empty, emptyBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", nil, nil)
	if empty.Code != http.StatusCreated || emptyBody["scope"] != "agent" {
		t.Fatalf("empty body: %d %s", empty.Code, empty.Body.String())
	}
	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, rec, body)
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE token_hash=$1`, services.HashAPIKey(token)) != 1 {
		t.Fatal("token hash was not stored")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE token_hash=$1 OR token_prefix=$1`, token) != 0 {
		t.Fatal("plaintext token was stored")
	}
	var email string
	var agentOnly, human bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT email, agent_only, human_login FROM owners WHERE id=$1`, body["owner_id"]).Scan(&email, &agentOnly, &human); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(email, "@agents.invalid") || !agentOnly || human {
		t.Fatalf("owner %s agent_only=%v human=%v", email, agentOnly, human)
	}
	var siteID, makerID *string
	if err := db.Pool.QueryRow(context.Background(), `SELECT site_id::text, maker_id::text FROM owners WHERE id=$1`, body["owner_id"]).Scan(&siteID, &makerID); err != nil {
		t.Fatal(err)
	}
	if siteID != nil || makerID != nil {
		t.Fatalf("role was assigned at signup site=%v maker=%v", siteID, makerID)
	}

	named, namedBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{"name": "Ada"}, nil)
	if named.Code != http.StatusCreated {
		t.Fatal(named.Body.String())
	}
	var storedName string
	if err := db.Pool.QueryRow(context.Background(), `SELECT name FROM owners WHERE id=$1`, namedBody["owner_id"]).Scan(&storedName); err != nil || storedName != "Ada" {
		t.Fatalf("name %q %v", storedName, err)
	}
	refused, refusedBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{"role": "publisher", "name": "Ada"}, nil)
	if refused.Code != http.StatusBadRequest || refusedBody["error"] != "role is not chosen here" {
		t.Fatalf("role: %d %s", refused.Code, refused.Body.String())
	}
	unknown, unknownBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{"source": "cli"}, nil)
	if unknown.Code != http.StatusBadRequest || unknownBody["error"] != "invalid payload" {
		t.Fatalf("unknown: %d %s", unknown.Code, unknown.Body.String())
	}
	longName := strings.Repeat("a", 81)
	tooLong, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{"name": longName}, nil)
	if tooLong.Code != http.StatusBadRequest {
		t.Fatalf("long name: %d %s", tooLong.Code, tooLong.Body.String())
	}
	newline, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{"name": "a\nb"}, nil)
	if newline.Code != http.StatusBadRequest {
		t.Fatalf("newline name: %d %s", newline.Code, newline.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	names := mcpToolNames(t, ctx, server.URL+"/mcp", token)
	for _, want := range []string{"preview_revenue", "prepare_placement", "list_offers"} {
		if !names[want] {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
	if names["register_maker"] {
		t.Fatal("register_maker was offered")
	}
	if mcpBearerStatus(r, token) != http.StatusOK {
		t.Fatal("mcp rejected the join token")
	}

	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if login.Code != http.StatusFound {
		t.Fatalf("google login: %d", login.Code)
	}
	asKey, _ := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{}, map[string]string{"X-API-Key": token})
	if asKey.Code != http.StatusUnauthorized {
		t.Fatalf("join token as site key: %d %s", asKey.Code, asKey.Body.String())
	}
	if h.resource() != h.BaseURL+"/mcp" {
		t.Fatal(h.resource())
	}
}

func TestAgentJoinInfersSidesAndRevokes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, h, r, server := mountJoinTokens(t, db, 8)
	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, rec, body)
	ownerID, _ := body["owner_id"].(string)
	auth := bearerHeader(token)

	early, earlyBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", map[string]any{"context": "a short note"}, auth)
	if early.Code != http.StatusForbidden || earlyBody["error"] != "publisher scope is not authorized for this action" {
		t.Fatalf("find before a placement: %d %s", early.Code, early.Body.String())
	}
	products := httptest.NewRecorder()
	productReq := httptest.NewRequest(http.MethodGet, "/v1/maker/products", nil)
	productReq.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(products, productReq)
	if products.Code != http.StatusForbidden {
		t.Fatalf("products before offers: %d %s", products.Code, products.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners WHERE id=$1 AND site_id IS NULL AND maker_id IS NULL`, ownerID) != 1 {
		t.Fatal("a read created a role")
	}

	listed := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/maker/offers", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(listed, listReq)
	if listed.Code != http.StatusOK {
		t.Fatalf("list offers: %d %s", listed.Code, listed.Body.String())
	}
	var makerEmail string
	if err := db.Pool.QueryRow(context.Background(), `SELECT m.email FROM makers m JOIN owners o ON o.maker_id=m.id WHERE o.id=$1`, ownerID).Scan(&makerEmail); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(makerEmail, "@agents.invalid") {
		t.Fatal(makerEmail)
	}

	placed, placedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{}, auth)
	if placed.Code != http.StatusBadRequest || placedBody["error"] != "offer_id, surface_url, context and copy required" {
		t.Fatalf("placement: %d %s", placed.Code, placed.Body.String())
	}
	var domain, publish string
	if err := db.Pool.QueryRow(context.Background(), `SELECT s.domain, m.placement_publish FROM sites s JOIN owners o ON o.site_id=s.id JOIN agent_step_modes m ON m.site_id=s.id WHERE o.id=$1`, ownerID).Scan(&domain, &publish); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(domain, ".invalid") || publish != "ask" {
		t.Fatalf("site %s publish %s", domain, publish)
	}
	var scope string
	if err := db.Pool.QueryRow(context.Background(), `SELECT scope FROM agent_join_tokens WHERE owner_id=$1`, ownerID).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"agent", "publisher", "maker"} {
		if !strings.Contains(" "+scope+" ", " "+word+" ") {
			t.Fatalf("scope %q missing %s", scope, word)
		}
	}
	listedTokens, listedBody := postJSON(t, r, http.MethodGet, "/v1/agent-tokens", nil, auth)
	if listedTokens.Code != http.StatusOK {
		t.Fatal(listedTokens.Body.String())
	}
	roles, _ := listedBody["roles"].([]any)
	if len(roles) != 2 || roles[0] != "publisher" || roles[1] != "maker" {
		t.Fatalf("roles %#v", listedBody["roles"])
	}
	rawList := listedTokens.Body.String()
	if strings.Contains(rawList, token) || strings.Contains(rawList, "token_hash") {
		t.Fatal("list returned a secret")
	}

	otherRec, otherBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	other := assertJoinIssued(t, otherRec, otherBody)
	otherID, _ := otherBody["token_id"].(string)
	ownID, _ := body["token_id"].(string)
	cross, crossBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens/"+otherID+"/revoke", nil, auth)
	if cross.Code != http.StatusNotFound || crossBody["error"] != "token not found" {
		t.Fatalf("cross revoke: %d %s", cross.Code, cross.Body.String())
	}
	badID, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens/not-a-uuid/revoke", nil, auth)
	if badID.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d %s", badID.Code, badID.Body.String())
	}
	own, ownBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens/"+ownID+"/revoke", nil, auth)
	if own.Code != http.StatusOK || ownBody["revoked"] != true {
		t.Fatalf("own revoke: %d %s", own.Code, own.Body.String())
	}
	if mcpBearerStatus(r, token) != http.StatusUnauthorized {
		t.Fatal("revoked token still opened mcp")
	}
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens/revoke", nil, auth)
	if again.Code != http.StatusOK || againBody["token_id"] != ownID {
		t.Fatalf("second revoke: %d %s", again.Code, again.Body.String())
	}
	unknownBearer, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens/revoke", nil, bearerHeader("orat_missing"))
	if unknownBearer.Code != http.StatusUnauthorized {
		t.Fatalf("unknown revoke: %d %s", unknownBearer.Code, unknownBearer.Body.String())
	}

	if _, err := db.AgentJoinToken(context.Background(), services.HashAPIKey(other), "https://other.example/mcp"); err == nil {
		t.Fatal("token accepted for a different resource")
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE agent_join_tokens SET expires_at=NOW() - INTERVAL '1 second' WHERE id=$1`, otherID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AgentJoinToken(context.Background(), services.HashAPIKey(other), h.resource()); err == nil {
		t.Fatal("expired token resolved")
	}
	if mcpBearerStatus(r, other) != http.StatusUnauthorized {
		t.Fatal("expired token opened mcp")
	}
	_ = server
}

func TestAgentJoinRateLimits(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", TokenLimiter: NewIPRateLimiter(1, time.Minute)}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	first, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	second, secondBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if second.Code != http.StatusTooManyRequests || secondBody["error"] != "rate limit exceeded" {
		t.Fatalf("minute cap: %d %s", second.Code, second.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != 1 {
		t.Fatal("limited request created an account")
	}

	ctx := context.Background()
	for i := 0; i < database.AgentJoinDailyCap; i++ {
		n := strconv.Itoa(i)
		if _, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("cap-"+n), "orat_cap", "same-ip", "http://127.0.0.1:8080/mcp", "Agent", "join:cap:"+n, "join-cap-"+n+"@agents.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("over"), "orat_over", "same-ip", "http://127.0.0.1:8080/mcp", "Agent", "join:over", "join-over@agents.invalid"); err != database.ErrJoinLimited {
		t.Fatalf("daily cap: %v", err)
	}

	ipHash := services.HashAPIKey("192.0.2.1")
	for i := 0; i < database.AgentJoinDailyCap-1; i++ {
		n := strconv.Itoa(i)
		if _, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("http-cap-"+n), "orat_htp", ipHash, "http://127.0.0.1:8080/mcp", "Agent", "join:http:"+n, "join-http-"+n+"@agents.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	open := gin.New()
	openEntry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080"}
	open.POST("/v1/agent-tokens", openEntry.CreateAgentToken)
	blocked, blockedBody := postJSON(t, open, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if blocked.Code != http.StatusTooManyRequests || blockedBody["error"] != "rate limit exceeded" {
		t.Fatalf("http daily: %d %s", blocked.Code, blocked.Body.String())
	}
}

func TestAgentJoinExpiresInRome(t *testing.T) {
	db, cleanup := setupTestDBInZone(t, "Europe/Rome")
	defer cleanup()
	var zone string
	if err := db.Pool.QueryRow(context.Background(), `SHOW timezone`).Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone == "UTC" || !strings.Contains(zone, "Rome") {
		t.Fatalf("timezone %q", zone)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &AgentEntry{DB: db, BaseURL: "http://join.example"}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, rec, body)
	var ahead bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT expires_at > NOW() + INTERVAL '23 hours 30 minutes' AND expires_at < NOW() + INTERVAL '24 hours 10 minutes' FROM agent_join_tokens WHERE id=$1`, body["token_id"]).Scan(&ahead); err != nil || !ahead {
		t.Fatalf("rome expiry ahead=%v %v", ahead, err)
	}
	grant, err := db.AgentJoinToken(context.Background(), services.HashAPIKey(token), "http://join.example/mcp")
	if err != nil || !grant.Join || grant.SiteID != "" || grant.MakerID != "" {
		t.Fatalf("rome resolve: %+v %v", grant, err)
	}
}

func TestLLMsJoinDoesNotRequireGoogle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{}).Mount(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("llms: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), AgentJoinCommand) || strings.Contains(rec.Body.String(), "/mcp needs the Google sign-in") || strings.Contains(rec.Body.String(), "This server needs a Google sign-in today") {
		t.Fatalf("fallback llms:\n%s", rec.Body.String())
	}
	root := filepath.Join("..", "..", "..")
	public, err := os.ReadFile(filepath.Join(root, "ui", "landing", "public", "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	built, err := os.ReadFile(filepath.Join(root, "affilio-wp", "admin", "build", "landing-v9", "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(public) != string(built) || !strings.Contains(string(public), AgentJoinCommand) || strings.Contains(string(public), "/mcp needs the Google sign-in") {
		t.Fatal("served llms.txt still sends agents to Google")
	}
}

func TestJoinOneLinerFromCleanFolder(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("curl is required")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, h, _, server := mountJoinTokens(t, db, 8)
	if h.BaseURL != server.URL {
		t.Fatalf("base %s server %s", h.BaseURL, server.URL)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", AgentJoinCommand)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "OFFER_RAILS_URL=" + h.BaseURL}
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("one liner: %v %s", err, out)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("clean folder gained %s", entries[0].Name())
	}
	text := string(out)
	for _, banned := range []string{"Sell something", "Earn from my posts", "I want to", "accounts.google.com", "/auth/login", "Sign in with Google"} {
		if strings.Contains(text, banned) {
			t.Fatalf("one liner contained %q\n%s", banned, text)
		}
	}
	var body map[string]any
	if err = json.Unmarshal(out, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["role"]; ok || body["token_type"] != "Bearer" || body["human_login"] != false {
		t.Fatalf("one liner body %s", text)
	}
	roles, _ := body["roles"].([]any)
	if len(roles) != 0 {
		t.Fatalf("roles %#v", body["roles"])
	}
	token, _ := body["access_token"].(string)
	names := mcpToolNames(t, ctx, server.URL+"/mcp", token)
	for _, want := range []string{"preview_revenue", "prepare_placement", "list_offers"} {
		if !names[want] {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
	if names["register_maker"] {
		t.Fatal("register_maker was offered")
	}
}
