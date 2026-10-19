package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type PlacementsHandler struct {
	DB                        *database.DB
	RequireVerifiedConnection bool
	Now                       func() time.Time
	PluginOfflineAfter        time.Duration
	StatusLimiter             *IPRateLimiter
	// Claims is the production checker. Nil uses the stub, which refuses a
	// number the offer does not contain and is not a live claim check.
	Claims entry.ClaimChecker
}

// PlacementPollInterval is the shortest period the placement status read may
// enforce. One call every 2 seconds per placement leaves headroom for a
// client that polls every 4 seconds.
const PlacementPollInterval = 2 * time.Second

func NewPlacementPollLimiter() *IPRateLimiter {
	return NewIPRateLimiter(1, PlacementPollInterval)
}

type opportunity struct {
	OfferID          string  `json:"offer_id"`
	ProductName      string  `json:"product_name"`
	Relevance        int     `json:"relevance"`
	Reason           string  `json:"reason"`
	CommissionType   string  `json:"commission_type"`
	CommissionValue  float64 `json:"commission_value"`
	Currency         string  `json:"currency"`
	RefundWindowDays int     `json:"refund_window_days"`
	PayoutMethod     string  `json:"payout_method"`
}

// relevanceThreshold is the same bar DryRunMatch uses. An offer below it is not a match.
const relevanceThreshold = services.RelevanceFloor

type opportunitySearch struct {
	items     []opportunity
	reason    string
	bestScore int
}

func (h *PlacementsHandler) opportunities(c *gin.Context, contextText string) (opportunitySearch, error) {
	siteID := siteIDFrom(c)
	if siteID == "" {
		siteID = "00000000-0000-0000-0000-000000000000"
	}
	offers, err := h.DB.ListMarketplaceOffers(c.Request.Context(), siteID)
	if err != nil {
		return opportunitySearch{}, err
	}
	eligible, err := h.DB.EligibleRevenueOfferIDs(c.Request.Context(), h.RequireVerifiedConnection)
	if err != nil {
		return opportunitySearch{}, err
	}
	profile := services.BuildSiteProfile([]string{contextText}, nil)
	found := opportunitySearch{items: []opportunity{}, bestScore: -1}
	sawEligible := false
	for _, o := range offers {
		if !eligible[o.OfferID] {
			continue
		}
		sawEligible = true
		score := services.ScoreOfferForProfile(profile, services.OfferTerms{Name: o.ProductName, Tagline: o.ProductTagline, Category: o.PrimaryCategory, Keywords: o.Keywords, Tags: o.Tags})
		if score > found.bestScore {
			found.bestScore = score
		}
		if score < relevanceThreshold {
			continue
		}
		found.items = append(found.items, opportunity{OfferID: o.OfferID, ProductName: o.ProductName, Relevance: score,
			Reason:         fmt.Sprintf("Product topics overlap this content (%d/100 lexical fit); review the product before recommending it.", score),
			CommissionType: o.CommissionType, CommissionValue: o.CommissionValue, Currency: o.CommissionCurrency,
			RefundWindowDays: o.RefundWindowDays, PayoutMethod: "direct_merchant"})
	}
	sort.SliceStable(found.items, func(i, j int) bool { return found.items[i].Relevance > found.items[j].Relevance })
	if len(found.items) > 5 {
		found.items = found.items[:5]
	}
	if len(found.items) == 0 {
		if sawEligible {
			found.reason = "below_relevance"
		} else {
			found.reason = "no_eligible_offers"
		}
	}
	return found, nil
}

func (h *PlacementsHandler) Find(c *gin.Context) {
	var req struct {
		Context    string `json:"context"`
		SurfaceURL string `json:"surface_url"`
	}
	if bindBoundedJSON(c, &req) != nil || len(strings.TrimSpace(req.Context)) < 10 || len(req.Context) > 12000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context must contain 10–12000 characters"})
		return
	}
	out, err := h.opportunities(c, req.Context)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "opportunity search failed"})
		return
	}
	status := "matched"
	body := gin.H{"status": status, "opportunities": out.items, "preview_only": siteIDFrom(c) == "", "payment_scope": "first paid Stripe checkout; USD/EUR/GBP"}
	if len(out.items) == 0 {
		status = "no_match"
		body["status"] = status
		body["reason"] = out.reason
		if out.reason == "below_relevance" {
			body["best_score"] = out.bestScore
			body["threshold"] = relevanceThreshold
		}
	}
	c.JSON(http.StatusOK, body)
}

