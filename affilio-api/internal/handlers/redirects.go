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
			landing, ok, lookupErr := h.DB.OfferLandingByPreviewCode(c.Request.Context(), code)
			if lookupErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
				return
			}
			if ok && httpsDestination(landing) {
				c.Redirect(http.StatusFound, landing)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown link"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	// Paused, unlisted, and removed links are gone. Do not send the visitor
	// onward and do not record a click, including for preview bots and HEAD.
	if status, reason, stop := trackingUnavailable(link); stop {
		c.JSON(status, gin.H{"error": "link unavailable", "reason": reason})
		return
	}
	if !httpsDestination(link.LandingURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "destination must be https"})
		return
	}
	if link.RequiresPlacement && c.Query("placement") == "" {
		c.JSON(http.StatusGone, gin.H{"error": "link unavailable", "reason": "removed"})
		return
	}
	if placementID := c.Query("placement"); placementID != "" {
		if !uuidPattern.MatchString(placementID) {
			c.JSON(http.StatusGone, gin.H{"error": "link unavailable", "reason": "removed"})
			return
		}
		removed, err := h.DB.PlacementRemoved(c.Request.Context(), placementID, link.CampaignID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
		if removed {
			c.Redirect(http.StatusFound, link.LandingURL)
			return
		}
		valid, err := h.DB.PlacementAllowsClick(c.Request.Context(), placementID, link.CampaignID)
		if err != nil || !valid {
			c.JSON(http.StatusGone, gin.H{"error": "link unavailable", "reason": "removed"})
			return
		}
	}
	// Link-preview fetches and HEAD probes still need the destination, but
	// they are not visitors and must not create a click or an attribution ref.
	if c.Request.Method == http.MethodHead || isPreviewBot(c.Request.UserAgent()) {
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	clickID, err := h.DB.InsertPlacementClick(c.Request.Context(), link.CampaignID, link.SiteID, link.ProgramID, hashIP(c.ClientIP()), c.Request.UserAgent(), c.Request.Referer(), c.Query("placement"))
	if err != nil {
		log.Printf("redirects.go click insert failed campaign=%s err=%v", link.CampaignID, err)
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	c.Redirect(http.StatusFound, appendRefParam(link.LandingURL, link.RefParam, clickID))
}

// previewBotMarkers are link-unfurl clients. Matching is a case-insensitive
// substring of the User-Agent, modeled on offerrails isPreviewBot.
var previewBotMarkers = []string{
	"facebookexternalhit",
	"facebot",
	"twitterbot",
	"linkedinbot",
	"slackbot",
	"slack-imgproxy",
	"discordbot",
	"telegrambot",
	"whatsapp",
	"pinterest",
	"redditbot",
	"embedly",
	"quora link preview",
	"showyoubot",
	"outbrain",
	"vkshare",
	"w3c_validator",
	"bingpreview",
	"skypeuripreview",
	"applebot",
	"iframely",
}

func httpsDestination(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func trackingUnavailable(link *database.CampaignLink) (int, string, bool) {
	if link.CampaignStatus == "paused" || link.OfferStatus == "paused" {
		return http.StatusGone, "paused", true
	}
	if link.OfferStatus == "retired" || link.CampaignStatus == "retired" || link.CampaignStatus == "inactive" {
		return http.StatusGone, "removed", true
	}
	if link.CampaignStatus != "active" || link.OfferStatus != "listed" {
		return http.StatusGone, "unlisted", true
	}
	return 0, "", false
}

func isPreviewBot(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return false
	}
	for _, marker := range previewBotMarkers {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
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
