package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func TestPlacementRemoval(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-removal"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	otherKey := "site-key-other"
	createTestSite(t, db, otherKey, "other.example.com", "Other")
	linkPublisherOwner(t, db, siteID)
	r := removalRouter(db)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": contextText,
		"copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %s", rec.Code, rec.Body.String())
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact placement.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	rec, committed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", rec.Code, rec.Body.String())
	}
	receipt := committed["placement"].(map[string]interface{})["receipt"].(map[string]interface{})
	tracked, _ := receipt["tracked_link"].(string)
	parsed, err := url.Parse(tracked)
	if err != nil || parsed.Path == "" {
		t.Fatalf("tracked link = %q", tracked)
	}
	code := strings.TrimPrefix(parsed.Path, "/v1/go/")

	t.Run("auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/placements/"+placementID+"/removed", nil)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("missing key: %d %s", res.Code, res.Body.String())
		}
	})
	t.Run("wrong site", func(t *testing.T) {
		rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/removed", otherKey, map[string]interface{}{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("wrong site: %d %v", rec.Code, body)
		}
	})

	before := placementClickCount(t, db, placementID)
	goReq := httptest.NewRequest(http.MethodGet, tracked, nil)
	goReq.Header.Set("User-Agent", "Mozilla/5.0")
	goRes := httptest.NewRecorder()
	r.ServeHTTP(goRes, goReq)
	if goRes.Code != http.StatusFound || !strings.Contains(goRes.Header().Get("Location"), "via=") {
		t.Fatalf("live go link: %d %s", goRes.Code, goRes.Header().Get("Location"))
	}
	if placementClickCount(t, db, placementID) != before+1 {
		t.Fatal("a live placement did not record a click")
	}

	rec, removed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/removed", siteKey, map[string]interface{}{})
	if rec.Code != http.StatusOK || removed["already_removed"] != false || removed["status"] != "removed" {
		t.Fatalf("remove: %d %v", rec.Code, removed)
	}
	rec, again := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/removed", siteKey, map[string]interface{}{})
	if rec.Code != http.StatusOK || again["already_removed"] != true || again["removed_at"] != removed["removed_at"] {
		t.Fatalf("idempotent remove: %d %v", rec.Code, again)
	}

	after := placementClickCount(t, db, placementID)
	stale := httptest.NewRequest(http.MethodGet, "/v1/go/"+code+"?placement="+placementID, nil)
	stale.Header.Set("User-Agent", "Mozilla/5.0")
	staleRes := httptest.NewRecorder()
	r.ServeHTTP(staleRes, stale)
	if staleRes.Code != http.StatusFound {
		t.Fatalf("removed go link: %d %s", staleRes.Code, staleRes.Body.String())
	}
	requireGoLinkHeaders(t, staleRes)
	location := staleRes.Header().Get("Location")
	if location != "https://inboxpilot.example.com" || strings.Contains(location, "via=") {
		t.Fatalf("removed redirect = %q", location)
	}
	if placementClickCount(t, db, placementID) != after {
		t.Fatal("a removed placement recorded a click")
	}

	removedAt := again["removed_at"].(string)
	date := removedAt[:10]
	ownerHTML := dashboardHTML(t, db, &database.Owner{ID: siteID, Email: "publisher@example.com", Name: "Publisher", SiteID: siteID, MakerID: makerID, CSRF: "csrf"}, "/owner/placements/"+placementID)
	if !strings.Contains(ownerHTML, "Rimosso "+date) {
		t.Fatalf("owner dashboard missing removal label:\n%s", ownerHTML)
	}
	makerHTML := dashboardHTML(t, db, &database.Owner{ID: siteID, Email: "maker@example.com", Name: "Maker", MakerID: makerID, CSRF: "csrf"}, "/owner")
	if !strings.Contains(makerHTML, "Publisher placements") || !strings.Contains(makerHTML, "Rimosso "+date) {
		t.Fatalf("maker dashboard missing removal label:\n%s", makerHTML)
	}
}

func TestRemovedAtMigrationKeepsExistingPlacements(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email"})
	siteKey := "site-key-migration"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := removalRouter(db)
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": "InboxPilot email inbox productivity tutorial",
		"copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %s", rec.Code, rec.Body.String())
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE placement_proposals DROP COLUMN removed_at`); err != nil {
		t.Fatal(err)
	}
	var stillThere int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals WHERE id=$1`, placementID).Scan(&stillThere); err != nil || stillThere != 1 {
		t.Fatalf("existing placement disappeared before the migration: %d %v", stillThere, err)
	}
	sqlBytes, err := os.ReadFile(filepath.Join("..", "migrations", "016_placement_removed_at.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatal("repeat migration failed", err)
	}
	var nullable bool
	if err = db.Pool.QueryRow(ctx, `SELECT removed_at IS NULL FROM placement_proposals WHERE id=$1`, placementID).Scan(&nullable); err != nil || !nullable {
		t.Fatalf("existing placement removed_at = null? %v err=%v", nullable, err)
	}
	var dataType, isNullable string
	if err = db.Pool.QueryRow(ctx, `SELECT data_type, is_nullable FROM information_schema.columns WHERE table_name='placement_proposals' AND column_name='removed_at'`).Scan(&dataType, &isNullable); err != nil {
		t.Fatal(err)
	}
	if dataType != "timestamp with time zone" || isNullable != "YES" {
		t.Fatalf("column type %s nullable %s", dataType, isNullable)
	}
}

func removalRouter(db *database.DB) *gin.Engine {
	r := publisherLegRouter(db, true)
	r.POST("/v1/placements/:id/removed", middleware.APIKeyAuth(db, nil), (&PlacementsHandler{DB: db}).MarkRemoved)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	return r
}

func placementClickCount(t *testing.T, db *database.DB, placementID string) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, placementID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func dashboardHTML(t *testing.T, db *database.DB, owner *database.Owner, path string) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if id := strings.TrimPrefix(path, "/owner/placements/"); id != path {
		c.Params = gin.Params{{Key: "id", Value: id}}
		c.Set("owner", owner)
		(&HostedHandler{DB: db, BaseURL: "https://distribution.example"}).ReviewPlacement(c)
		return w.Body.String()
	}
	c.Set("owner", owner)
	(&HostedHandler{DB: db, BaseURL: "https://distribution.example"}).Dashboard(c)
	return w.Body.String()
}
