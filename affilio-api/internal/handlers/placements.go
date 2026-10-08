package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type PlacementsHandler struct {
	DB                        *database.DB
	RequireVerifiedConnection bool
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

const relevanceThreshold = 25

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
		Context string `json:"context"`
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

func (h *PlacementsHandler) Prepare(c *gin.Context) {
	var req struct {
		OfferID    string `json:"offer_id"`
		SurfaceURL string `json:"surface_url"`
		Context    string `json:"context"`
		Copy       string `json:"copy"`
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
	u, err := url.Parse(req.SurfaceURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != site.Domain || u.User != nil || len(req.SurfaceURL) > 2048 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "surface_url must be an HTTPS article on the registered publisher domain"})
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
		c.JSON(http.StatusConflict, gin.H{"error": "offer is not eligible or relevant to this content"})
		return
	}
	p, err := h.DB.CreatePlacement(c.Request.Context(), database.Placement{SiteID: site.ID, OfferID: req.OfferID, SurfaceURL: req.SurfaceURL,
		Context: req.Context, Copy: strings.TrimSpace(req.Copy), Disclosure: "Affiliate disclosure: I may earn a commission if you buy through this link.", Reason: reason, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "offer unavailable"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"placement": p, "next_action": "Owner reviews exact copy, disclosure, surface and terms; pilot operator records their approval."})
}

func (h *PlacementsHandler) Get(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	p, err := h.DB.GetPlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"placement": p})
}

func (h *PlacementsHandler) List(c *gin.Context) {
	p, err := h.DB.ListPlacements(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task inbox unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"placements": p})
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
	p, err := h.DB.CommitPlacement(c.Request.Context(), siteIDFrom(c), c.Param("id"), code)
	if errors.Is(err, database.ErrPlacementAuthority) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
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
	c.JSON(http.StatusOK, gin.H{"placement": p, "next_action": "Use the publisher's existing publishing tool for approved copy, tracked link and disclosure, then record the receipt."})
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
		c.JSON(http.StatusConflict, gin.H{"error": "placement unavailable, expired or self referral"})
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
	if json.Unmarshal(p.Receipt, &receipt) != nil || receipt.TrackedLink == "" || !strings.Contains(req.PublishedText, p.Copy) || !publishedDisclosureOK(req.PublishedText, p.Disclosure) || !strings.Contains(req.PublishedText, receipt.TrackedLink) {
		c.JSON(http.StatusConflict, gin.H{"error": "published text must include approved copy, disclosure and exact tracked link"})
		return
	}
	hash := sha256.Sum256([]byte(req.PublishedText))
	updated, err := h.DB.RecordPlacementReceipt(c.Request.Context(), siteIDFrom(c), p.ID, req.SurfaceURL, hex.EncodeToString(hash[:]), req.Evidence)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "receipt recording failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusConflict, gin.H{"error": "receipt already recorded or placement unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "publisher_reported", "content_sha256": hex.EncodeToString(hash[:])})
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
		c.JSON(http.StatusConflict, gin.H{"error": "publisher not found"})
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

func bindBoundedJSON(c *gin.Context, out interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	return c.ShouldBindJSON(out)
}