func (h *PlacementsHandler) requireLinkedPublisher(c *gin.Context) bool {
	linked, err := h.DB.SiteHasOwner(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "publisher lookup failed"})
		return false
	}
	if !linked {
		c.JSON(http.StatusForbidden, gin.H{"error": "publisher account is not connected"})
		return false
	}
	return true
}

func (h *PlacementsHandler) claims() entry.ClaimChecker {
	if h.Claims == nil {
		return entry.StubClaimChecker{}
	}
	return h.Claims
}

func (h *PlacementsHandler) Prepare(c *gin.Context) {
	var req struct {
		OfferID    string `json:"offer_id"`
		SurfaceURL string `json:"surface_url"`
		Context    string `json:"context"`
		Copy       string `json:"copy"`
		PostID     int64  `json:"post_id"`
		WPPostID   int64  `json:"wp_post_id"`
		PostTitle  string `json:"post_title"`
	}
	if bindBoundedJSON(c, &req) != nil || !uuidPattern.MatchString(req.OfferID) || len(req.Context) < 10 || len(req.Context) > 12000 || len(strings.TrimSpace(req.Copy)) < 10 || len(req.Copy) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "offer_id, surface_url, context and copy required"})
		return
	}
	if !h.requireLinkedPublisher(c) {
		return
	}
	site, err := h.DB.GetSiteByID(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "publisher lookup failed"})
		return
	}
	if problem := surfaceURLProblem(req.SurfaceURL, site.Domain); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}
	matched, err := h.opportunities(c, req.Context)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "opportunity search failed"})
		return
	}
	reason := ""
	for _, o := range matched.items {
		if o.OfferID == req.OfferID {
			reason = o.Reason
			break
		}
	}
	if reason == "" {
		writeNamedConflict(c, "offer_ineligible")
		return
	}
	offerText, err := h.DB.OfferClaimText(c.Request.Context(), req.OfferID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	if claimErr := h.claims().CheckClaims(strings.TrimSpace(req.Copy), offerText); claimErr != nil {
		writeClaimsDenied(c, claimErr)
		return
	}
	postID := req.PostID
	if req.WPPostID > 0 {
		postID = req.WPPostID
	}
	boxes, err := h.DB.CountPlacementBoxes(c.Request.Context(), site.ID, req.SurfaceURL, "00000000-0000-0000-0000-000000000000", postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	if boxes > 0 {
		writeNamedConflict(c, "one_per_post")
		return
	}
	p, err := h.DB.CreatePlacement(c.Request.Context(), database.Placement{SiteID: site.ID, OfferID: req.OfferID, SurfaceURL: req.SurfaceURL,
		Context: req.Context, Copy: strings.TrimSpace(req.Copy), Disclosure: "Affiliate disclosure: I may earn a commission if you buy through this link.", Reason: reason, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
		WPPostID: postID, PostTitle: strings.TrimSpace(req.PostTitle)})
	if errors.Is(err, database.ErrOnePerPost) {
		writeNamedConflict(c, "one_per_post")
		return
	}
	if err != nil {
		writeNamedConflict(c, "offer_unavailable")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"placement": p, "next_action": "Owner reviews exact copy, disclosure, surface and terms; an operator records their approval."})
}

func (h *PlacementsHandler) Get(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	writePlacementStatus(c, h.DB, h.StatusLimiter, h.now(), h.pluginWindow(), siteIDFrom(c), "")
}

func (h *PlacementsHandler) pluginWindow() time.Duration {
	if h.PluginOfflineAfter > 0 {
		return h.PluginOfflineAfter
	}
	return 10 * time.Minute
}

