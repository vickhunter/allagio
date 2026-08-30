package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type RedirectsHandler struct {
	DB          *database.DB
	RateLimiter *IPRateLimiter
}

// Go is the tracked-link entry point published in publisher posts: it logs the
// click and forwards the visitor to the offer's landing page with the click id
// in the offer's ref parameter, which the maker's checkout carries back to us
// through provider metadata.
func (h *RedirectsHandler) Go(c *gin.Context) {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	if h.RateLimiter != nil && !h.RateLimiter.Allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	code := strings.TrimSpace(c.Param("code"))
	link, err := h.DB.GetCampaignByTrackingCode(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown link"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	// A paused campaign or unlisted offer still forwards the visitor, but
	// without attribution: the destination stays useful, the earning stops.
	if link.CampaignStatus != "active" || link.OfferStatus != "listed" {
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	clickID, err := h.DB.InsertCampaignClick(c.Request.Context(), link.CampaignID, link.SiteID, link.ProgramID, hashIP(c.ClientIP()), c.Request.UserAgent(), c.Request.Referer())
	if err != nil {
		log.Printf("redirects.go click insert failed campaign=%s err=%v", link.CampaignID, err)
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	c.Redirect(http.StatusFound, appendRefParam(link.LandingURL, link.RefParam, clickID))
}

func appendRefParam(landingURL, refParam, clickID string) string {
	parsed, err := url.Parse(landingURL)
	if err != nil {
		return landingURL
	}
	q := parsed.Query()
	q.Set(refParam, clickID)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
