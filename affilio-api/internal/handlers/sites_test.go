package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func TestSitesInstallationRegistrationRetries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	r := gin.New()
	r.POST("/sites/register", (&SitesHandler{DB: db}).Register)
	key := strings.Repeat("ab", 32)
	register := func(domain, installationKey string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("POST", "/sites/register", newJSONRequest(t, "POST", "/sites/register", map[string]any{"domain": domain, "installation_key": installationKey})))
		return rec
	}
	first, retry := register("blog.example", key), register("blog.example", key)
	if first.Code != 200 || retry.Code != 200 {
		t.Fatalf("registration failed: %d %d", first.Code, retry.Code)
	}
	var one, two struct {
		SiteID string `json:"site_id"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(retry.Body.Bytes(), &two); err != nil {
		t.Fatal(err)
	}
	if one.SiteID != two.SiteID || two.APIKey != key {
		t.Fatal("retry created another site or changed its credential")
	}
	site, err := db.GetSiteByID(context.Background(), one.SiteID)
	if err != nil || site.APIKeyHash != services.HashAPIKey(key) {
		t.Fatal("installation key was not hashed")
	}
	if rec := register("other.example", key); rec.Code != 409 {
		t.Fatalf("domain change accepted: %d", rec.Code)
	}
	if rec := register("blog.example", "weak-key"); rec.Code != 400 {
		t.Fatalf("invalid installation key accepted: %d", rec.Code)
	}
}

func TestSitesRegisterRateLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	handler := &SitesHandler{
		DB:              db,
		RegisterLimiter: NewIPRateLimiter(1, time.Minute),
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sites/register", handler.Register)

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/sites/register", newJSONRequest(t, http.MethodPost, "/sites/register", map[string]interface{}{
		"domain": "example.com",
		"name":   "Example",
	}))
	req1.RemoteAddr = "198.51.100.10:12345"
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/sites/register", newJSONRequest(t, http.MethodPost, "/sites/register", map[string]interface{}{
		"domain": "example.org",
		"name":   "Example Org",
	}))
	req2.RemoteAddr = "198.51.100.10:12345"
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}
}

func TestSitesDeleteRequiresAdminKeyAndSiteID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := createTestSite(t, db, "site-key", "example.com", "Example")

	handler := &SitesHandler{
		DB:          db,
		AdminAPIKey: "admin-secret",
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/sites", handler.Delete)

	unauthRec := httptest.NewRecorder()
	unauthReq := httptest.NewRequest(http.MethodDelete, "/sites", newJSONRequest(t, http.MethodDelete, "/sites", map[string]interface{}{
		"site_id": siteID,
		"confirm": "delete",
	}))
	r.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing admin key, got %d", unauthRec.Code)
	}

	badConfirmRec := httptest.NewRecorder()
	badConfirmReq := httptest.NewRequest(http.MethodDelete, "/sites", newJSONRequest(t, http.MethodDelete, "/sites", map[string]interface{}{
		"site_id": siteID,
		"confirm": "nope",
	}))
	badConfirmReq.Header.Set("X-Admin-Key", "admin-secret")
	r.ServeHTTP(badConfirmRec, badConfirmReq)
	if badConfirmRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid confirm value, got %d", badConfirmRec.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/sites", newJSONRequest(t, http.MethodDelete, "/sites", map[string]interface{}{
		"site_id": siteID,
		"domain":  "example.com",
		"confirm": "delete",
	}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if _, err := db.GetSiteByID(context.Background(), siteID); err == nil {
		t.Fatalf("expected site to be deleted")
	}
}

func TestSitesDeleteRejectsDomainMismatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := createTestSite(t, db, "site-key-2", "example.net", "Example Net")

	handler := &SitesHandler{
		DB:          db,
		AdminAPIKey: "admin-secret",
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/sites", handler.Delete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/sites", newJSONRequest(t, http.MethodDelete, "/sites", map[string]interface{}{
		"site_id": siteID,
		"domain":  "other.net",
		"confirm": "delete",
	}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for domain mismatch, got %d", rec.Code)
	}
}

func TestSitesRegisterValidatesDomain(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	handler := &SitesHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sites/register", handler.Register)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites/register", newJSONRequest(t, http.MethodPost, "/sites/register", map[string]interface{}{
		"domain": "not a domain",
		"name":   "Broken",
	}))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid domain, got %d", rec.Code)
	}
}