func writePlacementStatus(c *gin.Context, db *database.DB, limiter *IPRateLimiter, now time.Time, window time.Duration, siteID, userID string) {
	id := c.Param("id")
	if limiter != nil && !limiter.Allow(placementPollKey(c, id, userID)) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	if siteID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	status, wait, failedReason, failedAction, matchedRule, surfaceURL, waitAt, err := db.ReadPlacementStatus(c.Request.Context(), siteID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement status unavailable"})
		return
	}
	fallback, err := db.PlacementFallbackURL(c.Request.Context(), siteID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement status unavailable"})
		return
	}
	ruleID, ruleText := entry.PresentRule(matchedRule)
	c.JSON(http.StatusOK, gin.H{
		"id":                   id,
		"status":               status,
		"plugin_state":         entry.PluginState(wait, waitAt, failedReason != "", now, window),
		"plugin_failed_reason": failedReason,
		"failed_action":        failedAction,
		"fallback_url":         fallback,
		"policy_rule_id":       ruleID,
		"policy_rule_text":     ruleText,
		"surface_url":          surfaceURL,
		"placement":            gin.H{"surface_url": surfaceURL},
	})
}

func placementPollKey(c *gin.Context, placementID, userID string) string {
	identity := c.GetString("api_key_hash")
	if identity == "" {
		identity = userID
	}
	if identity == "" {
		identity = siteIDFrom(c)
	}
	return placementID + "\x00" + identity
}

func (h *PlacementsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h *PlacementsHandler) List(c *gin.Context) {
	siteID := siteIDFrom(c)
	if siteID != "" {
		if err := h.DB.TouchPlacementContact(c.Request.Context(), siteID, h.now()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "task inbox unavailable"})
			return
		}
	}
	p, err := h.DB.ListPlacements(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task inbox unavailable"})
		return
	}
	fallbacks, err := h.DB.PlacementFallbackURLs(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task inbox unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"placements": withFallbackURLs(p, fallbacks)})
}

type placementPayload struct {
	database.Placement
	FallbackURL string `json:"fallback_url"`
}

func withFallbackURLs(items []database.Placement, fallbacks map[string]string) []placementPayload {
	out := make([]placementPayload, 0, len(items))
	for _, item := range items {
		out = append(out, placementPayload{Placement: item, FallbackURL: fallbacks[item.ID]})
	}
	return out
}

func (h *PlacementsHandler) Commit(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	if !h.requireLinkedPublisher(c) {
		return
	}
	code, err := generateTrackingCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "link generation failed"})
		return
	}
	p, err := h.DB.CommitPlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"), code, h.claims())
	if errors.Is(err, database.ErrPlacementAuthority) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, database.ErrClaimsFromOffer) {
		writeClaimsDenied(c, err)
		return
	}
	if errors.Is(err, database.ErrOnePerPost) {
		writeNamedConflict(c, "one_per_post")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}
	fallback, err := h.DB.PlacementFallbackURL(c.Request.Context(), siteIDFrom(c), p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"placement": placementPayload{Placement: *p, FallbackURL: fallback}, "next_action": "Use the publisher's existing publishing tool for approved copy, tracked link and disclosure, then record the receipt."})
}

