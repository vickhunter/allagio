package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

func TestClaimsCheckValue(t *testing.T) {
	cases := []struct {
		mode, status, reason, want string
	}{
		{"stub", "listed", "", "not_checked"},
		{"stub", "submitted", "claims", "not_checked"},
		{"STUB", "draft", "", "not_checked"},
		{"off-local", "draft", "", "not_checked"},
		{"off-local", "", "", "not_checked"},
		{"off-local", "submitted", "claims", "failed"},
		{"real", "submitted", "claims", "failed"},
		{"off-local", "listed", "", "passed"},
		{"off-local", "submitted", "head", "passed"},
		{"", "listed", "", "passed"},
	}
	for _, tc := range cases {
		if got := claimsCheckValue(tc.mode, tc.status, tc.reason); got != tc.want {
			t.Fatalf("%s/%s/%s: %s want %s", tc.mode, tc.status, tc.reason, got, tc.want)
		}
	}
}

func TestDigestTruncationNotice(t *testing.T) {
	previous := database.DigestPageSize
	database.DigestPageSize = 1
	t.Cleanup(func() { database.DigestPageSize = previous })

	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, _, r := newEntryTest(t, db)
	owner, cookie := testOwnerSession(t, h, "trunc-owner", "trunc-owner@example.com")
	siteID := createTestSite(t, db, "trunc-site-key", "trunc.example", "Trunc")
	_, offerID := seedListedOffer(t, db, "trunc-maker", "trunc-maker@example.com", "Trunc Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, owner.ID, siteID); err != nil {
		t.Fatal(err)
	}
	for i, surface := range []string{"https://trunc.example/one", "https://trunc.example/two"} {
		id := insertPlacement(t, db, siteID, offerID, surface, "Trunc Product is worth a look.")
		markPlacementLive(t, db, siteID, offerID, id, "trunc-go-"+string(rune('a'+i)), surface)
	}
	rec, body := serveOwner(t, r, h, cookie, owner.CSRF, http.MethodGet, "/v1/digest", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", rec.Code, rec.Body.String())
	}
	items, _ := body["items"].([]any)
	total, _ := body["total"].(float64)
	if body["truncated"] != true || int(total) < 2 || len(items) != 1 {
		t.Fatalf("truncation: %s", rec.Body.String())
	}
}

