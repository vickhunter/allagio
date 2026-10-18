package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/middleware"
	"affilio-api/internal/preview"
	"github.com/gin-gonic/gin"
)

func TestPreviewHappyPathAndNoMatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)

	var offersBefore, eventsBefore int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM offers`).Scan(&offersBefore); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM landing_events`).Scan(&eventsBefore); err != nil {
		t.Fatal(err)
	}

	makerID := insertPublisherMaker(t, db)
	_, wantID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "PixelQuest", []string{"game", "retro", "platformer"})

	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/match":
			_, _ = io.WriteString(w, `<html><head><title>Inbox cleanup</title></head><body><nav>Home</nav><main><p>InboxPilot email inbox productivity for founders who live in their inbox.</p></main><script>secret()</script></body></html>`)
		case "/garden":
			_, _ = io.WriteString(w, `<html><head><title>Tomato notes</title></head><body><main><p>Gardening tips for tomatoes in small gardens and watering herbs.</p></main></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer page.Close()

	h := &PreviewHandler{
		DB:      db,
		Limiter: NewIPRateLimiter(10, time.Minute),
		Fetch: preview.Fetcher{
			LookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			},
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				if address != "203.0.113.10:80" {
					t.Errorf("dialed %s", address)
				}
				return (&net.Dialer{}).DialContext(ctx, "tcp", page.Listener.Addr().String())
			},
		},
	}
	r := newPreviewRouter(h)

	matched := postPreview(t, r, `{"url":"http://preview.example/match"}`, "203.0.113.20:4000")
	if matched.Code != http.StatusOK {
		t.Fatalf("match: %d %s", matched.Code, matched.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(matched.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["title"] != "Inbox cleanup" || body["slot"] != "end_of_article" {
		t.Fatalf("body = %s", matched.Body.String())
	}
	offer, _ := body["offer"].(map[string]any)
	if offer == nil || offer["offer_id"] != wantID || offer["name"] != "InboxPilot" {
		t.Fatalf("offer = %#v", body["offer"])
	}
	if relevance, _ := offer["relevance"].(float64); relevance < float64(relevanceThreshold) {
		t.Fatalf("relevance = %v", offer["relevance"])
	}
	if _, ok := body["low_confidence"]; ok || body["offers"] != nil {
		t.Fatalf("confident match returned a fallback list: %s", matched.Body.String())
	}
	if !strings.Contains(matched.Body.String(), "InboxPilot") || strings.Contains(matched.Body.String(), "secret()") {
		t.Fatalf("body leaked page script or dropped the offer: %s", matched.Body.String())
	}

	miss := postPreview(t, r, `{"url":"http://preview.example/garden"}`, "203.0.113.20:4000")
	if miss.Code != http.StatusOK {
		t.Fatalf("garden: %d %s", miss.Code, miss.Body.String())
	}
	if strings.Contains(strings.ToLower(miss.Body.String()), "no match") {
		t.Fatalf("garden said no match: %s", miss.Body.String())
	}
	var weak map[string]any
	if err := json.Unmarshal(miss.Body.Bytes(), &weak); err != nil {
		t.Fatal(err)
	}
	weakOffers, _ := weak["offers"].([]any)
	if weak["title"] != "Tomato notes" || weak["low_confidence"] != true || weak["offer"] == nil || weak["slot"] == nil || len(weakOffers) != 2 {
		t.Fatalf("garden body = %s", miss.Body.String())
	}
	names := map[string]bool{}
	for _, item := range weakOffers {
		card, _ := item.(map[string]any)
		if card["low_confidence"] != true {
			t.Fatalf("card = %#v", card)
		}
		names[fmt.Sprint(card["name"])] = true
	}
	if !names["InboxPilot"] || !names["PixelQuest"] {
		t.Fatalf("garden offers = %s", miss.Body.String())
	}

	var offersAfter, eventsAfter int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM offers`).Scan(&offersAfter); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM landing_events`).Scan(&eventsAfter); err != nil {
		t.Fatal(err)
	}
	if offersAfter != offersBefore+2 || eventsAfter != eventsBefore {
		t.Fatalf("stored rows changed: offers %d -> %d, events %d -> %d", offersBefore, offersAfter, eventsBefore, eventsAfter)
	}
}

func TestPreviewRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &PreviewHandler{Limiter: NewIPRateLimiter(10, time.Minute)}
	r := newPreviewRouter(h)
	for i := 0; i < 10; i++ {
		rec := postPreview(t, r, `{"url":"not a url"}`, "198.51.100.10:1234")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid url") {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	limited := postPreview(t, r, `{"url":"http://preview.example/"}`, "198.51.100.10:1234")
	if limited.Code != http.StatusTooManyRequests || !strings.Contains(limited.Body.String(), "rate limit exceeded") {
		t.Fatalf("limit: %d %s", limited.Code, limited.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/preview", strings.NewReader(`{"url":"not a url"}`))
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	spoofed := httptest.NewRecorder()
	r.ServeHTTP(spoofed, req)
	if spoofed.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed forwarded-for opened a new bucket: %d %s", spoofed.Code, spoofed.Body.String())
	}
	other := postPreview(t, r, `{"url":"not a url"}`, "198.51.100.11:1234")
	if other.Code != http.StatusBadRequest {
		t.Fatalf("other ip: %d %s", other.Code, other.Body.String())
	}
}

func TestPreviewErrorJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var dialed bool
	h := &PreviewHandler{
		Limiter: NewIPRateLimiter(10, time.Minute),
		Fetch: preview.Fetcher{
			LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
				if host == "inside.example" {
					return []net.IP{net.ParseIP("10.1.2.3")}, nil
				}
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			},
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				dialed = true
				return nil, io.ErrClosedPipe
			},
		},
	}
	r := newPreviewRouter(h)
	bad := postPreview(t, r, `{"url":"ftp://example.com/"}`, "198.51.100.20:1")
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), `"error":"invalid url"`) {
		t.Fatalf("bad url: %d %s", bad.Code, bad.Body.String())
	}
	blocked := postPreview(t, r, `{"url":"http://inside.example/admin"}`, "198.51.100.20:1")
	if blocked.Code != http.StatusForbidden || !strings.Contains(blocked.Body.String(), `"error":"blocked address"`) {
		t.Fatalf("blocked: %d %s", blocked.Code, blocked.Body.String())
	}
	if dialed {
		t.Fatal("blocked address was dialed")
	}
	failed := postPreview(t, r, `{"url":"http://preview.example/down"}`, "198.51.100.20:1")
	if failed.Code != http.StatusUnprocessableEntity || !strings.Contains(failed.Body.String(), `"error":"fetch failed"`) {
		t.Fatalf("fetch: %d %s", failed.Code, failed.Body.String())
	}
}

func TestPreviewUnreadableIsNotNoMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	htmlPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, `<html><head><title>Empty</title></head><body></body></html>`)
		case "/login":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, `<html><head><title>Sign in</title></head><body><main><p>Log in to see this post.</p></main></body></html>`)
		case "/forbidden":
			http.Error(w, "forbidden", http.StatusForbidden)
		case "/missing":
			http.NotFound(w, r)
		case "/article":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, `<html><head><title>Notes</title></head><body><main><p>`+strings.Repeat("word ", 90)+`login appears once in this long article.</p></main></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(htmlPage.Close)
	htmlHandler := &PreviewHandler{Limiter: NewIPRateLimiter(20, time.Minute), Fetch: pinnedPreviewFetcher(htmlPage, false)}
	htmlRouter := newPreviewRouter(htmlHandler)
	for _, path := range []string{"/empty", "/login", "/forbidden"} {
		rec := postPreview(t, htmlRouter, `{"url":"http://preview.example`+path+`"}`, "203.0.113.40:1")
		assertUnreadable(t, rec)
	}
	missing := postPreview(t, htmlRouter, `{"url":"http://preview.example/missing"}`, "203.0.113.40:1")
	if missing.Code != http.StatusUnprocessableEntity || !strings.Contains(missing.Body.String(), `"error":"fetch failed"`) {
		t.Fatalf("html 404: %d %s", missing.Code, missing.Body.String())
	}

	xFixture := readPreviewFixture(t)
	tlsPage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oembed" {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Query().Get("url"), "/status/404") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(xFixture)
	}))
	t.Cleanup(tlsPage.Close)
	tlsHandler := &PreviewHandler{Limiter: NewIPRateLimiter(20, time.Minute), Fetch: pinnedPreviewFetcher(tlsPage, true)}
	tlsRouter := newPreviewRouter(tlsHandler)
	private := postPreview(t, tlsRouter, `{"url":"https://twitter.com/jack/status/404"}`, "203.0.113.40:1")
	assertUnreadable(t, private)

	article := postPreview(t, htmlRouter, `{"url":"http://preview.example/article"}`, "203.0.113.40:2")
	if article.Code != http.StatusInternalServerError {
		t.Fatalf("long article that mentions login: %d %s", article.Code, article.Body.String())
	}
}

