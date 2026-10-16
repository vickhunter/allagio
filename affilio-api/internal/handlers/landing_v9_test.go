package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func TestLandingV9RoutesStayOnTheGoApp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)

	for _, path := range []string{"/", "/makers", "/makers/", "/publishers", "/publishers/"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Referrer-Policy") != "strict-origin" {
			t.Fatalf("%s Referrer-Policy = %q", path, rec.Header().Get("Referrer-Policy"))
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("%s content type %q", path, ct)
		}
	}

	home := httptest.NewRecorder()
	r.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	body := home.Body.String()
	if !strings.Contains(body, `id="how"`) || !strings.Contains(body, `id="publishers"`) {
		t.Fatal("home is missing #how or #publishers")
	}
	if !strings.Contains(body, `href="/auth/login?return_to=%2Fowner%3Fsetup%3Dmaker"`) {
		t.Fatal("home sign-in link changed")
	}
	if strings.Contains(body, "source=") {
		t.Fatal("home without a source tag rewrote owner links")
	}

	owner := httptest.NewRecorder()
	r.ServeHTTP(owner, httptest.NewRequest(http.MethodGet, "/owner", nil))
	if owner.Code != http.StatusFound || !strings.HasPrefix(owner.Header().Get("Location"), "/auth/login?return_to=") {
		t.Fatalf("owner sign-in redirect: %d %s", owner.Code, owner.Header().Get("Location"))
	}
	settings := httptest.NewRecorder()
	r.ServeHTTP(settings, httptest.NewRequest(http.MethodGet, "/owner/settings", nil))
	if settings.Code != http.StatusFound || !strings.Contains(settings.Header().Get("Location"), "/auth/login?return_to=") {
		t.Fatalf("owner settings: %d %s", settings.Code, settings.Header().Get("Location"))
	}
	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/auth/login?return_to=https://evil.example", nil))
	if login.Code != http.StatusBadRequest || !strings.Contains(login.Body.String(), "invalid return path") {
		t.Fatalf("google sign-in: %d %s", login.Code, login.Body.String())
	}
}

func TestWordPressDownloadHeadMatchesGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	payload := releasePluginZip(t, nil)
	if err := os.WriteFile(filepath.Join(dir, "affilio.zip"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: dir,
	}
	r := gin.New()
	h.Mount(r)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	getResp, err := http.Get(server.URL + "/downloads/affilio.zip")
	if err != nil {
		t.Fatal(err)
	}
	getBody, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != http.StatusOK || len(getBody) == 0 {
		t.Fatalf("GET %d len %d", getResp.StatusCode, len(getBody))
	}

	headResp, err := http.Head(server.URL + "/downloads/affilio.zip")
	if err != nil {
		t.Fatal(err)
	}
	headBody, err := io.ReadAll(headResp.Body)
	headResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if headResp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD status %d", headResp.StatusCode)
	}
	if len(headBody) != 0 {
		t.Fatalf("HEAD body len %d", len(headBody))
	}
	if headResp.Header.Get("Content-Type") != getResp.Header.Get("Content-Type") {
		t.Fatalf("Content-Type HEAD %q GET %q", headResp.Header.Get("Content-Type"), getResp.Header.Get("Content-Type"))
	}
	if headResp.Header.Get("Content-Length") == "" || headResp.Header.Get("Content-Length") != getResp.Header.Get("Content-Length") {
		t.Fatalf("Content-Length HEAD %q GET %q", headResp.Header.Get("Content-Length"), getResp.Header.Get("Content-Length"))
	}
	if int(headResp.ContentLength) != len(getBody) {
		t.Fatalf("HEAD Content-Length %d, GET body %d", headResp.ContentLength, len(getBody))
	}
}

func landingApp(t *testing.T, db *database.DB, limiter *IPRateLimiter) (*HostedHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		DB:            db,
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
		EventLimiter:  limiter,
	}
	r := gin.New()
	h.Mount(r)
	(&AgentEntry{DB: db, BaseURL: h.BaseURL}).Mount(r)
	return h, r
}

