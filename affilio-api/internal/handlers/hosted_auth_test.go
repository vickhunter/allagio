package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

type fakeOwnerLogin struct {
	owner           database.Owner
	nonce, verifier string
}

func (f *fakeOwnerLogin) AuthorizationURL(state, nonce, verifier string) string {
	f.nonce = nonce
	f.verifier = verifier
	return "https://identity.example/authorize?state=" + url.QueryEscape(state)
}
func (f *fakeOwnerLogin) Verify(_ context.Context, code, nonce, verifier string) (database.Owner, error) {
	if code != "valid-code" || nonce != f.nonce || verifier != f.verifier {
		return database.Owner{}, http.ErrNoCookie
	}
	return f.owner, nil
}

func hostedTestServer(t *testing.T, db *database.DB) (*HostedHandler, *gin.Engine, *httptest.Server) {
	t.Helper()
	r := gin.New()
	server := httptest.NewUnstartedServer(r)
	baseURL := "http://" + server.Listener.Addr().String()
	h := &HostedHandler{DB: db, BaseURL: baseURL, LoopbackURL: baseURL, Login: &fakeOwnerLogin{owner: database.Owner{Issuer: "https://identity.example", Subject: "verified-publisher", Email: "publisher@example.com", Name: "Publisher"}}}
	h.Mount(r)
	p := &PlacementsHandler{DB: db, RequireVerifiedConnection: true}
	site := r.Group("/v1", middleware.APIKeyAuth(db, nil, h.resource()))
	site.POST("/revenue/opportunities", p.Find)
	site.POST("/placements", p.Prepare)
	site.GET("/placements", p.List)
	site.GET("/placements/:id", p.Get)
	site.POST("/placements/:id/commit", p.Commit)
	site.POST("/placements/:id/receipt", p.RecordReceipt)
	site.POST("/placements/:id/revoke", p.Revoke)
	site.GET("/payouts/balance", (&PayoutsHandler{DB: db}).Balance)
	site.POST("/payouts/:id/confirm-received", (&PayoutsHandler{DB: db}).ConfirmReceived)
	maker := r.Group("/v1/maker", middleware.MakerKeyAuth(db, h.resource()))
	maker.GET("/products", (&ProductsHandler{DB: db}).List)
	r.POST("/v1/revenue/preview", p.Find)
	server.Start()
	t.Cleanup(server.Close)
	return h, r, server
}

func testOwnerSession(t *testing.T, h *HostedHandler, subject, email string) (*database.Owner, *http.Cookie) {
	t.Helper()
	token, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if err = h.DB.StartOwnerSession(context.Background(), database.Owner{Issuer: "https://identity.example", Subject: subject, Email: email, Name: "Pilot owner"}, services.HashAPIKey(token), csrf); err != nil {
		t.Fatal(err)
	}
	o, err := h.DB.OwnerSession(context.Background(), services.HashAPIKey(token))
	if err != nil {
		t.Fatal(err)
	}
	return o, &http.Cookie{Name: h.cookieName(), Value: token}
}

