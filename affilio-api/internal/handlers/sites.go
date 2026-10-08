package handlers

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/jackc/pgx/v5"

	"github.com/gin-gonic/gin"
)

type SitesHandler struct {
	DB              *database.DB
	AdminAPIKey     string
	RegisterLimiter *IPRateLimiter
	Audit           *services.AuditLogger
}

type siteRegisterRequest struct {
	Domain          string `json:"domain"`
	Name            string `json:"name"`
	InstallationKey string `json:"installation_key"`
	SiteURL         string `json:"site_url"`
	PluginVersion   string `json:"plugin_version"`
}

type siteDeleteRequest struct {
	SiteID  string `json:"site_id"`
	Domain  string `json:"domain"`
	Confirm string `json:"confirm"`
}

type rateLimitEntry struct {
	WindowStart time.Time
	Count       int
}

type IPRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateLimitEntry
	nowFn   func() time.Time
}

var domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		limit:   limit,
		window:  window,
		entries: map[string]rateLimitEntry{},
		nowFn:   time.Now,
	}
}

func (l *IPRateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true
	}
	now := l.nowFn()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.WindowStart) >= l.window {
		l.entries[key] = rateLimitEntry{WindowStart: now, Count: 1}
		return true
	}
	if entry.Count >= l.limit {
		return false
	}
	entry.Count++
	l.entries[key] = entry
	return true
}

func (h *SitesHandler) Register(c *gin.Context) {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	if h.RegisterLimiter != nil && !h.RegisterLimiter.Allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var req siteRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.PluginVersion = strings.TrimSpace(req.PluginVersion)
	storedName := req.Name
	if siteURL := strings.TrimSpace(req.SiteURL); siteURL != "" {
		derived, ok := domainFromSiteURL(siteURL)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid domain is required"})
			return
		}
		if strings.TrimSpace(req.Domain) != "" && normalizeDomain(req.Domain) != derived {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid domain is required"})
			return
		}
		req.Domain = derived
		storedName = ""
	} else {
		req.Domain = normalizeDomain(req.Domain)
	}
	if req.Domain == "" || !validDomain(req.Domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid domain is required"})
		return
	}
	if len(storedName) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name too long"})
		return
	}
	apiKey := req.InstallationKey
	var err error
	if apiKey != "" {
		decoded, decodeErr := hex.DecodeString(apiKey)
		if decodeErr != nil || len(decoded) != 32 || apiKey != strings.ToLower(apiKey) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "installation_key must be 32 random bytes encoded as lowercase hex"})
			return
		}
	} else {
		apiKey, err = generateAPIKey()
	}
	if err != nil {
		log.Printf("sites.register key generation failed domain=%s err=%v", req.Domain, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	keyHash := services.HashAPIKey(apiKey)
	keyPrefix := services.KeyPrefix(apiKey)
	var siteID string
	if req.InstallationKey != "" {
		siteID, err = h.DB.RegisterInstallation(c.Request.Context(), keyHash, keyPrefix, req.Domain, storedName)
	} else {
		siteID, err = h.DB.CreateSite(c.Request.Context(), keyHash, keyPrefix, req.Domain, storedName)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, gin.H{"error": "installation is already connected to a different domain"})
		return
	}
	if err != nil {
		log.Printf("sites.register db create failed domain=%s name=%s err=%v", req.Domain, storedName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register site"})
		return
	}
	if err := h.DB.SaveSitePluginVersion(c.Request.Context(), siteID, req.PluginVersion); err != nil {
		log.Printf("sites.register plugin version save failed domain=%s err=%v", req.Domain, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register site"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_key": apiKey, "site_id": siteID})
}

func (h *SitesHandler) Delete(c *gin.Context) {
	if strings.TrimSpace(h.AdminAPIKey) == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin key not configured"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Admin-Key")), []byte(h.AdminAPIKey)) != 1 {
		if h.Audit != nil {
			h.Audit.Log(database.AuditEvent{
				EventType: "auth_failure_admin_key",
				Severity:  "warn",
				IPAddress: strings.TrimSpace(c.ClientIP()),
				Endpoint:  "/v1/sites",
				Method:    "DELETE",
				Details:   map[string]interface{}{"reason": "invalid admin key on site delete"},
			})
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid admin key"})
		return
	}
	var req siteDeleteRequest
	siteID := strings.TrimSpace(c.Query("site_id"))
	if siteID == "" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		siteID = strings.TrimSpace(req.SiteID)
	} else {
		req.Confirm = strings.TrimSpace(c.Query("confirm"))
		req.Domain = normalizeDomain(c.Query("domain"))
	}
	if req.Confirm != "delete" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required"})
		return
	}
	if siteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id is required"})
		return
	}
	site, err := h.DB.GetSiteByID(c.Request.Context(), siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	if req.Domain != "" && normalizeDomain(site.Domain) != normalizeDomain(req.Domain) {
		c.JSON(http.StatusConflict, gin.H{"error": "domain confirmation mismatch"})
		return
	}
	deleted, err := h.DB.DeleteSiteByID(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if h.Audit != nil {
		h.Audit.Log(database.AuditEvent{
			EventType: "site_deleted",
			Severity:  "info",
			IPAddress: strings.TrimSpace(c.ClientIP()),
			Endpoint:  "/v1/sites",
			Method:    "DELETE",
			Details:   map[string]interface{}{"site_id": siteID, "domain": site.Domain},
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "site_id": siteID})
}

func domainFromSiteURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") || parsed.Hostname() == "" {
		return "", false
	}
	domain := normalizeDomain(parsed.Hostname())
	if domain == "" || !validDomain(domain) {
		return "", false
	}
	return domain, true
}

func normalizeDomain(domain string) string {
	raw := strings.TrimSpace(strings.ToLower(domain))
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if parsed, err := url.Parse(raw); err == nil {
			raw = strings.TrimSpace(strings.ToLower(parsed.Hostname()))
		}
	}
	return strings.Trim(raw, ".")
}

func validDomain(domain string) bool {
	domain = normalizeDomain(domain)
	if domain == "localhost" {
		return true
	}
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}
	if strings.Contains(domain, "..") {
		return false
	}
	return domainPattern.MatchString(domain)
}
