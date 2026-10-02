package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestWorkspaceBrowser(t *testing.T) {
	if os.Getenv("AFFILIO_WORKSPACE_BROWSER") != "1" {
		t.Skip("set AFFILIO_WORKSPACE_BROWSER=1 to run browser integration")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	r := gin.New()
	server := httptest.NewUnstartedServer(r)
	base := "http://" + server.Listener.Addr().String()
	assets := os.Getenv("AFFILIO_WORKSPACE_ASSETS")
	if assets == "" {
		assets = "../../../affilio-wp/admin/build"
	}
	h := &HostedHandler{DB: db, BaseURL: base, LoopbackURL: base, AssetsPath: assets, DownloadsPath: os.Getenv("AFFILIO_WORKSPACE_DOWNLOADS"), Login: &fakeOwnerLogin{}}
	if err := h.LoadWorkspace(); err != nil {
		t.Fatal(err)
	}
	h.Mount(r)
	server.Start()
	defer server.Close()
	builder, builderCookie := linkedMaker(t, h, "browser-builder")
	publisher, publisherCookie := testOwnerSession(t, h, "browser-publisher", "publisher@example.com")
	if err := db.CreateOwnerAccount(context.Background(), publisher, "publisher", "publisher.example.com", "browser-publisher-key", "browser"); err != nil {
		t.Fatal(err)
	}
	h.OperatorID = builder.ID
	_, key := createTestMaker(t, db)
	listedOffer(t, db, newRevenueRouter(db), key)
	if _, err := db.Pool.Exec(context.Background(), `UPDATE product_connections SET configuration_verified_at=NOW()`); err != nil {
		t.Fatal(err)
	}
	var payoutID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO payouts(site_id,maker_id,amount,currency,method,status,payment_mode) SELECT site_id,$2,10,'USD','manual','pending','live' FROM owners WHERE id=$1 RETURNING id`, publisher.ID, builder.MakerID).Scan(&payoutID); err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(map[string]any{"base": base, "builder": builderCookie, "publisher": publisherCookie, "payout": payoutID, "screenshots": os.Getenv("AFFILIO_WORKSPACE_SCREENSHOTS")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join("../../../affilio-wp/e2e", "workspace.mjs"))
	cmd.Stdin = bytes.NewReader(fixture)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser integration: %v\n%s", err, output)
	}
	t.Log(string(output))
}
