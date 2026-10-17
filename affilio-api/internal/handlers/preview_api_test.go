package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(matched.Body.String(), "InboxPilot") || strings.Contains(matched.Body.String(), "secret()") {
		t.Fatalf("body leaked page script or dropped the offer: %s", matched.Body.String())
	}

	miss := postPreview(t, r, `{"url":"http://preview.example/garden"}`, "203.0.113.20:4000")
	if miss.Code != http.StatusOK {
		t.Fatalf("no match: %d %s", miss.Code, miss.Body.String())
	}
	var empty map[string]any
	if err := json.Unmarshal(miss.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty["title"] != "Tomato notes" || empty["offer"] != nil || empty["slot"] != nil {
		t.Fatalf("no match body = %s", miss.Body.String())
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
