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
	// HomeURL is the Offer Rails home page. Unknown campaign codes land here.
	// An unknown placement id is a 404 page instead of a redirect.
	HomeURL string
	// IPSaltSecret is mixed into the daily server salt for direct go links.
	IPSaltSecret string
	now          func() time.Time
}

const (
	// OfferRailsHome is the public host. An unknown /v1/go code lands here
	// when HomeURL is empty. PUBLIC_BASE_URL, stored in HomeURL, wins when set.
	OfferRailsHome             = "https://offer-rails.onrender.com/"
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
	// /v1/go/demo is the sample link on the demo post. It redirects and records nothing.
	if code == demoGoCode {
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return
	}
	if uuidPattern.MatchString(code) {
		h.goPlacement(c, code, req)
		return
	}
	h.goCampaign(c, code, req)
}

// goCampaign is the permanent GET /v1/go/<tracking_code> route. Saved posts,
// feeds, and caches keep this link, and nothing rewrites them, so both this
// form and /v1/go/<placement_id> follow the go-link rules forever. Readers
// never see a 4xx or a 5xx. ?placement= delegates to goPlacement. Without it,
// a hit is recorded only when exactly one placement on this campaign is
// clickable: live, approved and still inside its window, or committed
// before plugin-ack. Anything else
// is a 302 to the plain offer URL with no hit. An offer that is not live
// redirects to the site home, and an unknown code redirects to the Offer
// Rails home. A rate limit is that untracked 302, never a 429.
func (h *RedirectsHandler) goCampaign(c *gin.Context, code string, req goRequest) {
	if placementID := strings.TrimSpace(c.Query("placement")); placementID != "" {
		h.goPlacement(c, placementID, req)
		return
	}
	link, err := h.DB.GetCampaignByTrackingCode(c.Request.Context(), code)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			logGo("redirects.go campaign lookup failed code=%s err=%v", code, err)
		}
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return
	}
	if !campaignOfferLive(link) {
		c.Redirect(http.StatusFound, h.campaignFallback(c, link, ""))
		return
	}
	liveID, err := h.DB.LivePlacementForCampaign(c.Request.Context(), link.CampaignID)
	if err != nil {
		logGo("redirects.go live placement lookup failed campaign=%s err=%v", link.CampaignID, err)
		c.Redirect(http.StatusFound, link.LandingURL)
		return
	}
	if liveID != "" {
		h.goPlacement(c, liveID, req)
		return
	}
	c.Redirect(http.StatusFound, link.LandingURL)
}

// campaignOfferLive is the offer a reader can be sent to: listed, with an
// https landing URL.
func campaignOfferLive(link *database.CampaignLink) bool {
	return link != nil && link.OfferStatus == "listed" && httpsDestination(link.LandingURL)
}

