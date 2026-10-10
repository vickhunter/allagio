package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func pluginRegisterFixture(t *testing.T) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the register contract test")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "affilio-wp", "tests", "fixtures", "sites-register.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("plugin register fixture: %v", err)
	}
	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if body["site_url"] == "" || body["plugin_version"] == "" || len(body) != 2 {
		t.Fatalf("plugin register fixture must be site_url and plugin_version only: %#v", body)
	}
	return payload
}

func registerRaw(t *testing.T, r http.Handler, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites/register", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func TestSitesRegisterAcceptsPluginContract(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sites/register", (&SitesHandler{DB: db}).Register)

	rec := registerRaw(t, r, pluginRegisterFixture(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("plugin payload rejected: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created["site_id"] == "" || created["api_key"] == "" {
		t.Fatalf("plugin registration response: %s", rec.Body.String())
	}
	var domain, name, row string
	if err := db.Pool.QueryRow(context.Background(), `SELECT domain, COALESCE(name,''), row_to_json(sites)::text FROM sites WHERE id=$1`, created["site_id"]).Scan(&domain, &name, &row); err != nil {
		t.Fatal(err)
	}
	if domain != "blog.example" || name != "" {
		t.Fatalf("stored domain %q name %q", domain, name)
	}
	if strings.Contains(row, "0.1.19") {
		t.Fatalf("plugin_version was stored without a column: %s", row)
	}

	extra := []byte(`{"site_url":"https://Blog.Example/welcome","plugin_version":"0.1.10","email":"owner@example.com","name":"Ada Admin Blog","owner":"Ada"}`)
	extraRec := registerRaw(t, r, extra)
	if extraRec.Code != http.StatusOK {
		t.Fatalf("extra fields rejected: %d %s", extraRec.Code, extraRec.Body.String())
	}
	var extraCreated map[string]string
	if err := json.Unmarshal(extraRec.Body.Bytes(), &extraCreated); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT domain, COALESCE(name,''), row_to_json(sites)::text FROM sites WHERE id=$1`, extraCreated["site_id"]).Scan(&domain, &name, &row); err != nil {
		t.Fatal(err)
	}
	if domain != "blog.example" || name != "" || strings.Contains(row, "owner@example.com") || strings.Contains(row, "Ada") {
		t.Fatalf("extra register fields were persisted: domain=%q name=%q row=%s", domain, name, row)
	}

	wwwRec := registerRaw(t, r, []byte(`{"site_url":"https://www.blog.example/path"}`))
	if wwwRec.Code != http.StatusOK {
		t.Fatalf("www host rejected: %d %s", wwwRec.Code, wwwRec.Body.String())
	}
	var wwwCreated map[string]string
	if err := json.Unmarshal(wwwRec.Body.Bytes(), &wwwCreated); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT domain FROM sites WHERE id=$1`, wwwCreated["site_id"]).Scan(&domain); err != nil {
		t.Fatal(err)
	}
	if domain != "www.blog.example" {
		t.Fatalf("www host was rewritten to %q", domain)
	}

	for _, payload := range []string{
		`{"site_url":"ftp://blog.example"}`,
		`{"site_url":"https://"}`,
		`{"site_url":"blog.example"}`,
		`{"site_url":"http:///missing"}`,
		`{"site_url":"https://blog.example","domain":"other.example"}`,
	} {
		bad := registerRaw(t, r, []byte(payload))
		if bad.Code != http.StatusBadRequest {
			t.Fatalf("accepted %s: %d %s", payload, bad.Code, bad.Body.String())
		}
	}

	legacy := registerRaw(t, r, []byte(`{"domain":"Blog.Example","name":"Legacy Blog"}`))
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy domain body rejected: %d %s", legacy.Code, legacy.Body.String())
	}
	var legacyCreated map[string]string
	if err := json.Unmarshal(legacy.Body.Bytes(), &legacyCreated); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT domain, COALESCE(name,'') FROM sites WHERE id=$1`, legacyCreated["site_id"]).Scan(&domain, &name); err != nil {
		t.Fatal(err)
	}
	if domain != "blog.example" || name != "Legacy Blog" {
		t.Fatalf("legacy body stored domain %q name %q", domain, name)
	}
}

func TestSitesRegisterStoresPluginVersionOnlyWhenColumnExists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sites/register", (&SitesHandler{DB: db}).Register)

	var columns int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sites' AND column_name = 'plugin_version'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("sites.plugin_version already exists; this PR must not add it")
	}
	if _, err := db.Pool.Exec(context.Background(), `ALTER TABLE sites ADD COLUMN plugin_version VARCHAR(32)`); err != nil {
		t.Fatal(err)
	}
	rec := registerRaw(t, r, pluginRegisterFixture(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("plugin payload rejected after column add: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(plugin_version,'') FROM sites WHERE id=$1`, created["site_id"]).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "0.1.19" {
		t.Fatalf("plugin_version column was not stored: %q", version)
	}
}
