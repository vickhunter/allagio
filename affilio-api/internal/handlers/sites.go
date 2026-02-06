package handlers

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"affilio-api/internal/database"
	"github.com/jackc/pgx/v5"

	"github.com/gin-gonic/gin"
)

type SitesHandler struct {
	DB              *database.DB
	AdminAPIKey     string
	RegisterLimiter *IPRateLimiter
}

type siteRegisterRequest struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
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
	var req siteRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Domain = normalizeDomain(req.Domain)
	req.Name = strings.TrimSpace(req.Name)
	if req.Domain == "" || !validDomain(req.Domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid domain is required"})
		return
	}
	if len(req.Name) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name too long"})
		return
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("sites.register key generation failed domain=%s err=%v", req.Domain, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	_, err = h.DB.CreateSite(c.Request.Context(), apiKey, req.Domain, req.Name)
	if err != nil {
		log.Printf("sites.register db create failed domain=%s name=%s err=%v", req.Domain, req.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register site"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_key": apiKey})
}

func (h *SitesHandler) Delete(c *gin.Context) {
	if strings.TrimSpace(h.AdminAPIKey) == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin key not configured"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Admin-Key")), []byte(h.AdminAPIKey)) != 1 {
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
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "site_id": siteID})
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
