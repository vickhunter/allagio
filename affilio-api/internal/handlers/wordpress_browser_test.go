package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"affilio-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestWordPressDistributionBrowser(t *testing.T) {
	qa := os.Getenv("AFFILIO_WORDPRESS_QA")
	if qa == "" {
		t.Skip("set AFFILIO_WORDPRESS_QA to an isolated WordPress QA directory")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	r := gin.New()
	server := httptest.NewUnstartedServer(r)
	server.Listener.Close()
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Listener = listener
	base := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	h := &HostedHandler{DB: db, BaseURL: base, LoopbackURL: base, AssetsPath: "../../../affilio-wp/admin/build", Login: &fakeOwnerLogin{}}
	h.DownloadsPath = os.Getenv("AFFILIO_WORKSPACE_DOWNLOADS")
	if err = h.LoadWorkspace(); err != nil {
		t.Fatal(err)
	}
	h.Mount(r)
	r.GET("/v1/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "site_installation_registration": true}) })
	r.POST("/v1/sites/register", (&SitesHandler{DB: db}).Register)
	r.GET("/v1/go/:code", (&RedirectsHandler{DB: db}).Go)
	p := &PlacementsHandler{DB: db, RequireVerifiedConnection: true}
	site := r.Group("/v1", middleware.APIKeyAuth(db, nil, h.resource()))
	site.GET("/articles", (&ArticlesHandler{DB: db}).List)
	site.GET("/sites/owner-connection", h.SiteConnectionStatus)
	site.POST("/sites/owner-connection", h.StartSiteConnection)
	site.POST("/revenue/opportunities", p.Find)
	site.GET("/placements", p.List)
	site.GET("/placements/:id", p.Get)
	site.POST("/placements", p.Prepare)
	site.POST("/placements/:id/commit", p.Commit)
	site.POST("/placements/:id/receipt", p.RecordReceipt)
	server.Start()
	defer server.Close()
	_, cookie := testOwnerSession(t, h, "wordpress-owner", "wordpress-owner@example.com")
	_, key := createTestMaker(t, db)
	listedOffer(t, db, newRevenueRouter(db), key)
	if _, err = db.Pool.Exec(context.Background(), `UPDATE product_connections SET configuration_verified_at=NOW()`); err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(map[string]any{"base": base, "owner": cookie, "qa": qa, "download": h.DownloadsPath != "", "screenshots": os.Getenv("AFFILIO_WORKSPACE_SCREENSHOTS")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join("../../../affilio-wp/e2e", "distribution.mjs"))
	cmd.Stdin = bytes.NewReader(fixture)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("WordPress browser integration: %v\n%s", err, output)
	}
	var recorded int
	if err = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM placement_proposals WHERE status='committed' AND receipt->>'publication_status'='publisher_reported'`).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("publication receipt missing: %d %v", recorded, err)
	}
	if err = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks c JOIN placement_proposals p ON p.id=c.placement_id WHERE p.status='committed' AND c.campaign_id=p.campaign_id`).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("published recommendation click missing: %d %v", recorded, err)
	}
	t.Log(string(output))
}