func TestClickClassStoredPaidAndCapped(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "class-site-key", "class.example", "Class")
	_, offerID := seedListedOffer(t, db, "class-maker-key", "class-maker@example.com", "Class Product")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://offer.example/buy', status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	id := insertPlacement(t, db, siteID, offerID, "https://class.example/the-post", "Class Product is worth a look.")
	markPlacementLive(t, db, siteID, offerID, id, "class-go", "https://class.example/the-post")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	redirects := &RedirectsHandler{DB: db, IPSaltSecret: "class-salt", RateLimiter: NewIPRateLimiter(8, time.Hour)}
	r.GET("/v1/go/:code", redirects.Go)
	r.HEAD("/v1/go/:code", redirects.Go)

	hit := func(method, ip, ua string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/v1/go/"+id, nil)
		req.RemoteAddr = ip + ":1000"
		req.Header.Set("User-Agent", ua)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		requireGoLinkHeaders(t, rec)
		return rec
	}
	if loc := hit(http.MethodGet, "203.0.113.10", "Mozilla/5.0 reader", nil).Header().Get("Location"); !strings.Contains(loc, "via=") {
		t.Fatalf("browser: %s", loc)
	}
	if loc := hit(http.MethodGet, "203.0.113.11", "Mozilla/5.0 (compatible; GPTBot/1.2)", nil).Header().Get("Location"); !strings.Contains(loc, "via=") {
		t.Fatalf("verified agent: %s", loc)
	}
	if loc := hit(http.MethodGet, "203.0.113.12", "Mozilla/5.0 reader", map[string]string{"X-Affilio-Agent": "1"}).Header().Get("Location"); !strings.Contains(loc, "via=") {
		t.Fatalf("declared agent: %s", loc)
	}
	for _, call := range []struct {
		method, ip, ua string
		headers        map[string]string
	}{
		{http.MethodGet, "203.0.113.13", "Mozilla/5.0 (compatible; Microsoft SafeLinks)", nil},
		{http.MethodHead, "203.0.113.14", "Mozilla/5.0 reader", nil},
		{http.MethodGet, "203.0.113.15", "Mozilla/5.0 reader", map[string]string{"Purpose": "prefetch"}},
	} {
		if loc := hit(call.method, call.ip, call.ua, call.headers).Header().Get("Location"); strings.Contains(loc, "via=") {
			t.Fatalf("%s %s carried a ref: %s", call.method, call.ua, loc)
		}
	}

	var browserN, verifiedN, declaredN, unknownN int
	if err := db.Pool.QueryRow(context.Background(), `SELECT
COUNT(*) FILTER (WHERE click_class='browser'),
COUNT(*) FILTER (WHERE click_class='verified_agent'),
COUNT(*) FILTER (WHERE click_class='declared_agent'),
COUNT(*) FILTER (WHERE click_class='unknown')
FROM clicks WHERE placement_id=$1`, id).Scan(&browserN, &verifiedN, &declaredN, &unknownN); err != nil {
		t.Fatal(err)
	}
	if browserN != 1 || verifiedN != 1 || declaredN != 1 || unknownN != 2 {
		t.Fatalf("classes browser=%d verified=%d declared=%d unknown=%d", browserN, verifiedN, declaredN, unknownN)
	}
	summary, err := db.GetClickSummary(context.Background(), siteID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PaidClicks != 1 || summary.BrowserClicks != 1 || summary.VerifiedAgentClicks != 1 || summary.DeclaredAgentClicks != 1 || summary.UnknownClicks != 2 || summary.TotalClicks != 5 {
		t.Fatalf("summary %+v", summary)
	}

	track := gin.New()
	handler := &TrackingHandler{DB: db}
	track.GET("/track/summary", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.Summary(c)
	})
	summaryRec := httptest.NewRecorder()
	track.ServeHTTP(summaryRec, httptest.NewRequest(http.MethodGet, "/track/summary", nil))
	if summaryRec.Code != http.StatusOK || !strings.Contains(summaryRec.Body.String(), `"paid_clicks":1`) || !strings.Contains(summaryRec.Body.String(), `"verified_agent_clicks":1`) {
		t.Fatalf("report: %d %s", summaryRec.Code, summaryRec.Body.String())
	}

	var clickID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM clicks WHERE placement_id=$1 AND click_class='verified_agent'`, id).Scan(&clickID); err != nil {
		t.Fatal(err)
	}
	sale, err := db.RecordSaleConversion(context.Background(), clickID, "manual", "evt-agent-sale", "obj-agent-sale", "sale", 20, "USD", time.Now().Add(2*time.Minute))
	if err != nil || sale == nil || sale.Commission != 6 || sale.ConversionID == "" {
		t.Fatalf("agent sale %+v %v", sale, err)
	}

	cappedID := insertPlacement(t, db, siteID, offerID, "https://class.example/capped", "Class Product is worth a look.")
	markPlacementLive(t, db, siteID, offerID, cappedID, "class-cap", "https://class.example/capped")
	limited := gin.New()
	if err := limited.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	cap := &RedirectsHandler{DB: db, IPSaltSecret: "class-salt", RateLimiter: NewIPRateLimiter(1, time.Hour)}
	limited.GET("/v1/go/:code", cap.Go)
	open := func(ip string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/go/"+cappedID, nil)
		req.RemoteAddr = ip + ":1000"
		req.Header.Set("User-Agent", "Mozilla/5.0 reader")
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, req)
		return rec
	}
	first := open("203.0.113.20")
	second := open("203.0.113.21")
	if !strings.Contains(first.Header().Get("Location"), "via=") || strings.Contains(second.Header().Get("Location"), "via=") {
		t.Fatalf("cap first %s second %s", first.Header().Get("Location"), second.Header().Get("Location"))
	}
	var cappedN int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, cappedID).Scan(&cappedN); err != nil || cappedN != 1 {
		t.Fatalf("placement cap recorded %d (%v)", cappedN, err)
	}
}