func ownerForm(t *testing.T, h *HostedHandler, r http.Handler, cookie *http.Cookie, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", h.BaseURL)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func authorizeTestAgent(t *testing.T, h *HostedHandler, r http.Handler, o *database.Owner, cookie *http.Cookie, scope string) (url.Values, string) {
	t.Helper()
	clientID, _ := randomToken()
	redirect := "http://127.0.0.1:32001/callback"
	if err := h.DB.SaveOAuthClient(context.Background(), database.OAuthClient{ID: clientID, Name: "Test MCP client", RedirectURIs: []string{redirect}}); err != nil {
		t.Fatal(err)
	}
	verifier, _ := randomToken()
	q := url.Values{"client_id": {clientID}, "redirect_uri": {redirect}, "resource": {h.resource()}, "response_type": {"code"}, "scope": {scope}, "code_challenge": {oauth2.S256ChallengeFromVerifier(verifier)}, "code_challenge_method": {"S256"}, "state": {"client-state"}}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("authorize: %d %s", rec.Code, rec.Body.String())
	}
	match := regexp.MustCompile(`name="request_id" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if len(match) != 2 {
		t.Fatalf("no consent request: %s", rec.Body.String())
	}
	consent := ownerForm(t, h, r, cookie, "/owner/authorize", url.Values{"csrf": {o.CSRF}, "request_id": {match[1]}})
	if consent.Code != 303 {
		t.Fatalf("consent: %d %s", consent.Code, consent.Body.String())
	}
	u, err := url.Parse(consent.Header().Get("Location"))
	if err != nil || u.Query().Get("state") != "client-state" {
		t.Fatal("client state lost")
	}
	return url.Values{"grant_type": {"authorization_code"}, "code": {u.Query().Get("code")}, "client_id": {clientID}, "redirect_uri": {redirect}, "resource": {h.resource()}, "code_verifier": {verifier}}, clientID
}

func exchangeTestToken(t *testing.T, h *HostedHandler, r http.Handler, values url.Values) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := ownerForm(t, h, r, nil, "/oauth/token", values)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("token response: %d %s", rec.Code, rec.Body.String())
	}
	return rec, out
}

func TestHostedLoginClaimsAndCSRF(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	start := httptest.NewRecorder()
	r.ServeHTTP(start, httptest.NewRequest("GET", "/auth/login", nil))
	if start.Code != 302 {
		t.Fatal(start.Code)
	}
	loginURL, _ := url.Parse(start.Header().Get("Location"))
	cookies := start.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("missing protected login cookie")
	}
	callback := httptest.NewRequest("GET", "/auth/callback?code=valid-code&state="+url.QueryEscape(loginURL.Query().Get("state")), nil)
	callback.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, callback)
	if rec.Code != 302 {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	var ownerCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == h.cookieName() {
			ownerCookie = cookie
		}
	}
	if ownerCookie == nil || !ownerCookie.HttpOnly || ownerCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("missing protected session cookie")
	}
	replay := httptest.NewRecorder()
	r.ServeHTTP(replay, callback.Clone(context.Background()))
	if replay.Code != 400 {
		t.Fatal("login state reused")
	}
	o, err := h.currentOwner(func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/owner", nil)
		c.Request.AddCookie(ownerCookie)
		return c
	}())
	if err != nil {
		t.Fatal(err)
	}
	bad := ownerForm(t, h, r, ownerCookie, "/owner/accounts", url.Values{"role": {"maker"}, "csrf": {"wrong"}})
	if bad.Code != 403 {
		t.Fatal("CSRF bypass")
	}
	good := ownerForm(t, h, r, ownerCookie, "/owner/accounts", url.Values{"role": {"maker"}, "csrf": {o.CSRF}})
	if good.Code != 303 {
		t.Fatalf("maker onboarding: %d %s", good.Code, good.Body.String())
	}
	var makerID string
	if err = db.Pool.QueryRow(context.Background(), `SELECT maker_id FROM owners WHERE id=$1`, o.ID).Scan(&makerID); err != nil || makerID == "" {
		t.Fatalf("maker not bound: %v", err)
	}
	other, _ := testOwnerSession(t, h, "different-subject", o.Email)
	if other.MakerID != "" {
		t.Fatal("email match transferred ownership")
	}
	external := httptest.NewRecorder()
	r.ServeHTTP(external, httptest.NewRequest("GET", "/auth/login?return_to=https://evil.example", nil))
	if external.Code != 400 {
		t.Fatal("open login redirect")
	}
}

func TestHostedMakerSetupCreatesOnlyNewOwnedAccount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "setup-maker", "setup-maker@example.com")
	values, _ := authorizeTestAgent(t, h, r, o, cookie, "maker")
	rec, token := exchangeTestToken(t, h, r, values)
	if rec.Code != 200 {
		t.Fatal("maker setup did not reach token exchange")
	}
	grant, err := db.AgentToken(context.Background(), services.HashAPIKey(token["access_token"].(string)), h.resource())
	if err != nil || grant.MakerID == "" || grant.SiteID != "" {
		t.Fatal("maker setup did not bind exactly the maker account")
	}
	var key *string
	if err = db.Pool.QueryRow(context.Background(), `SELECT api_key FROM makers WHERE id=$1`, grant.MakerID).Scan(&key); err != nil || key != nil {
		t.Fatal("hosted setup created an unnecessary legacy key")
	}
	authorizeTestAgent(t, h, r, o, cookie, "maker")
	var count int
	if err = db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM makers WHERE email=$1`, o.Email).Scan(&count); err != nil || count != 1 {
		t.Fatal("rerun duplicated maker")
	}
	foreign, err := db.CreateMaker(context.Background(), "existing-maker-owner", "existing-setup@example.com", "Existing maker")
	if err != nil {
		t.Fatal(err)
	}
	_, foreignCookie := testOwnerSession(t, h, "other-identity", "existing-setup@example.com")
	request := httptest.NewRequest("GET", "/owner?setup=maker", nil)
	request.AddCookie(foreignCookie)
	dashboard := httptest.NewRecorder()
	r.ServeHTTP(dashboard, request)
	if dashboard.Code != 200 {
		t.Fatal("existing-account recovery form missing")
	}
	var ownerID *string
	if err = db.Pool.QueryRow(context.Background(), `SELECT id::text FROM owners WHERE maker_id=$1`, foreign).Scan(&ownerID); err == nil {
		t.Fatal("matching email claimed another maker")
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	cloned := r.Clone(r.Context())
	cloned.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(cloned)
}

