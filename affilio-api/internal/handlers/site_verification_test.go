package handlers

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/preview"

	"github.com/gin-gonic/gin"
)

func newVerifyRouter(db *database.DB, fetch preview.Fetcher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", Claims: entry.StubClaimChecker{}}
	api.Mount(r)
	sites := &SitesHandler{DB: db, Fetch: fetch}
	r.POST("/v1/sites/register", sites.Register)
	secured := r.Group("/v1", middleware.APIKeyAuth(db, nil))
	secured.GET("/sites/current", sites.Current)
	secured.GET("/sites/:id", sites.Get)
	secured.POST("/sites/:id/verify", sites.Verify)
	secured.POST("/placements", (&PlacementsHandler{DB: db, Claims: entry.StubClaimChecker{}}).Prepare)
	return r
}

func verificationFetcher(page *httptest.Server, dialed *[]string, lookup func(string) ([]net.IP, error)) preview.Fetcher {
	return preview.Fetcher{
		LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
			return lookup(host)
		},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			*dialed = append(*dialed, address)
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if previewBlocked(host) {
				return nil, errBlockedDial
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", page.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"http/1.1"}},
	}
}

type dialError string

func (e dialError) Error() string { return string(e) }

const errBlockedDial = dialError("blocked dial")

func previewBlocked(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func TestDomainVerificationSignupFileAndSecondAgent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var mu sync.Mutex
	fileBody := "not-the-token\n"
	fileStatus := http.StatusOK
	fileType := "text/plain; charset=utf-8"
	homeBody := "<html><head><title>Home</title></head><body>No token here.</body></html>"
	homeStatus := http.StatusOK
	page := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/.well-known/offer-rails.txt":
			w.Header().Set("Content-Type", fileType)
			w.WriteHeader(fileStatus)
			_, _ = w.Write([]byte(fileBody))
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(homeStatus)
			_, _ = w.Write([]byte(homeBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(page.Close)

	var dialed []string
	r := newVerifyRouter(db, verificationFetcher(page, &dialed, func(host string) ([]net.IP, error) {
		if host == "verify.example" || host == "meta.example" || host == "register.example" {
			return []net.IP{net.ParseIP("203.0.113.40")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}))

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Verify Blog", "email": "verify-blog@example.com", "domain": "Verify.Example",
	}, nil)
	if created.Code != http.StatusCreated || body["verified"] != false || body["verified_at"] != nil {
		t.Fatalf("signup: %d %s", created.Code, created.Body.String())
	}
	token, _ := body["verification_token"].(string)
	siteID, _ := body["site_id"].(string)
	siteKey, _ := body["api_key"].(string)
	if len(token) != 48 || siteID == "" || siteKey == "" || body["plugin_key"] == "" {
		t.Fatalf("signup fields: %s", created.Body.String())
	}
	var stored string
	var verifiedAt *time.Time
	if err := db.Pool.QueryRow(context.Background(), `SELECT verification_token, verified_at FROM sites WHERE id=$1`, siteID).Scan(&stored, &verifiedAt); err != nil || stored != token || verifiedAt != nil {
		t.Fatalf("stored token %q at %v err %v", stored, verifiedAt, err)
	}

	current, currentBody := postJSON(t, r, http.MethodGet, "/v1/sites/current", nil, map[string]string{"X-API-Key": siteKey})
	if current.Code != http.StatusOK || currentBody["verification_token"] != token || currentBody["verified"] != false || currentBody["domain"] != "verify.example" {
		t.Fatalf("current: %d %s", current.Code, current.Body.String())
	}
	byID, byIDBody := postJSON(t, r, http.MethodGet, "/v1/sites/"+siteID, nil, map[string]string{"X-API-Key": siteKey})
	if byID.Code != http.StatusOK || byIDBody["verification_token"] != token {
		t.Fatalf("get: %d %s", byID.Code, byID.Body.String())
	}
	other, _ := postJSON(t, r, http.MethodGet, "/v1/sites/00000000-0000-4000-8000-000000000099", nil, map[string]string{"X-API-Key": siteKey})
	if other.Code != http.StatusNotFound {
		t.Fatalf("other site: %d %s", other.Code, other.Body.String())
	}
	anon, _ := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, nil)
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("anon verify: %d %s", anon.Code, anon.Body.String())
	}

	missing, missingBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if missing.Code != http.StatusUnprocessableEntity || missingBody["reason"] != "verification_missing" {
		t.Fatalf("missing: %d %s", missing.Code, missing.Body.String())
	}

	mu.Lock()
	fileBody = "# offer rails\n" + token + "\n"
	mu.Unlock()
	dialed = nil
	verified, verifiedBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if verified.Code != http.StatusOK || verifiedBody["verified"] != true || verifiedBody["verification_token"] != token || verifiedBody["verified_at"] == "" {
		t.Fatalf("verify: %d %s", verified.Code, verified.Body.String())
	}
	if _, err := time.Parse(time.RFC3339, verifiedBody["verified_at"].(string)); err != nil {
		t.Fatal(err)
	}
	if len(dialed) != 1 || dialed[0] != "203.0.113.40:443" {
		t.Fatalf("file proof dialed %v", dialed)
	}
	dialsAfter := len(dialed)
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if again.Code != http.StatusConflict || againBody["reason"] != "domain_verified" || len(dialed) != dialsAfter {
		t.Fatalf("reverify: %d %s dials %d", again.Code, again.Body.String(), len(dialed))
	}

	second, secondBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Second Blog", "email": "second-blog@example.com", "domain": "verify.example",
	}, nil)
	secondID, _ := secondBody["site_id"].(string)
	secondKey, _ := secondBody["api_key"].(string)
	secondToken, _ := secondBody["verification_token"].(string)
	if second.Code != http.StatusCreated || secondID == "" || secondID == siteID || secondToken == "" || secondToken == token || secondBody["verified"] != false {
		t.Fatalf("second signup: %d %s", second.Code, second.Body.String())
	}
	conflict, conflictBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+secondID+"/verify", map[string]any{}, map[string]string{"X-API-Key": secondKey})
	if conflict.Code != http.StatusConflict || conflictBody["reason"] != "domain_verified" || len(dialed) != dialsAfter {
		t.Fatalf("second verify: %d %s", conflict.Code, conflict.Body.String())
	}
	if strings.Contains(conflict.Body.String(), "127.0.0.1") || strings.Contains(conflict.Body.String(), token) {
		t.Fatalf("conflict leaked a secret: %s", conflict.Body.String())
	}

	_, offerID := seedListedOffer(t, db, "verify-offer-maker", "verify-offer@example.com", "CodeLens")
	contextText := "CodeLens devtools reviews every pull request. CodeLens finds bugs before merge. This devtools note is about CodeLens."
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://verify.example/posts/review", "context": contextText,
		"copy": "We use CodeLens to catch bugs before merge.", "post_id": 42, "post_title": "Review notes",
	}, map[string]string{"X-API-Key": siteKey})
	if prepared.Code != http.StatusCreated || !strings.Contains(preparedBody["next_action"].(string), "operator") {
		t.Fatalf("placement: %d %s", prepared.Code, prepared.Body.String())
	}
	var status, approved string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, COALESCE(approved_by_kind,'') FROM placement_proposals WHERE site_id=$1`, siteID).Scan(&status, &approved); err != nil || status != "pending" || approved != "" {
		t.Fatalf("verified site placement status %s approved %q err %v", status, approved, err)
	}

	maker, makerBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "maker", "name": "Verify Maker", "email": "verify-maker@example.com",
	}, nil)
	if maker.Code != http.StatusCreated || makerBody["verification_token"] != nil || makerBody["verified"] != nil {
		t.Fatalf("maker: %d %s", maker.Code, maker.Body.String())
	}
}

func TestDomainVerificationMetaAndUnreachable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	var mu sync.Mutex
	home := `<html><head><meta content="WRONG" name="offer-rails-verification"></head></html>`
	page := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/.well-known/offer-rails.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(home))
	}))
	t.Cleanup(page.Close)
	var dialed []string
	r := newVerifyRouter(db, verificationFetcher(page, &dialed, func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.41")}, nil
	}))
	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Meta Blog", "email": "meta-blog@example.com", "domain": "meta.example",
	}, nil)
	token := body["verification_token"].(string)
	siteID := body["site_id"].(string)
	siteKey := body["api_key"].(string)
	if created.Code != http.StatusCreated || token == "" {
		t.Fatal(created.Body.String())
	}
	held, heldBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if held.Code != http.StatusUnprocessableEntity || heldBody["reason"] != "verification_missing" {
		t.Fatalf("wrong meta: %d %s", held.Code, held.Body.String())
	}
	mu.Lock()
	home = `<html><head><meta content="` + token + `" name="Offer-Rails-Verification"></head><body></body></html>`
	mu.Unlock()
	verified, verifiedBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if verified.Code != http.StatusOK || verifiedBody["verified"] != true {
		t.Fatalf("meta verify: %d %s", verified.Code, verified.Body.String())
	}

	blockedDialed := 0
	blocked := newVerifyRouter(db, preview.Fetcher{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("10.1.2.3")}, nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			blockedDialed++
			return nil, errBlockedDial
		},
	})
	reg, regBody := postJSON(t, blocked, http.MethodPost, "/v1/sites/register", map[string]any{"domain": "blocked.example", "name": "Blocked"}, nil)
	regID, _ := regBody["site_id"].(string)
	regKey, _ := regBody["api_key"].(string)
	regToken, _ := regBody["verification_token"].(string)
	if reg.Code != http.StatusOK || regBody["verified"] != false || regToken == "" || regID == "" {
		t.Fatalf("register: %d %s", reg.Code, reg.Body.String())
	}
	got, gotBody := postJSON(t, blocked, http.MethodGet, "/v1/sites/current", nil, map[string]string{"X-API-Key": regKey})
	if got.Code != http.StatusOK || gotBody["verification_token"] != regToken {
		t.Fatalf("register get: %d %s", got.Code, got.Body.String())
	}
	unreachable, unreachableBody := postJSON(t, blocked, http.MethodPost, "/v1/sites/"+regID+"/verify", map[string]any{}, map[string]string{"X-API-Key": regKey})
	if unreachable.Code != http.StatusUnprocessableEntity || unreachableBody["reason"] != "verification_unreachable" || blockedDialed != 0 {
		t.Fatalf("unreachable: %d %s dials %d", unreachable.Code, unreachable.Body.String(), blockedDialed)
	}
	if strings.Contains(unreachable.Body.String(), "10.1.2.3") {
		t.Fatalf("unreachable leaked an address: %s", unreachable.Body.String())
	}
}

func TestVerificationTokenMatchers(t *testing.T) {
	token := "abc123token"
	if !fileProves(preview.Document{ContentType: "text/plain", Body: []byte("note\n" + token + "\n")}, token) {
		t.Fatal("line token was not accepted")
	}
	if fileProves(preview.Document{ContentType: "application/json", Body: []byte(token)}, token) {
		t.Fatal("json body was accepted as the file")
	}
	if fileProves(preview.Document{ContentType: "text/plain", Body: []byte("prefix " + token)}, token) {
		t.Fatal("a longer line was accepted")
	}
	page := `<html><head><meta name="offer-rails-verification" content="` + token + `"></head></html>`
	swapped := `<html><meta content="` + token + `" name="offer-rails-verification"></html>`
	if !metaProves(preview.Document{ContentType: "text/html", Body: []byte(page)}, token) || !metaProves(preview.Document{ContentType: "text/html", Body: []byte(swapped)}, token) {
		t.Fatal("meta tag was not accepted")
	}
	if metaProves(preview.Document{ContentType: "text/html", Body: []byte(`<meta name="offer-rails-verification-extra" content="` + token + `">`)}, token) {
		t.Fatal("a longer meta name was accepted")
	}
	bare := preview.Document{ContentType: "", Body: []byte(token + "\n")}
	if !fileProves(bare, token) {
		t.Fatal("missing content type was refused")
	}
}