func TestPreviewLowConfidenceReturnsTopThree(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	noise := []string{"quilt", "marble", "cipher", "velvet", "orbit", "lantern", "cinder", "plume", "sable", "ivory"}
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "QuiltNone", append([]string{"quilt"}, noise...))
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "GardenLow", append([]string{"garden"}, noise...))
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "HerbMid", append([]string{"garden", "herb"}, noise...))
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "WaterMost", append([]string{"garden", "herb", "water"}, noise...))

	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><head><title>Tomato notes</title><meta property="og:title" content="OG Tomato notes"><meta property="og:description" content="A public note about garden beds."></head><body><main><p>Gardening tips for tomatoes in small gardens and watering herbs.</p></main></body></html>`)
	}))
	t.Cleanup(page.Close)
	h := &PreviewHandler{DB: db, Limiter: NewIPRateLimiter(10, time.Minute), Fetch: pinnedPreviewFetcher(page, false)}
	rec := postPreview(t, newPreviewRouter(h), `{"url":"http://preview.example/garden"}`, "203.0.113.41:1")
	if rec.Code != http.StatusOK {
		t.Fatalf("low confidence: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "no match") {
		t.Fatalf("low confidence said no match: %s", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["title"] != "OG Tomato notes" || body["low_confidence"] != true {
		t.Fatalf("body = %s", rec.Body.String())
	}
	offers, _ := body["offers"].([]any)
	if len(offers) != 3 {
		t.Fatalf("offers = %s", rec.Body.String())
	}
	want := []string{"WaterMost", "HerbMid", "GardenLow"}
	for i, name := range want {
		card, _ := offers[i].(map[string]any)
		if card["name"] != name || card["low_confidence"] != true {
			t.Fatalf("offer %d = %#v", i, card)
		}
	}
	top, _ := body["offer"].(map[string]any)
	if top["name"] != "WaterMost" {
		t.Fatalf("top = %#v", top)
	}
}

func TestPreviewXPostFixtureReturnsOffers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	for _, name := range []string{"QuiltNone", "MarbleKiln", "CipherLoom"} {
		insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", name, []string{"quilt", "marble", "cipher"})
	}
	fixture := readPreviewFixture(t)
	page := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "publish.twitter.com" {
			http.Redirect(w, r, "https://publish.x.com/oembed?"+r.URL.RawQuery, http.StatusMovedPermanently)
			return
		}
		if r.Host != "publish.x.com" || r.URL.Path != "/oembed" {
			http.NotFound(w, r)
			return
		}
		if !strings.Contains(r.URL.Query().Get("url"), "twitter.com/jack/status/20") && !strings.Contains(r.URL.Query().Get("url"), "x.com/jack/status/20") {
			t.Errorf("oembed url = %s", r.URL.Query().Get("url"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(page.Close)
	h := &PreviewHandler{DB: db, Limiter: NewIPRateLimiter(10, time.Minute), Fetch: pinnedPreviewFetcher(page, true)}
	r := newPreviewRouter(h)
	for _, raw := range []string{
		`{"url":"https://twitter.com/jack/status/20"}`,
		`{"url":"https://x.com/jack/status/20"}`,
	} {
		rec := postPreview(t, r, raw, "203.0.113.42:1")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", raw, rec.Code, rec.Body.String())
		}
		if strings.Contains(strings.ToLower(rec.Body.String()), "no match") {
			t.Fatalf("%s said no match: %s", raw, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		offers, _ := body["offers"].([]any)
		title, _ := body["title"].(string)
		if len(offers) < 3 || body["low_confidence"] != true || !strings.Contains(title, "just setting up my twttr") {
			t.Fatalf("%s body = %s", raw, rec.Body.String())
		}
	}
}

func TestPreviewYouTubeOEmbed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "QuiltNone", []string{"quilt"})
	page := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "www.youtube.com" || r.URL.Path != "/oembed" || r.URL.Query().Get("format") != "json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"title":"Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)","author_name":"Rick Astley","html":"<iframe src=\"https://www.youtube.com/embed/dQw4w9WgXcQ\"></iframe>"}`)
	}))
	t.Cleanup(page.Close)
	h := &PreviewHandler{DB: db, Limiter: NewIPRateLimiter(10, time.Minute), Fetch: pinnedPreviewFetcher(page, true)}
	rec := postPreview(t, newPreviewRouter(h), `{"url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}`, "203.0.113.43:1")
	if rec.Code != http.StatusOK {
		t.Fatalf("youtube: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["title"] != "Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)" || body["offer"] == nil {
		t.Fatalf("youtube body = %s", rec.Body.String())
	}
}