func TestHostedPublisherApprovalAndRealHTTPMCPClient(t *testing.T) {
	db, _, siteKey, offerID, _ := testPaidCampaign(t)
	h, r, server := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "publisher-owner", "publisher@example.com")
	claim := ownerForm(t, h, r, cookie, "/owner/accounts", url.Values{"csrf": {o.CSRF}, "role": {"publisher"}, "api_key": {siteKey}})
	if claim.Code != 303 {
		t.Fatalf("claim: %d %s", claim.Code, claim.Body.String())
	}
	o, _ = h.DB.OwnerSession(context.Background(), services.HashAPIKey(cookie.Value))
	values, _ := authorizeTestAgent(t, h, r, o, cookie, "publisher")
	tokenRec, tokens := exchangeTestToken(t, h, r, values)
	if tokenRec.Code != 200 {
		t.Fatal(tokens)
	}
	token := tokens["access_token"].(string)
	unauth := httptest.NewRecorder()
	r.ServeHTTP(unauth, httptest.NewRequest("POST", "/mcp", strings.NewReader(`{}`)))
	if unauth.Code != 401 || !strings.Contains(unauth.Header().Get("WWW-Authenticate"), "oauth-protected-resource/mcp") {
		t.Fatal("OAuth discovery unavailable")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "pilot-sdk-client", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}}, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "register_maker" || tool.Name == "register_product" || strings.Contains(tool.Name, "approve") {
			t.Fatalf("publisher exposed %s", tool.Name)
		}
	}
	call := func(name string, args map[string]any, wantError bool) map[string]any {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("tool %s error=%v content=%v", name, result.IsError, result.Content)
		}
		out := map[string]any{}
		if !wantError {
			if err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	before := call("find_opportunities", map[string]any{"context": "InboxPilot email inbox productivity tutorial"}, false)
	if before["status"] != "no_match" {
		t.Fatal("unverified connection eligible")
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE product_connections SET configuration_verified_at=NOW()`); err != nil {
		t.Fatal(err)
	}
	prepared := call("prepare_placement", map[string]any{"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": "InboxPilot email inbox productivity tutorial", "copy": "Consider InboxPilot for organizing an email inbox."}, false)
	p := prepared["placement"].(map[string]any)
	id := p["id"].(string)
	if prepared["owner_approval_url"] != h.BaseURL+"/owner/placements/"+id {
		t.Fatal("missing owner review URL")
	}
	call("commit_placement", map[string]any{"placement_id": id, "confirm": true}, true)
	for _, path := range []string{"/owner/placements/" + id + "/approve", "/v1/payouts/11111111-1111-1111-1111-111111111111/confirm-received"} {
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 401 && rec.Code != 403 {
			t.Fatalf("agent approved protected action: %s %d", path, rec.Code)
		}
	}
	foreign, foreignCookie := testOwnerSession(t, h, "foreign-owner", "foreign@example.com")
	create := ownerForm(t, h, r, foreignCookie, "/owner/accounts", url.Values{"csrf": {foreign.CSRF}, "role": {"publisher"}, "domain": {"other.example.com"}})
	if create.Code != 303 {
		t.Fatal(create.Body.String())
	}
	foreignApprove := ownerForm(t, h, r, foreignCookie, "/owner/placements/"+id+"/approve", url.Values{"csrf": {foreign.CSRF}})
	if foreignApprove.Code != 409 {
		t.Fatal("cross-owner approval")
	}
	approve := ownerForm(t, h, r, cookie, "/owner/placements/"+id+"/approve", url.Values{"csrf": {o.CSRF}})
	if approve.Code != 303 {
		t.Fatalf("approval: %d %s", approve.Code, approve.Body.String())
	}
	reviewRequest := httptest.NewRequest("GET", "/owner/placements/"+id, nil)
	reviewRequest.AddCookie(cookie)
	review := httptest.NewRecorder()
	r.ServeHTTP(review, reviewRequest)
	if review.Code != 200 || !strings.Contains(review.Body.String(), p["copy"].(string)) || !strings.Contains(review.Body.String(), "excluding tax and shipping") {
		t.Fatal("owner review omitted the approved copy or readable financial terms")
	}
	if !strings.Contains(review.Body.String(), "Destination: "+p["terms"].(map[string]any)["landing_url"].(string)) {
		t.Fatal("owner review omitted the approved destination")
	}
	committed := call("commit_placement", map[string]any{"placement_id": id}, false)
	if committed["placement"].(map[string]any)["status"] != "committed" {
		t.Fatal("approved placement not committed")
	}
	grants, err := db.OwnerGrants(ctx, o.ID)
	if err != nil || len(grants) != 1 {
		t.Fatal(err)
	}
	revoked := ownerForm(t, h, r, cookie, "/owner/grants/"+grants[0].ID+"/revoke", url.Values{"csrf": {o.CSRF}})
	if revoked.Code != 303 {
		t.Fatal(revoked.Code)
	}
	if _, err = db.AgentToken(ctx, services.HashAPIKey(token), h.resource()); err == nil {
		t.Fatal("revoked token usable")
	}
}

func TestHostedPKCECodeReplayRefreshAndAudience(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "maker-owner", "maker-owner@example.com")
	if rec := ownerForm(t, h, r, cookie, "/owner/accounts", url.Values{"csrf": {o.CSRF}, "role": {"maker"}}); rec.Code != 303 {
		t.Fatal(rec.Body.String())
	}
	o, _ = db.OwnerSession(context.Background(), services.HashAPIKey(cookie.Value))
	values, clientID := authorizeTestAgent(t, h, r, o, cookie, "maker")
	bad, _ := url.ParseQuery(values.Encode())
	bad.Set("code_verifier", strings.Repeat("x", 43))
	if rec, _ := exchangeTestToken(t, h, r, bad); rec.Code != 400 {
		t.Fatal("wrong PKCE verifier accepted")
	}
	bad, _ = url.ParseQuery(values.Encode())
	bad.Set("resource", "https://other.example/mcp")
	if rec, _ := exchangeTestToken(t, h, r, bad); rec.Code != 400 {
		t.Fatal("wrong audience accepted")
	}
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- ownerForm(t, h, r, nil, "/oauth/token", values) }()
	}
	wg.Wait()
	close(responses)
	var tokens map[string]any
	successes := 0
	for rec := range responses {
		if rec.Code == 200 {
			successes++
			if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
				t.Fatal(err)
			}
		} else if rec.Code != 400 {
			t.Fatal(rec.Code)
		}
	}
	if successes != 1 {
		t.Fatalf("code exchanged %d times", successes)
	}
	access := tokens["access_token"].(string)
	if _, err := db.AgentToken(context.Background(), services.HashAPIKey(access), "https://other.example/mcp"); err == nil {
		t.Fatal("token accepted at another resource")
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE oauth_tokens SET expires_at=NOW()-INTERVAL '1 second' WHERE token_hash=$1`, services.HashAPIKey(access)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AgentToken(context.Background(), services.HashAPIKey(access), h.resource()); err == nil {
		t.Fatal("expired access token accepted")
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens["refresh_token"].(string)}, "client_id": {clientID}, "resource": {h.resource()}}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE oauth_grants SET expires_at=NOW()+INTERVAL '45 seconds' WHERE client_id=$1`, clientID); err != nil {
		t.Fatal(err)
	}
	rec, rotated := exchangeTestToken(t, h, r, refresh)
	if rec.Code != 200 || rotated["refresh_token"] == tokens["refresh_token"] {
		t.Fatal("refresh not rotated")
	}
	if rotated["expires_in"].(float64) > 45 {
		t.Fatal("token response overstated the remaining grant lifetime")
	}
	if rec, _ = exchangeTestToken(t, h, r, refresh); rec.Code != 400 {
		t.Fatal("refresh reused")
	}
	if _, err := db.AgentToken(context.Background(), services.HashAPIKey(rotated["access_token"].(string)), h.resource()); err == nil {
		t.Fatal("replayed refresh did not revoke grant")
	}
}

func TestHostedClientRegistrationValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r, _ := hostedTestServer(t, db)
	for _, redirect := range []string{"http://evil.example/callback", "https://user:pass@example.com/callback", "https://example.com/#fragment", "javascript:alert(1)"} {
		req := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{"redirect_uris":["`+redirect+`"]}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("unsafe redirect accepted: %s", redirect)
		}
	}
	req := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{"client_name":"Cursor pilot","redirect_uris":["http://127.0.0.1:32001/callback"],"token_endpoint_auth_method":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	if _, err := io.ReadAll(rec.Result().Body); err != nil {
		t.Fatal(err)
	}
}