func postLandingEvent(t *testing.T, r http.Handler, raw, query string, cookie *http.Cookie, referer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/events"+query, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.8:4242"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func landingEventCount(t *testing.T, db *database.DB) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM landing_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLandingEventsAllowlistSourceAndLimits(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := landingApp(t, db, NewIPRateLimiter(30, time.Minute))

	rejected := []string{
		`{"event":"page_view","source":"test"}`,
		`{"event":"sign_in_click","source":"test"}`,
		`{"event":"landing_view","source":"test","email":"person@example.com"}`,
		`{"event":"landing_view","source":"person@example.com"}`,
		`{"event":"landing_view","ip":"203.0.113.8"}`,
	}
	for _, raw := range rejected {
		rec := postLandingEvent(t, r, raw, "", nil, "http://127.0.0.1:8080/from/test")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("payload %s: %d %s", raw, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "person@example.com") || strings.Contains(rec.Body.String(), "203.0.113.8") {
			t.Fatalf("error echoed a payload value: %s", rec.Body.String())
		}
	}
	if landingEventCount(t, db) != 0 {
		t.Fatal("rejected events were stored")
	}

	oversized := `{"event":"landing_view","source":"` + strings.Repeat("a", 2000) + `"}`
	rec := postLandingEvent(t, r, oversized, "", nil, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized payload: %d %s", rec.Code, rec.Body.String())
	}
	if landingEventCount(t, db) != 0 {
		t.Fatal("oversized event was stored")
	}

	for _, name := range []string{"landing_view", "signin_click", "zip_download"} {
		rec = postLandingEvent(t, r, `{"event":"`+name+`","source":"test"}`, "", nil, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	var stored int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM landing_events WHERE name='signin_click' AND source='test'`).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("signin_click rows %d %v", stored, err)
	}

	rec = postLandingEvent(t, r, `{"event":"landing_view"}`, "?source=cursor", nil, "http://127.0.0.1:8080/from/ignored")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("query source: %d %s", rec.Code, rec.Body.String())
	}
	rec = postLandingEvent(t, r, `{"event":"zip_download"}`, "", &http.Cookie{Name: "or_source", Value: "cli"}, "http://127.0.0.1:8080/from/other")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cookie source: %d %s", rec.Code, rec.Body.String())
	}
	rec = postLandingEvent(t, r, `{"event":"signin_click","source":"grok"}`, "", &http.Cookie{Name: "or_source", Value: "cli"}, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("body source: %d %s", rec.Code, rec.Body.String())
	}
	rec = postLandingEvent(t, r, `{"event":"landing_view"}`, "", nil, "http://127.0.0.1:8080/from/test")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("referrer-only source: %d %s", rec.Code, rec.Body.String())
	}

	var cursor, cli, grok, blank int
	if err := db.Pool.QueryRow(context.Background(), `SELECT
		COUNT(*) FILTER (WHERE name='landing_view' AND source='cursor'),
		COUNT(*) FILTER (WHERE name='zip_download' AND source='cli'),
		COUNT(*) FILTER (WHERE name='signin_click' AND source='grok'),
		COUNT(*) FILTER (WHERE source='')
		FROM landing_events`).Scan(&cursor, &cli, &grok, &blank); err != nil {
		t.Fatal(err)
	}
	if cursor != 1 || cli != 1 || grok != 1 || blank != 1 {
		t.Fatalf("source propagation cursor=%d cli=%d grok=%d blank=%d", cursor, cli, grok, blank)
	}

	var columns string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(string_agg(column_name, ',' ORDER BY column_name), '')
		FROM information_schema.columns WHERE table_schema='public' AND table_name='landing_events'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != "created_at,id,name,source" {
		t.Fatalf("landing_events columns = %s", columns)
	}
}

func TestLandingEventRateLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := landingApp(t, db, NewIPRateLimiter(2, time.Minute))
	for i := 0; i < 2; i++ {
		rec := postLandingEvent(t, r, `{"event":"landing_view","source":"test"}`, "", nil, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("event %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := postLandingEvent(t, r, `{"event":"landing_view","source":"test"}`, "", nil, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit: %d %s", rec.Code, rec.Body.String())
	}
	if landingEventCount(t, db) != 2 {
		t.Fatalf("stored %d events past the limit", landingEventCount(t, db))
	}
}

func TestFromSourceSurvivesStrictReferrer(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := landingApp(t, db, NewIPRateLimiter(30, time.Minute))

	from := httptest.NewRecorder()
	r.ServeHTTP(from, httptest.NewRequest(http.MethodGet, "/from/test", nil))
	if from.Code != http.StatusFound || from.Header().Get("Location") != h.BaseURL+"/" {
		t.Fatalf("from redirect: %d %s", from.Code, from.Header().Get("Location"))
	}
	if from.Header().Get("Referrer-Policy") != "strict-origin" {
		t.Fatalf("from Referrer-Policy = %q", from.Header().Get("Referrer-Policy"))
	}
	var cookie *http.Cookie
	for _, item := range from.Result().Cookies() {
		if item.Name == "or_source" {
			cookie = item
		}
	}
	if cookie == nil || cookie.Value != "test" {
		t.Fatalf("or_source cookie = %#v", cookie)
	}
	directHits, err := db.SourceHits(context.Background(), "direct")
	if err != nil || directHits != 1 {
		t.Fatalf("unknown tag still counts as direct, hits %d %v", directHits, err)
	}

	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.AddCookie(cookie)
	page.Header.Set("Referer", "http://127.0.0.1:8080/")
	home := httptest.NewRecorder()
	r.ServeHTTP(home, page)
	if home.Header().Get("Referrer-Policy") != "strict-origin" {
		t.Fatalf("home Referrer-Policy = %q", home.Header().Get("Referrer-Policy"))
	}
	body := home.Body.String()
	if !strings.Contains(body, `/owner/settings?source=test`) {
		t.Fatal("owner link dropped the source tag")
	}
	if !strings.Contains(body, `/auth/login?`) || !strings.Contains(body, `source=test`) {
		t.Fatal("sign-in link dropped the source tag")
	}

	bare := httptest.NewRequest(http.MethodGet, "/", nil)
	bare.Header.Set("Referer", "http://127.0.0.1:8080/from/test")
	bareRec := httptest.NewRecorder()
	r.ServeHTTP(bareRec, bare)
	if strings.Contains(bareRec.Body.String(), "source=test") {
		t.Fatal("strict-origin referrer path was used as the source")
	}

	query := httptest.NewRequest(http.MethodGet, "/makers?source=test", nil)
	query.Header.Set("Referer", "http://127.0.0.1:8080/")
	makers := httptest.NewRecorder()
	r.ServeHTTP(makers, query)
	if !strings.Contains(makers.Body.String(), `source=test`) || !strings.Contains(makers.Body.String(), `/auth/login?`) {
		t.Fatal("makers page dropped ?source=")
	}

	click := postLandingEvent(t, r, `{"event":"signin_click"}`, "", cookie, "http://127.0.0.1:8080/")
	if click.Code != http.StatusNoContent {
		t.Fatalf("signin click: %d %s", click.Code, click.Body.String())
	}
	var rows int
	if err = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM landing_events WHERE name='signin_click' AND source='test'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("signin_click source=test rows %d %v", rows, err)
	}
}