func (h *PlacementsHandler) Approve(c *gin.Context) {
	var req struct {
		SiteID     string `json:"site_id"`
		OwnerEmail string `json:"owner_email"`
		Evidence   string `json:"evidence"`
	}
	if bindBoundedJSON(c, &req) != nil || !uuidPattern.MatchString(c.Param("id")) || !uuidPattern.MatchString(req.SiteID) || len(req.Evidence) < 10 || len(req.Evidence) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id, owner_email and owner approval evidence required"})
		return
	}
	address, err := mail.ParseAddress(req.OwnerEmail)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid owner_email required"})
		return
	}
	evidence, _ := json.Marshal(gin.H{"owner_email": address.Address, "evidence": req.Evidence})
	updated, err := h.DB.ApprovePlacement(c.Request.Context(), c.Param("id"), req.SiteID, address.Address, string(evidence))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	if !updated {
		writeNamedConflict(c, "placement_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "approved", "placement_id": c.Param("id")})
}

func (h *PlacementsHandler) Revoke(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	updated, err := h.DB.RevokePlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revocation failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

var (
	htmlTags        = regexp.MustCompile(`(?s)<[^>]*>`)
	looseDisclosure = regexp.MustCompile(`(?i)link.{0,60}affiliat|affiliat.{0,60}link`)
)

func publishedDisclosureOK(published, approved string) bool {
	if approved != "" && strings.Contains(published, approved) {
		return true
	}
	text := strings.ToLower(htmlTags.ReplaceAllString(published, ""))
	return strings.Contains(text, "commissione") || looseDisclosure.MatchString(text)
}

func (h *PlacementsHandler) RecordReceipt(c *gin.Context) {
	var req struct {
		SurfaceURL    string `json:"surface_url"`
		PublishedText string `json:"published_text"`
		Evidence      string `json:"evidence"`
		PostTitle     string `json:"post_title"`
		PostID        int64  `json:"post_id"`
		WPPostID      int64  `json:"wp_post_id"`
	}
	if !uuidPattern.MatchString(c.Param("id")) || bindBoundedJSON(c, &req) != nil || len(req.PublishedText) > 16000 || len(req.Evidence) < 10 || len(req.Evidence) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "surface_url, published_text and evidence required"})
		return
	}
	p, err := h.DB.GetPlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	var receipt struct {
		TrackedLink string `json:"tracked_link"`
	}
	wantLink := "/v1/go/" + p.ID
	publishedLink := strings.Contains(req.PublishedText, "/go/"+p.ID)
	if json.Unmarshal(p.Receipt, &receipt) != nil || receipt.TrackedLink != wantLink || !strings.Contains(req.PublishedText, p.Copy) || !publishedDisclosureOK(req.PublishedText, p.Disclosure) || !publishedLink {
		if err = h.DB.ReopenRefusedReceipt(c.Request.Context(), siteIDFrom(c), p.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "receipt recording failed"})
			return
		}
		writeNamedConflict(c, "receipt_mismatch")
		return
	}
	hash := sha256.Sum256([]byte(req.PublishedText))
	postID := req.PostID
	if req.WPPostID > 0 {
		postID = req.WPPostID
	}
	updated, err := h.DB.RecordPlacementReceipt(c.Request.Context(), siteIDFrom(c), p.ID, req.SurfaceURL, hex.EncodeToString(hash[:]), req.Evidence, strings.TrimSpace(req.PostTitle), postID, h.now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "receipt recording failed"})
		return
	}
	if !updated {
		writeNamedConflict(c, "receipt_recorded")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "publisher_reported", "content_sha256": hex.EncodeToString(hash[:])})
}

func (h *PlacementsHandler) Refuse(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if !uuidPattern.MatchString(c.Param("id")) || bindBoundedJSON(c, &req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "placement id and reason required"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len(reason) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason required"})
		return
	}
	outcome, err := h.DB.RefusePlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"), reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement refusal failed"})
		return
	}
	switch outcome {
	case "missing":
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
	case "recorded":
		writeNamedConflict(c, "receipt_recorded")
	case "unavailable":
		writeNamedConflict(c, "placement_unavailable")
	case "already refused":
		c.JSON(http.StatusOK, gin.H{"status": "already refused", "publication_status": "refused", "placement_id": c.Param("id")})
	case "refused":
		c.JSON(http.StatusOK, gin.H{"status": "pending", "publication_status": "refused", "placement_id": c.Param("id")})
	default:
		writeNamedConflict(c, "placement_unavailable")
	}
}

func (h *PlacementsHandler) MarkRemoved(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	already, at, found, err := h.DB.MarkPlacementRemoved(c.Request.Context(), siteIDFrom(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "removal recording failed"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "removed", "already_removed": already, "removed_at": at.UTC().Format(time.RFC3339)})
}

