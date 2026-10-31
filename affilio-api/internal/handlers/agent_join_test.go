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

	"affilio-api/internal/config"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func mountJoinTokens(t *testing.T, db *database.DB, perMinute int) (*AgentEntry, *HostedHandler, *gin.Engine, *httptest.Server) {
	t.Helper()
	h, r, server := hostedTestServer(t, db)
	entry := &AgentEntry{DB: db, BaseURL: h.BaseURL, AddressKey: testAddressKey}
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
	var handle string
	if err := db.Pool.QueryRow(context.Background(), `SELECT name FROM owners WHERE id=$1`, emptyBody["owner_id"]).Scan(&handle); err != nil {
		t.Fatal(err)
	}
	if len(handle) != len("agent-")+8 || !strings.HasPrefix(handle, "agent-") || handle == "Agent" {
		t.Fatalf("generated handle %q", handle)
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

var testAddressKey = []byte("test-address-secret")

func TestClientAddressHMACUsesAPIKeySecret(t *testing.T) {
	key, warning := services.ClientAddressHMACKey("api-secret")
	if warning != "" || string(key) != "api-secret" {
		t.Fatalf("stable key %q %q", key, warning)
	}
	empty, emptyWarning := services.ClientAddressHMACKey("  ")
	if len(empty) != 32 || !strings.Contains(emptyWarning, "API_KEY_SECRET is unset") {
		t.Fatalf("fallback %d %q", len(empty), emptyWarning)
	}
	other, _ := services.ClientAddressHMACKey("")
	if string(other) == string(empty) {
		t.Fatal("fallback key was reused across calls")
	}
	ip := "203.0.113.10"
	got := services.HashClientAddress(key, ip)
	if got == services.HashAPIKey(ip) {
		t.Fatal("client address hash is unsalted sha256")
	}
	salt := GoLinkDailySalt("api-secret", time.Now().UTC())
	if got == GoLinkClientHash(ip, salt) || got == GoLinkClientHash(TruncateReaderIP(ip), salt) {
		t.Fatal("client address hash uses the daily go-link salt")
	}
	if services.HashClientAddress(key, ip) != got {
		t.Fatal("hmac was not stable")
	}
}

func TestAgentTokenEleventhRapidRequestIs429(t *testing.T) {
	if OpenSignupPerMinute != 8 {
		t.Fatalf("per-minute cap is %d; the served text says 8 and must not be raised", OpenSignupPerMinute)
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		TokenLimiter: NewIPRateLimiter(OpenSignupPerMinute, time.Minute),
		AddressKey:   testAddressKey,
	}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	for i := 1; i <= 11; i++ {
		rec, _ := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
		if i <= OpenSignupPerMinute {
			if rec.Code != http.StatusCreated {
				t.Fatalf("request %d status %d", i, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d status %d", i, rec.Code)
		}
	}
}

func TestAgentJoinRateLimits(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	shared := NewIPRateLimiter(2, time.Minute)
	r := gin.New()
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AccountLimiter: shared, TokenLimiter: shared, AddressKey: testAddressKey}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	r.POST("/v1/agent-accounts", entry.CreateAgentAccount)
	first, firstBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	ipHash := services.HashClientAddress(testAddressKey, "192.0.2.1")
	if ipHash == services.HashAPIKey("192.0.2.1") || ipHash == GoLinkClientHash("192.0.2.1", GoLinkDailySalt(string(testAddressKey), time.Now().UTC())) {
		t.Fatal("stored address hash uses the wrong key")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1 AND creator_ip_hash=$2`, firstBody["owner_id"], ipHash) != 1 {
		t.Fatal("join token stored a different address hash")
	}
	account, accountBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{"role": "publisher", "name": "Ada", "email": "ada@example.com", "domain": "ada.example"}, nil)
	if account.Code != http.StatusCreated || accountBody["api_key"] == "" {
		t.Fatalf("account: %d %s", account.Code, account.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_open_signups WHERE creator_ip_hash=$1`, ipHash) != 1 {
		t.Fatal("account signup was not counted")
	}
	third, thirdBody := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if third.Code != http.StatusTooManyRequests || thirdBody["error"] != "rate limit exceeded" {
		t.Fatalf("shared minute cap: %d %s", third.Code, third.Body.String())
	}
	fourth, _ := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{"role": "maker", "name": "Bea", "email": "bea@example.com"}, nil)
	if fourth.Code != http.StatusTooManyRequests {
		t.Fatalf("account minute cap: %d %s", fourth.Code, fourth.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != 2 {
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

	for i := 0; i < database.AgentJoinDailyCap-2; i++ {
		n := strconv.Itoa(i)
		if _, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("http-cap-"+n), "orat_htp", ipHash, "http://127.0.0.1:8080/mcp", "Agent", "join:http:"+n, "join-http-"+n+"@agents.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	open := gin.New()
	openEntry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey}
	open.POST("/v1/agent-tokens", openEntry.CreateAgentToken)
	open.POST("/v1/agent-accounts", openEntry.CreateAgentAccount)
	blocked, blockedBody := postJSON(t, open, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	if blocked.Code != http.StatusTooManyRequests || blockedBody["error"] != "rate limit exceeded" {
		t.Fatalf("http daily token: %d %s", blocked.Code, blocked.Body.String())
	}
	blockedAccount, blockedAccountBody := postJSON(t, open, http.MethodPost, "/v1/agent-accounts", map[string]any{"role": "maker", "name": "Cam", "email": "cam@example.com"}, nil)
	if blockedAccount.Code != http.StatusTooManyRequests || blockedAccountBody["error"] != "rate limit exceeded" {
		t.Fatalf("http daily account: %d %s", blockedAccount.Code, blockedAccount.Body.String())
	}
}

func TestPruneClientAddressHashes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	issued, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("old"), "orat_old", "hashed-ip", "http://join.example/mcp", "Agent", "join:old", "join-old@agents.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE agent_join_tokens SET created_at=NOW() - INTERVAL '25 hours', expires_at=NOW() - INTERVAL '1 hour' WHERE id=$1`, issued.TokenID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO agent_open_signups(creator_ip_hash, created_at) VALUES('hashed-ip', NOW() - INTERVAL '25 hours')`); err != nil {
		t.Fatal(err)
	}
	if err = db.PruneClientAddressHashes(ctx); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err = db.Pool.QueryRow(ctx, `SELECT creator_ip_hash FROM agent_join_tokens WHERE id=$1`, issued.TokenID).Scan(&hash); err != nil || hash != "" {
		t.Fatalf("join hash %q %v", hash, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_open_signups`) != 0 {
		t.Fatal("old account signup row remained")
	}
}

func TestJoinBearerCannotEscalate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry, h, r, _ := mountJoinTokens(t, db, 8)
	r.PUT("/v1/step-modes", entry.PutStepModes)
	r.POST("/v1/placements/:id/plugin-ack", entry.PluginAck)
	admin := r.Group("/v1/offer-review", middleware.AdminKeyAuth("test-admin", nil))
	admin.POST("/placements/:id/approve", (&PlacementsHandler{DB: db}).Approve)

	mint, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, mint, body)
	ownerID, _ := body["owner_id"].(string)
	auth := bearerHeader(token)
	id := "00000000-0000-4000-8000-0000000000aa"
	approveBody := map[string]any{"site_id": id, "owner_email": "owner@example.com", "evidence": "owner approved this placement"}

	asAdmin, asAdminBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", approveBody, map[string]string{"X-Admin-Key": token})
	if asAdmin.Code != http.StatusUnauthorized || asAdminBody["error"] != "invalid admin key" {
		t.Fatalf("admin key: %d %s", asAdmin.Code, asAdmin.Body.String())
	}
	asAdminBearer, asAdminBearerBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", approveBody, map[string]string{"Authorization": "Bearer " + token, "X-Admin-Key": token})
	if asAdminBearer.Code != http.StatusUnauthorized || asAdminBearerBody["error"] != "invalid admin key" {
		t.Fatalf("admin bearer: %d %s", asAdminBearer.Code, asAdminBearer.Body.String())
	}

	asPlugin, asPluginBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, map[string]string{"X-Plugin-Key": token})
	if asPlugin.Code != http.StatusForbidden || asPluginBody["error"] != "plugin key required" {
		t.Fatalf("plugin key: %d %s", asPlugin.Code, asPlugin.Body.String())
	}
	asPluginBearer, asPluginBearerBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, auth)
	if asPluginBearer.Code != http.StatusForbidden || asPluginBearerBody["error"] != "plugin key required" {
		t.Fatalf("plugin bearer: %d %s", asPluginBearer.Code, asPluginBearer.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/owner/placements/"+id+"/approve", strings.NewReader("csrf=nope"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", h.BaseURL)
	ownerRec := httptest.NewRecorder()
	r.ServeHTTP(ownerRec, req)
	if ownerRec.Code != http.StatusUnauthorized || !strings.Contains(ownerRec.Body.String(), "sign in at /auth/login") {
		t.Fatalf("owner approve: %d %s", ownerRec.Code, ownerRec.Body.String())
	}

	loose, looseBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"placement_publish": "auto"}, auth)
	if loose.Code != http.StatusForbidden || looseBody["error"] != "a person approves this step" {
		t.Fatalf("owner-wide auto: %d %s", loose.Code, loose.Body.String())
	}
	wide, wideBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"offer_match": "off"}, auth)
	if wide.Code != http.StatusForbidden || wideBody["error"] != "a person approves this step" {
		t.Fatalf("owner-wide mode: %d %s", wide.Code, wide.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1`, ownerID) != 0 {
		t.Fatal("join bearer wrote a step mode before a site existed")
	}

	place, _ := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{}, auth)
	if place.Code != http.StatusBadRequest {
		t.Fatalf("empty placement: %d %s", place.Code, place.Body.String())
	}
	var siteID, publish string
	err := db.Pool.QueryRow(context.Background(), `SELECT s.id::text, m.placement_publish
FROM sites s
JOIN owners o ON o.site_id = s.id
JOIN agent_step_modes m ON m.site_id = s.id
WHERE o.id=$1`, ownerID).Scan(&siteID, &publish)
	if err != nil || publish != "ask" || siteID == "" {
		t.Fatalf("seed ask site=%s publish=%s err=%v", siteID, publish, err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1 AND site_id IS NULL`, ownerID) != 0 {
		t.Fatal("owner-wide step mode exists")
	}

	again, againBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"placement_publish": "auto"}, auth)
	if again.Code != http.StatusForbidden || againBody["error"] != "a person approves this step" {
		t.Fatalf("site auto: %d %s", again.Code, again.Body.String())
	}
	var still string
	if err = db.Pool.QueryRow(context.Background(), `SELECT placement_publish FROM agent_step_modes WHERE site_id=$1`, siteID).Scan(&still); err != nil || still != "ask" {
		t.Fatalf("publish after auto %q %v", still, err)
	}

	tight, tightBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"offer_match": "off"}, auth)
	if tight.Code != http.StatusOK {
		t.Fatalf("tighten: %d %s", tight.Code, tight.Body.String())
	}
	modes, _ := tightBody["modes"].(map[string]any)
	if modes["offer_match"] != "off" || modes["placement_publish"] != "ask" {
		t.Fatalf("modes %#v", tightBody["modes"])
	}
}

func TestJoinBearerLookupErrorForbidsWrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry, _, r, _ := mountJoinTokens(t, db, 8)
	r.PUT("/v1/step-modes", entry.PutStepModes)
	t.Cleanup(func() { entry.beforeJoinBearer = nil })

	siteOf := func(ownerID string) (siteID, publish, match string) {
		t.Helper()
		err := db.Pool.QueryRow(context.Background(), `SELECT s.id::text, m.placement_publish, m.offer_match
FROM sites s
JOIN owners o ON o.site_id = s.id
JOIN agent_step_modes m ON m.site_id = s.id
WHERE o.id=$1`, ownerID).Scan(&siteID, &publish, &match)
		if err != nil {
			t.Fatal(err)
		}
		return siteID, publish, match
	}
	joinSite := func(token, ownerID string) {
		t.Helper()
		place, _ := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{}, bearerHeader(token))
		if place.Code != http.StatusBadRequest {
			t.Fatalf("empty placement: %d %s", place.Code, place.Body.String())
		}
		_, publish, match := siteOf(ownerID)
		if publish != "ask" || match != "auto" {
			t.Fatalf("seed publish=%s match=%s", publish, match)
		}
	}

	mint, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, mint, body)
	ownerID, _ := body["owner_id"].(string)
	joinSite(token, ownerID)
	var revokeErr error
	entry.beforeJoinBearer = func() {
		_, revokeErr = db.RevokeAgentJoinToken(context.Background(), services.HashAPIKey(token))
	}
	revoked, revokedBody := postJSON(t, r, http.MethodPut, "/v1/step-modes", map[string]any{"offer_match": "off"}, bearerHeader(token))
	if revokeErr != nil {
		t.Fatal(revokeErr)
	}
	if revoked.Code != http.StatusForbidden || revokedBody["error"] != "a person approves this step" {
		t.Fatalf("revoked between reads: %d %s", revoked.Code, revoked.Body.String())
	}
	var gone bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM agent_join_tokens WHERE token_hash=$1`, services.HashAPIKey(token)).Scan(&gone); err != nil || !gone {
		t.Fatalf("token revoked=%v %v", gone, err)
	}
	if _, publish, match := siteOf(ownerID); publish != "ask" || match != "auto" {
		t.Fatalf("revoked write publish=%s match=%s", publish, match)
	}

	mint, body = postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token = assertJoinIssued(t, mint, body)
	ownerID, _ = body["owner_id"].(string)
	joinSite(token, ownerID)
	ctx, cancel := context.WithCancel(context.Background())
	entry.beforeJoinBearer = func() { cancel() }
	req := httptest.NewRequest(http.MethodPut, "/v1/step-modes", strings.NewReader(`{"offer_match":"off"}`))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "a person approves this step") {
		t.Fatalf("lookup error: %d %s", rec.Code, rec.Body.String())
	}
	if _, publish, match := siteOf(ownerID); publish != "ask" || match != "auto" {
		t.Fatalf("error write publish=%s match=%s", publish, match)
	}
}

func TestJoinBearerFreshOwnerPlacementIsNotApproved(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	base := "http://join.example"
	r := gin.New()
	api := &AgentEntry{DB: db, BaseURL: base, AddressKey: testAddressKey, Claims: entry.LiveClaimChecker{}}
	r.POST("/v1/agent-tokens", api.CreateAgentToken)
	r.GET("/v1/step-modes", api.GetStepModes)
	placements := &PlacementsHandler{DB: db, Claims: entry.LiveClaimChecker{}}
	secured := r.Group("/v1", middleware.APIKeyAuth(db, nil, base+"/mcp"))
	secured.POST("/placements", placements.Prepare)

	mint, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
	token := assertJoinIssued(t, mint, body)
	ownerID, _ := body["owner_id"].(string)
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1`, ownerID) != 0 {
		t.Fatal("fresh owner already had a step-mode row")
	}
	got, gotBody := postJSON(t, r, http.MethodGet, "/v1/step-modes", nil, bearerHeader(token))
	modes := modeMap(t, gotBody)
	if got.Code != http.StatusOK || modes["placement_publish"] != "ask" || modes["site_signup"] != "auto" || modes["offer_match"] != "auto" || modes["link_change"] != "auto" || modes["payout"] != "auto" {
		t.Fatalf("fresh modes: %d %s", got.Code, got.Body.String())
	}

	_, offerID := seedListedOffer(t, db, "join-fresh-maker", "join-fresh-maker@example.com", "CodeLens")
	compact := strings.ReplaceAll(ownerID, "-", "")
	domain := "j" + compact[:12] + ".invalid"
	rec, placed := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://" + domain + "/posts/first",
		"context": stepArticle("CodeLens"), "copy": stepCopy, "post_id": 1,
	}, bearerHeader(token))
	if rec.Code != http.StatusCreated || placed["status"] == "published" || placed["error"] != nil {
		t.Fatalf("fresh placement: %d %s", rec.Code, rec.Body.String())
	}
	placement := placementBody(t, placed)
	if placement["status"] == "approved" || placement["status"] == "committed" || placement["approved_by"] != nil {
		t.Fatalf("join bearer published without a person: %s", rec.Body.String())
	}
	if placed["status"] != "awaiting_approval" || placement["status"] != "pending" {
		t.Fatalf("fresh placement was not held: %s", rec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes m JOIN owners o ON o.site_id = m.site_id WHERE o.id=$1 AND m.placement_publish='ask'`, ownerID) != 1 {
		t.Fatal("join site did not store placement publish ask")
	}
}

func TestOpenSignupClientAddressFollowsRender(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)

	mount := func(render bool, platform string) (*AgentEntry, *gin.Engine) {
		shared := NewIPRateLimiter(1, time.Minute)
		entry := &AgentEntry{
			DB: db, BaseURL: "http://127.0.0.1:8080",
			AccountLimiter: shared, TokenLimiter: shared, AddressKey: testAddressKey,
			Render: render, TrustedPlatform: platform,
		}
		r := gin.New()
		r.POST("/v1/agent-tokens", entry.CreateAgentToken)
		r.POST("/v1/agent-accounts", entry.CreateAgentAccount)
		return entry, r
	}
	_, renderRouter := mount(true, "")
	first, firstBody := serveOpenSignup(t, renderRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.0.0.8:443", "198.51.100.9, 203.0.113.10", nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("render client: %d %s", first.Code, first.Body.String())
	}
	clientHash := services.HashClientAddress(testAddressKey, "203.0.113.10")
	if clientHash == services.HashClientAddress(testAddressKey, "10.0.0.8") {
		t.Fatal("client hash collided with the proxy")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1 AND creator_ip_hash=$2`, firstBody["owner_id"], clientHash) != 1 {
		t.Fatal("stored hash is not the rightmost forwarded address")
	}
	spoof, _ := serveOpenSignup(t, renderRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.0.0.8:443", "203.0.113.50, 203.0.113.10", nil)
	if spoof.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed left address: %d %s", spoof.Code, spoof.Body.String())
	}
	account, accountBody := serveOpenSignup(t, renderRouter, http.MethodPost, "/v1/agent-accounts", `{"role":"maker","name":"Ada","email":"ada-render@example.com"}`, "10.0.0.9:443", "198.51.100.1, 203.0.113.10", nil)
	if account.Code != http.StatusTooManyRequests || accountBody["error"] != "rate limit exceeded" {
		t.Fatalf("shared account bucket: %d %s", account.Code, account.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_open_signups WHERE creator_ip_hash=$1`, clientHash) != 0 {
		t.Fatal("limited account signup was stored")
	}
	other, otherBody := serveOpenSignup(t, renderRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.0.0.8:443", "198.51.100.9, 203.0.113.11", nil)
	if other.Code != http.StatusCreated {
		t.Fatalf("second render client: %d %s", other.Code, other.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1 AND creator_ip_hash=$2`, otherBody["owner_id"], services.HashClientAddress(testAddressKey, "203.0.113.11")) != 1 {
		t.Fatal("second client did not get its own bucket")
	}

	_, platformRouter := mount(true, "CF-Connecting-IP")
	platform, platformBody := serveOpenSignup(t, platformRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.0.0.8:443", "203.0.113.10", map[string]string{"CF-Connecting-IP": "203.0.113.20"})
	if platform.Code != http.StatusCreated {
		t.Fatalf("platform client: %d %s", platform.Code, platform.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1 AND creator_ip_hash=$2`, platformBody["owner_id"], services.HashClientAddress(testAddressKey, "203.0.113.20")) != 1 {
		t.Fatal("stored hash ignored the platform header")
	}
	samePlatform, _ := serveOpenSignup(t, platformRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.0.0.8:443", "203.0.113.77", map[string]string{"CF-Connecting-IP": "203.0.113.20"})
	if samePlatform.Code != http.StatusTooManyRequests {
		t.Fatalf("same platform client: %d %s", samePlatform.Code, samePlatform.Body.String())
	}

	_, localRouter := mount(false, "")
	local, localBody := serveOpenSignup(t, localRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.1.0.5:9", "203.0.113.10", nil)
	if local.Code != http.StatusCreated {
		t.Fatalf("local client: %d %s", local.Code, local.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1 AND creator_ip_hash=$2`, localBody["owner_id"], services.HashClientAddress(testAddressKey, "10.1.0.5")) != 1 {
		t.Fatal("local hash used the forwarded address")
	}
	forwarded, _ := serveOpenSignup(t, localRouter, http.MethodPost, "/v1/agent-tokens", `{}`, "10.1.0.5:9", "203.0.113.11", nil)
	if forwarded.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed local forward: %d %s", forwarded.Code, forwarded.Body.String())
	}
}

func serveOpenSignup(t *testing.T, r http.Handler, method, path, raw, remote, forwarded string, extra map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	for key, value := range extra {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
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

func TestLLMsNamesORPTestDeal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{}).Mount(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("llms: %d", rec.Code)
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
	phrases := []string{
		"## ORP 0-3 test deal",
		"In testing",
		"/.well-known/orp.json",
		"POST /orp",
		"message format 0.3",
		"The schema is 0.3",
		"POST /v1/agent-tokens",
		"POST /orp/join",
		"{origin}/orp/proof/<deal>",
		"hello then discover then a directed offer (to) then accept by the second party then activate then execute then proof",
		`{"id":"place","by":"acceptor","action":"place","destination":"https://example.com/notes","proof":["proof_url"]}`,
		"obligation is the id from step 3",
		"The acceptor is the second party",
		"HTTP 403 forbidden",
		"HTTP 404 not_found",
		"placement_publish auto",
		"platform is one of x, blog, newsletter, email, web",
		"Allowed values: x, blog, newsletter, email, web.",
		"409 state_conflict",
		"The deal is in the wrong state.",
		"Execute does not read proof_url",
		"obligation state becomes claimed",
		"The deal stays active",
		"id is 8 to 128 characters of letters, digits, underscore, dot, colon, or hyphen",
		"A hello body requires versions",
		"https://offer-rails.onrender.com/orp/schemas/envelope.schema.json",
		"https://offer-rails.onrender.com/orp/spec",
		"body.terms_hash is optional",
		"A missing or empty terms_hash is HTTP 400 invalid_message",
		"Only a well-formed hash that does not match the terms is HTTP 409 terms_mismatch",
	}
	for _, item := range []struct {
		name string
		body string
	}{
		{"fallback", rec.Body.String()},
		{"public", string(public)},
		{"built", string(built)},
	} {
		body := config.ExpandPublicBase(item.body, "")
		if !strings.Contains(body, "## ORP 0-3 test deal") || !strings.Contains(body, "Cold path:") || !strings.Contains(body, "](https://offer-rails.onrender.com/skill.md)") || strings.Contains(body, "](https://offer-rails.onrender.com/llms.txt)") {
			t.Errorf("%s is not the cold path", item.name)
		}
	}
	fullBuilt, err := os.ReadFile(filepath.Join(root, "affilio-wp", "admin", "build", "landing-v9", "llms-full.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		body string
	}{
		{"full fallback", llmsFullDocument()},
		{"full built", string(fullBuilt)},
	} {
		if !strings.Contains(item.body, orpTestDealSection) {
			t.Errorf("%s does not contain the ORP test-deal section", item.name)
		}
		if strings.Contains(item.body, "PR #83 lands") || strings.Contains(item.body, "not claimed as live") || strings.Contains(orpTestDealSection, "The deal stays awaiting") {
			t.Errorf("%s still describes the pre-D-104 host", item.name)
		}
		expanded := config.ExpandPublicBase(item.body, "")
		for _, phrase := range phrases {
			if !strings.Contains(expanded, phrase) {
				t.Errorf("%s missing %q", item.name, phrase)
			}
		}
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
	if !strings.Contains(rec.Body.String(), "Cold path:") || !strings.Contains(rec.Body.String(), "POST /orp/join") || strings.Contains(rec.Body.String(), "/mcp needs the Google sign-in") || strings.Contains(rec.Body.String(), "This server needs a Google sign-in today") {
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
	if string(public) != string(built) {
		t.Fatal("llms.txt copies differ")
	}
	if strings.Contains(string(public), "/mcp needs the Google sign-in") || strings.Contains(llmsFullDocument(), "/mcp needs the Google sign-in") || strings.Contains(llmsFullDocument(), "This server needs a Google sign-in today") {
		t.Fatal("served llms text still sends agents to Google")
	}
	const stdioSentence = "The MCP server name is allagio-mcp. The npm package is coming soon. From this repository, run node cli/bin/offer-rails.mjs --api-url <origin>. Older binary names still start the same server. Set ALLAGIO_API_URL, ALLAGIO_API_KEY, and ALLAGIO_SITE_KEY. The previous names still work when the new name is unset."
	if !strings.Contains(llmsFullDocument(), AgentJoinCommand) || !strings.Contains(llmsFullDocument(), stdioSentence) {
		t.Fatal("llms-full.txt dropped the join command or the stdio sentence")
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