func TestPreviewLivePublicXPost(t *testing.T) {
	if os.Getenv("AFFILIO_LIVE_PREVIEW") != "1" {
		t.Skip("set AFFILIO_LIVE_PREVIEW=1 to fetch a real public X post")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	for _, name := range []string{"QuiltNone", "MarbleKiln", "CipherLoom"} {
		insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", name, []string{"quilt", "marble", "cipher"})
	}
	h := &PreviewHandler{DB: db, Limiter: NewIPRateLimiter(10, time.Minute), Fetch: preview.Fetcher{}}
	rec := postPreview(t, newPreviewRouter(h), `{"url":"https://twitter.com/jack/status/20"}`, "203.0.113.44:1")
	if rec.Code != http.StatusOK {
		t.Fatalf("live: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "no match") {
		t.Fatalf("live said no match: %s", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["result"] == "unreadable_or_private" {
		t.Fatalf("public post was unreadable: %s", rec.Body.String())
	}
	offers, _ := body["offers"].([]any)
	title, _ := body["title"].(string)
	if len(offers) < 3 || body["low_confidence"] != true || !strings.Contains(title, "just setting up my twttr") {
		t.Fatalf("live body = %s", rec.Body.String())
	}
}

func assertUnreadable(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "no match") {
		t.Fatalf("unreadable said no match: %s", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["result"] != "unreadable_or_private" || body["message"] != "only public links work" || body["offer"] != nil || body["title"] != nil {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func readPreviewFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("../preview/testdata/x_post_oembed.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func pinnedPreviewFetcher(page *httptest.Server, tlsServer bool) preview.Fetcher {
	f := preview.Fetcher{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
				return nil, fmt.Errorf("dialed blocked %s", address)
			}
			return (&net.Dialer{}).DialContext(ctx, network, page.Listener.Addr().String())
		},
	}
	if tlsServer {
		f.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return f
}

func newPreviewRouter(h *PreviewHandler) *gin.Engine {
	r := gin.New()
	if err := middleware.ConfigureTrustedProxies(r, ""); err != nil {
		panic(err)
	}
	r.POST("/api/preview", h.Preview)
	return r
}

func postPreview(t *testing.T, r *gin.Engine, body, remote string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/preview", strings.NewReader(body))
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