func (h *PlacementsHandler) ListForMaker(c *gin.Context) {
	items, err := h.DB.ListMakerPlacements(c.Request.Context(), makerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load placements"})
		return
	}
	if items == nil {
		items = []database.MakerPlacement{}
	}
	c.JSON(http.StatusOK, gin.H{"placements": items})
}

func PlacementDashboardLabel(status string, removed *time.Time) string {
	if removed != nil {
		return "Rimosso " + removed.UTC().Format("2006-01-02")
	}
	return status
}

func (h *PlacementsHandler) CreateAgentAccess(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("site_id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid publisher id"})
		return
	}
	key, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key generation failed"})
		return
	}
	key = "agent_" + key
	id, err := h.DB.CreateAgentAccess(c.Request.Context(), c.Param("site_id"), services.HashAPIKey(key))
	if err != nil {
		writeNamedConflict(c, "publisher_missing")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "api_key": key, "expires_in_days": 7, "scope": "opportunities, placements and revenue reads; no owner approvals, payout confirmation or legacy joins"})
}

func (h *PlacementsHandler) RevokeAgentAccess(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid access id"})
		return
	}
	updated, err := h.DB.RevokeAgentAccess(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "access revocation failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "active access not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func hostWithoutPort(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	return strings.Trim(strings.TrimSuffix(host, "."), "[]")
}

func localPublisherHost(host string) bool {
	host = hostWithoutPort(host)
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}

func registeredHostPort(domain string) (host, port string) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	name, p, ok := splitOptionalPort(domain)
	if !ok {
		return strings.Trim(domain, "[]"), ""
	}
	return strings.Trim(name, "[]"), p
}

func surfaceURLProblem(raw, siteDomain string) string {
	const registered = "surface_url must be an HTTPS article on the registered publisher domain"
	const httpsRule = "surface_url must use HTTPS. HTTP is allowed only when the registered site is 127.0.0.1, ::1, localhost, or a name ending in .localhost."
	if len(raw) > 2048 {
		return registered
	}
	u, err := url.Parse(raw)
	host, port := registeredHostPort(siteDomain)
	if err != nil || u.Hostname() == "" || u.User != nil || !strings.EqualFold(u.Hostname(), host) || (port != "" && u.Port() != port) {
		return registered
	}
	if u.Scheme == "https" {
		return ""
	}
	if u.Scheme == "http" && localPublisherHost(siteDomain) {
		return ""
	}
	if u.Scheme == "http" {
		return httpsRule
	}
	return registered
}

func (h *PlacementsHandler) ArticleText(c *gin.Context) {
	var req struct {
		SurfaceURL string `json:"surface_url"`
	}
	if bindBoundedJSON(c, &req) != nil || strings.TrimSpace(req.SurfaceURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "surface_url is required"})
		return
	}
	if !h.requireLinkedPublisher(c) {
		return
	}
	site, err := h.DB.GetSiteByID(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "publisher lookup failed"})
		return
	}
	if problem := surfaceURLProblem(req.SurfaceURL, site.Domain); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}
	text, err := fetchArticleText(c.Request.Context(), req.SurfaceURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "The article text could not be fetched. Paste a longer excerpt instead."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}

var articleScriptStyle = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
var articleTags = regexp.MustCompile(`(?s)<[^>]+>`)

func stripArticleHTML(raw string) string {
	raw = articleScriptStyle.ReplaceAllString(raw, " ")
	raw = articleTags.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	return strings.TrimSpace(strings.Join(strings.Fields(raw), " "))
}

func fetchArticleText(ctx context.Context, raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if req.URL.Hostname() != host || req.URL.User != nil {
				return errors.New("redirect left the article host")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "OfferRails/article-text")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("article fetch status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return "", err
	}
	if len(body) > 1<<20 {
		return "", errors.New("article is too large")
	}
	text := stripArticleHTML(string(body))
	runes := []rune(text)
	if len(runes) > 12000 {
		text = string(runes[:12000])
	}
	if len(strings.TrimSpace(text)) < 10 {
		return "", errors.New("article text is too short")
	}
	return text, nil
}

func bindBoundedJSON(c *gin.Context, out interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	return c.ShouldBindJSON(out)
}