// campaignFallback is where a reader goes when the offer itself is not live:
// the placement's post, then the site home, then the Offer Rails home.
func (h *RedirectsHandler) campaignFallback(c *gin.Context, link *database.CampaignLink, placementID string) string {
	if uuidPattern.MatchString(placementID) {
		post, err := h.DB.PlacementPostURL(c.Request.Context(), placementID, link.CampaignID)
		if err != nil {
			logGo("redirects.go post url lookup failed campaign=%s err=%v", link.CampaignID, err)
		} else if httpsDestination(post) {
			return post
		}
	}
	if home := siteHome(link.SiteDomain); home != "" {
		return home
	}
	return h.offerRailsHome()
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

// goPlacement is GET /v1/go/<placement_id>. A known placement answers 302. A
// ref is attached and a hit is recorded when CountsAsLive is true: the
// placement is in placement_counts_as_live, it is approved and still inside
// its window with an active campaign for the listed offer, or it is committed
// and waiting on plugin-ack. plugin-ack sets the verified placed state and
// does not gate the click. Approval does not store campaign_id; the lookup
// uses that campaign. An unknown id is an HTML 404 with no Location, so the
// request port stays on the page the reader opened.
func (h *RedirectsHandler) goPlacement(c *gin.Context, placementID string, req goRequest) {
	link, err := h.DB.LookupPlacementRedirect(c.Request.Context(), placementID)
	if err != nil {
		logGo("redirects.go placement lookup failed id=%s err=%v", placementID, err)
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return
	}
	if !link.Found {
		if h.goRecommendation(c, placementID, req) {
			return
		}
		writeUnknownGoLink(c)
		return
	}
	reader, _, _ := h.placementReader(c, link, req)
	if !link.OfferListed || !httpsDestination(link.LandingURL) {
		c.Redirect(http.StatusFound, h.postOrHome(link))
		return
	}
	plain := link.LandingURL
	if !link.CountsAsLive {
		c.Redirect(http.StatusFound, plain)
		return
	}
	class := ClassifyClick(c.Request.Method, reader.userAgent, c.Request.Header)
	if link.Demo {
		class = ClickClassTest
	}
	if !PaidClick(class) {
		h.finishPlacementRedirect(c, link, placementID, reader, class, plain, "unpaid:"+placementID)
		return
	}
	h.finishPlacementRedirect(c, link, placementID, reader, class, plain, "placement:"+placementID)
}

// goRecommendation serves GET /v1/go/<recommendation_id>. A known recommendation
// with an https affiliate URL records one attributable click and answers 302
// to that URL unchanged, including its merchant ref. An unknown id returns
// false so the caller can write the placement 404.
func (h *RedirectsHandler) goRecommendation(c *gin.Context, recommendationID string, req goRequest) bool {
	rec, err := h.DB.LookupRecommendationRedirect(c.Request.Context(), recommendationID)
	if err != nil {
		logGo("redirects.go recommendation lookup failed id=%s err=%v", recommendationID, err)
		c.Redirect(http.StatusFound, h.offerRailsHome())
		return true
	}
	if !rec.Found || !httpsDestination(rec.Destination) {
		return false
	}
	reader := h.recommendationReader(c, rec.SiteID, req)
	class := ClassifyClick(c.Request.Method, reader.userAgent, c.Request.Header)
	if rec.Demo {
		class = ClickClassTest
	}
	if h.RateLimiter != nil && !h.RateLimiter.Allow("recommendation:"+recommendationID) {
		c.Redirect(http.StatusFound, rec.Destination)
		return true
	}
	if AttributableClick(class) || class == ClickClassTest {
		if _, err := h.recordRecommendationHit(c, rec, recommendationID, reader, class); err != nil {
			logGo("redirects.go recommendation click failed id=%s err=%v", recommendationID, err)
		}
	}
	c.Redirect(http.StatusFound, rec.Destination)
	return true
}

func (h *RedirectsHandler) recordRecommendationHit(c *gin.Context, rec database.RecommendationRedirect, recommendationID string, reader goReader, class string) (string, error) {
	existing, err := h.DB.RecentRecommendationClick(c.Request.Context(), rec.SiteID, recommendationID, reader.hash, reader.userAgent, class, placementClickDedupSeconds)
	if err != nil || existing != "" {
		return existing, err
	}
	return h.DB.InsertRecommendationClick(c.Request.Context(), rec.SiteID, rec.ArticleID, recommendationID, rec.ProgramID, reader.hash, reader.userAgent, reader.referer, class)
}

// finishPlacementRedirect records the hit when the named budget still has
// room. Paid clicks use the placement budget. Every other class uses its own
// budget and returns before that placement budget is touched.
func (h *RedirectsHandler) finishPlacementRedirect(c *gin.Context, link database.PlacementRedirect, placementID string, reader goReader, class, plain, limitKey string) {
	if h.RateLimiter != nil && !h.RateLimiter.Allow(limitKey) {
		c.Redirect(http.StatusFound, plain)
		return
	}
	clickID, err := h.recordPlacementHit(c, link, placementID, reader, class)
	if err != nil {
		logGo("redirects.go placement click failed id=%s err=%v", placementID, err)
		c.Redirect(http.StatusFound, plain)
		return
	}
	if !AttributableClick(class) {
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

// recommendationReader hashes the connection address for a public hit.
// via=plugin uses the plugin's client hash, user agent, and referer only when
// the site key matches this recommendation's site. A missing key, a key that
// does not resolve, and a key for another site leave those headers unused, so
// a WordPress user agent on the connection is what gets classified.
func (h *RedirectsHandler) recommendationReader(c *gin.Context, siteID string, req goRequest) goReader {
	reader := goReader{
		hash:      h.directReaderHash(req.clientIP),
		userAgent: req.userAgent,
		referer:   req.referer,
	}
	if req.via != "plugin" || req.apiKey == "" || siteID == "" {
		return reader
	}
	keySite, _, ok, err := h.DB.SiteForGoKey(c.Request.Context(), services.HashAPIKey(req.apiKey), services.KeyPrefix(req.apiKey))
	if err != nil || !ok || keySite != siteID {
		if err != nil {
			logGo("redirects.go plugin key lookup failed err=%v", err)
		}
		return reader
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
	return reader
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

func (h *RedirectsHandler) recordPlacementHit(c *gin.Context, link database.PlacementRedirect, placementID string, reader goReader, class string) (string, error) {
	if link.CampaignID == "" {
		return "", errors.New("live placement has no campaign")
	}
	existing, err := h.DB.RecentPlacementClick(c.Request.Context(), link.SiteID, placementID, reader.hash, reader.userAgent, class, placementClickDedupSeconds)
	if err != nil || existing != "" {
		return existing, err
	}
	return h.DB.InsertPlacementClick(c.Request.Context(), link.CampaignID, link.SiteID, link.ProgramID, reader.hash, reader.userAgent, reader.referer, placementID, class)
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

func writeUnknownGoLink(c *gin.Context) {
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte("<!DOCTYPE html><html><head><title>Not found</title></head><body><p>This link is not on Offer Rails.</p></body></html>\n"))
}

func (h *RedirectsHandler) offerRailsHome() string {
	if home := strings.TrimSpace(h.HomeURL); home != "" {
		return home
	}
	return OfferRailsHome
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
