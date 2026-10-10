package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type RedirectsHandler struct {
	DB          *database.DB
	RateLimiter *IPRateLimiter
	// HomeURL is the Offer Rails home page. Unknown placement ids land here
	// when the site is not known.
	HomeURL string
	// IPSaltSecret is mixed into the daily server salt for direct go links.
	// Empty uses the development secret.
	IPSaltSecret string
	now          func() time.Time
}

const (
	offerRailsHome             = "https://offerrails.com/"
	placementClickDedupSeconds = 45
)

// Go is the tracked-link entry point published in publisher posts: it logs the
// click and forwards the visitor to the offer's landing page with the click id
// in the offer's ref parameter, which the maker's checkout carries back to us
// through provider metadata.
func (h *RedirectsHandler) Go(c *gin.Context) {
	stampGoResponse(c)
	code := strings.TrimSpace(c.Param("code"))
	req := captureGoRequest(c)
	scrubGoRequest(c.Request)
	if uuidPattern.MatchString(code) {
		h.goPlacement(c, code, req)
		return
	}
	if h.RateLimiter != nil && !h.RateLimiter.Allow(h.rateLimitKey(req.clientIP)) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	link, err := h.DB.GetCampaignByTrackingCode(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		revoked, err := h.DB.PlacementRevoked(c.Request.Context(), placementID, link.CampaignID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
		if revoked {
			c.JSON(http.StatusGone, gin.H{"error": "link unavailable", "reason": "revoked"})
			return
		}
		held, err := h.DB.PlacementRestoreUnconfirmed(c.Request.Context(), placementID, link.CampaignID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
		if held {
			c.JSON(http.StatusGone, gin.H{"error": "link unavailable", "reason": "failed"})
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
	if c.Request.Method == http.MethodHead || isPreviewBot(req.userAgent) {
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	clickID, err := h.DB.InsertPlacementClick(c.Request.Context(), link.CampaignID, link.SiteID, link.ProgramID, h.directReaderHash(req.clientIP), req.userAgent, req.referer, c.Query("placement"))
	if err != nil {
		logGo("redirects.go click insert failed campaign=%s err=%v", link.CampaignID, err)
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

// goPlacement is GET /v1/go/<placement_id>. It always answers 302. A ref is
// attached and a hit is recorded only when placement_counts_as_live is true.
func (h *RedirectsHandler) goPlacement(c *gin.Context, placementID string, req goRequest) {
	link, err := h.DB.LookupPlacementRedirect(c.Request.Context(), placementID)
	if err != nil {
		logGo("redirects.go placement lookup failed id=%s err=%v", placementID, err)
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return
	}
	reader, siteID, domain := h.placementReader(c, link, req)
	if !link.Found {
		if siteID != "" {
			c.Redirect(http.StatusFound, siteHome(domain))
			return
		}
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return
	}
	if !link.OfferListed || !httpsDestination(link.LandingURL) {
		c.Redirect(http.StatusFound, h.postOrHome(link))
		return
	}
	plain := link.LandingURL
	if !link.CountsAsLive || c.Request.Method == http.MethodHead || isPreviewBot(reader.userAgent) {
		c.Redirect(http.StatusFound, plain)
		return
	}
	clickID, err := h.recordPlacementHit(c, link, placementID, reader)
	if err != nil {
		logGo("redirects.go placement click failed id=%s err=%v", placementID, err)
		c.Redirect(http.StatusFound, plain)
		return
	}
	c.Redirect(http.StatusFound, appendRefParam(plain, link.RefParam, clickID))
}

type goReader struct {
	hash      string
	userAgent string
	referer   string
}

// placementReader hashes the connection address for a public hit. via=plugin
// uses the plugin's client hash, user agent, and referer only when the site
// key matches this placement's site. Any other key leaves those headers unused.
// A trusted plugin call does not fall back to the publisher server's address.
func (h *RedirectsHandler) placementReader(c *gin.Context, link database.PlacementRedirect, req goRequest) (goReader, string, string) {
	reader := goReader{
		hash:      h.directReaderHash(req.clientIP),
		userAgent: req.userAgent,
		referer:   req.referer,
	}
	if req.via != "plugin" {
		return reader, "", ""
	}
	if req.apiKey == "" {
		return reader, "", ""
	}
	siteID, domain, ok, err := h.DB.SiteForGoKey(c.Request.Context(), services.HashAPIKey(req.apiKey), services.KeyPrefix(req.apiKey))
	if err != nil || !ok {
		if err != nil {
			logGo("redirects.go plugin key lookup failed err=%v", err)
		}
		return reader, "", ""
	}
	if link.Found && siteID != link.SiteID {
		return reader, "", ""
	}
	if !link.Found {
		return reader, siteID, domain
	}
	if hashed := clientHashHeader(req.pluginHash); hashed != "" {
		reader.hash = hashed
	} else {
		reader.hash = ""
	}
	if ua := strings.TrimSpace(req.pluginUA); ua != "" {
		reader.userAgent = ua
	}
	if ref := strings.TrimSpace(req.pluginRef); ref != "" {
		reader.referer = ref
	}
	return reader, siteID, domain
}

func (h *RedirectsHandler) recordPlacementHit(c *gin.Context, link database.PlacementRedirect, placementID string, reader goReader) (string, error) {
	if link.CampaignID == "" {
		return "", errors.New("live placement has no campaign")
	}
	existing, err := h.DB.RecentPlacementClick(c.Request.Context(), link.SiteID, placementID, reader.hash, reader.userAgent, placementClickDedupSeconds)
	if err != nil || existing != "" {
		return existing, err
	}
	return h.DB.InsertPlacementClick(c.Request.Context(), link.CampaignID, link.SiteID, link.ProgramID, reader.hash, reader.userAgent, reader.referer, placementID)
}

func (h *RedirectsHandler) postOrHome(link database.PlacementRedirect) string {
	if httpsDestination(link.PostURL) {
		return link.PostURL
	}
	if home := siteHome(link.SiteDomain); home != "" {
		return home
	}
	return h.offerRailsHome()
}

func (h *RedirectsHandler) offerRailsHome() string {
	if home := strings.TrimSpace(h.HomeURL); home != "" {
		return home
	}
	return offerRailsHome
}

func siteHome(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if strings.Contains(domain, "://") {
		return strings.TrimRight(domain, "/") + "/"
	}
	return "https://" + strings.Trim(domain, "/") + "/"
}
