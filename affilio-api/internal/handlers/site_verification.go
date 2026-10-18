package handlers

import (
	"bytes"
	"errors"
	"html"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/preview"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const bareVerificationBytes = 8192

func (h *SitesHandler) Current(c *gin.Context) {
	h.writeVerification(c, siteIDFrom(c))
}

func (h *SitesHandler) Get(c *gin.Context) {
	if strings.TrimSpace(c.Param("id")) != siteIDFrom(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	h.writeVerification(c, siteIDFrom(c))
}

func (h *SitesHandler) writeVerification(c *gin.Context, siteID string) {
	if siteID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	rec, err := h.DB.GetSiteVerification(c.Request.Context(), siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		log.Printf("sites.verification lookup failed site_id=%s", siteID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	if rec.Token == "" {
		rec.Token, err = h.DB.EnsureSiteVerificationToken(c.Request.Context(), siteID)
		if err != nil {
			log.Printf("sites.verification token failed site_id=%s", siteID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
	}
	c.JSON(http.StatusOK, verificationJSON(rec))
}

func (h *SitesHandler) Verify(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("id"))
	if siteID == "" || siteID != siteIDFrom(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	rec, err := h.DB.GetSiteVerification(c.Request.Context(), siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		log.Printf("sites.verify lookup failed site_id=%s", siteID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	if rec.Token == "" {
		rec.Token, err = h.DB.EnsureSiteVerificationToken(c.Request.Context(), siteID)
		if err != nil {
			log.Printf("sites.verify token failed site_id=%s", siteID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
	}
	verified, err := h.DB.DomainVerified(c.Request.Context(), rec.Domain)
	if err != nil {
		log.Printf("sites.verify domain lookup failed site_id=%s", siteID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	if verified {
		writeVerifyResult(c, http.StatusConflict, "domain_verified", "This domain is already verified.")
		return
	}
	wellKnown, home, err := siteVerificationURLs(rec.Domain)
	if err != nil {
		writeVerifyResult(c, http.StatusUnprocessableEntity, "verification_unreachable", "The domain could not be checked for the verification file or home page.")
		return
	}
	sawPage := false
	proved := false
	if doc, fetchErr := h.Fetch.FetchDocument(c.Request.Context(), wellKnown); fetchErr == nil {
		sawPage = true
		proved = fileProves(doc, rec.Token)
	}
	if !proved {
		if doc, fetchErr := h.Fetch.FetchDocument(c.Request.Context(), home); fetchErr == nil {
			sawPage = true
			proved = metaProves(doc, rec.Token)
		}
	}
	if !proved {
		if sawPage {
			writeVerifyResult(c, http.StatusUnprocessableEntity, "verification_missing", "The verification token was not found in the well-known file or the home page meta tag.")
			return
		}
		writeVerifyResult(c, http.StatusUnprocessableEntity, "verification_unreachable", "The domain could not be checked for the verification file or home page.")
		return
	}
	at, err := h.DB.MarkSiteVerified(c.Request.Context(), siteID)
	if errors.Is(err, pgx.ErrNoRows) || isUniqueViolation(err) {
		writeVerifyResult(c, http.StatusConflict, "domain_verified", "This domain is already verified.")
		return
	}
	if err != nil {
		log.Printf("sites.verify store failed site_id=%s", siteID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "verification failed"})
		return
	}
	rec.VerifiedAt = &at
	c.JSON(http.StatusOK, verificationJSON(rec))
}

func writeVerifyResult(c *gin.Context, status int, reason, message string) {
	c.JSON(status, gin.H{"reason": reason, "message": message})
}

func verificationJSON(rec database.SiteVerification) gin.H {
	body := gin.H{
		"site_id":            rec.ID,
		"domain":             rec.Domain,
		"verified":           rec.VerifiedAt != nil,
		"verified_at":        nil,
		"verification_token": rec.Token,
	}
	if rec.VerifiedAt != nil {
		body["verified_at"] = rec.VerifiedAt.UTC().Format(time.RFC3339)
	}
	return body
}

func siteVerificationURLs(domain string) (string, string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" || strings.ContainsAny(domain, "/\\?#@") || strings.Contains(domain, "..") {
		return "", "", preview.ErrInvalidURL
	}
	host := domain
	if strings.Contains(domain, ":") {
		name, port, err := net.SplitHostPort(domain)
		if err != nil || name == "" || port == "" {
			return "", "", preview.ErrInvalidURL
		}
		host = net.JoinHostPort(name, port)
	}
	wellKnown := url.URL{Scheme: "https", Host: host, Path: "/.well-known/offer-rails.txt"}
	home := url.URL{Scheme: "https", Host: host, Path: "/"}
	return wellKnown.String(), home.String(), nil
}

func fileProves(doc preview.Document, token string) bool {
	if token == "" || !fileContent(doc.ContentType, len(doc.Body)) {
		return false
	}
	text := string(bytes.TrimPrefix(doc.Body, []byte{0xEF, 0xBB, 0xBF}))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if strings.TrimSpace(text) == token {
		return true
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == token {
			return true
		}
	}
	return false
}

func fileContent(header string, n int) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return n <= bareVerificationBytes
	}
	media, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	switch strings.ToLower(media) {
	case "text/plain", "text/html":
		return true
	default:
		return false
	}
}

func metaProves(doc preview.Document, token string) bool {
	if token == "" || !htmlDocument(doc) {
		return false
	}
	return metaHasToken(string(doc.Body), token)
}

func htmlDocument(doc preview.Document) bool {
	header := strings.TrimSpace(doc.ContentType)
	if header == "" {
		return strings.Contains(strings.ToLower(string(doc.Body)), "<meta")
	}
	media, _, err := mime.ParseMediaType(header)
	return err == nil && strings.EqualFold(media, "text/html")
}

var (
	metaTagPattern  = regexp.MustCompile(`(?i)<meta\b([^>]*)>`)
	metaAttrPattern = regexp.MustCompile(`([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)')`)
)

func metaHasToken(raw, token string) bool {
	tags := metaTagPattern.FindAllStringSubmatch(raw, -1)
	for _, tag := range tags {
		attrs := map[string]string{}
		for _, attr := range metaAttrPattern.FindAllStringSubmatch(tag[1], -1) {
			inner := attr[4]
			if strings.HasPrefix(attr[2], `"`) {
				inner = attr[3]
			}
			attrs[strings.ToLower(attr[1])] = strings.TrimSpace(html.UnescapeString(inner))
		}
		if strings.EqualFold(attrs["name"], "offer-rails-verification") && attrs["content"] == token {
			return true
		}
	}
	return false
}
