package handlers

import (
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const earningsNote = "Earnings are recorded test-mode commission only. The number is null when that scope has no test-mode rows, and 0 when those rows add up to zero. Publisher amounts are ledger rows joined to test conversions. Maker amounts are test conversion commissions. Live and legacy rows are omitted. Click source is the referrer host; a click with no host counts as direct."

func (h *AgentEntry) HardChecks(c *gin.Context) {
	checks := entry.HardChecks()
	labels := entry.HardCheckLabels()
	out := make([]gin.H, 0, len(checks))
	for i := range checks {
		label := checks[i]
		if i < len(labels) {
			label = labels[i]
		}
		out = append(out, gin.H{"code": checks[i], "label": label})
	}
	c.JSON(http.StatusOK, gin.H{"checks": out})
}

func (h *AgentEntry) CreateAccountKey(c *gin.Context) {
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account keys can only be created from a signed-in dashboard session"})
		return
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	plain, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key was not created"})
		return
	}
	plain = "or_" + plain
	at := time.Now().UTC()
	id, err := h.DB.InsertAccountKey(c.Request.Context(), owner.ID, services.HashAPIKey(plain), services.KeyPrefix(plain), at)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key was not created"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": id, "key": plain, "key_prefix": services.KeyPrefix(plain), "kind": "account",
		"created_at": at.Format(time.RFC3339),
	})
}

func (h *AgentEntry) ListAccountKeys(c *gin.Context) {
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account keys can only be listed from a signed-in dashboard session"})
		return
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	rows, err := h.DB.ListAccountKeys(c.Request.Context(), owner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "keys unavailable"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		item := gin.H{"id": row.ID, "key_prefix": row.Prefix, "kind": "account", "created_at": row.CreatedAt.UTC().Format(time.RFC3339), "revoked": row.RevokedAt != nil}
		if row.RevokedAt != nil {
			item["revoked_at"] = row.RevokedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"keys": out})
}

func (h *AgentEntry) RevokeAccountKey(c *gin.Context) {
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account keys can only be revoked from a signed-in dashboard session"})
		return
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}
	found, err := h.DB.RevokeAccountKey(c.Request.Context(), owner.ID, c.Param("id"), time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key was not revoked"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "revoked": true})
}

func (h *AgentEntry) AccountProjects(c *gin.Context) {
	owner, ok := h.accountOwner(c)
	if !ok {
		return
	}
	projects, err := h.DB.AccountProjects(c.Request.Context(), owner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "projects unavailable"})
		return
	}
	out := make([]gin.H, 0, len(projects))
	for _, project := range projects {
		mode, err := h.projectMode(c, project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "projects unavailable"})
			return
		}
		out = append(out, gin.H{"kind": project.Kind, "id": project.ID, "name": project.Name, "mode": mode})
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

func (h *AgentEntry) AccountDigest(c *gin.Context) {
	owner, ok := h.accountOwner(c)
	if !ok {
		return
	}
	digest, err := h.DB.AccountDigest(c.Request.Context(), owner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	projects := make([]gin.H, 0, len(digest.Projects))
	for _, project := range digest.Projects {
		mode, err := h.projectMode(c, project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		project.Mode = mode
		items := make([]gin.H, 0, len(project.Items))
		for _, item := range project.Items {
			token, err := h.issueUndo(c, owner.ID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			items = append(items, digestJSON(item, token))
		}
		projects = append(projects, gin.H{
			"kind": project.Kind, "id": project.ID, "name": project.Name, "mode": project.Mode,
			"went_live": project.WentLive, "clicks": project.Clicks,
			"earnings": earningsNumber(project.Earnings), "clicks_by_source": sourceJSON(project.ClicksBySource),
			"items": items,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"totals": gin.H{
			"went_live":        digest.WentLive,
			"clicks":           digest.Clicks,
			"earnings":         combineEarnings(digest.PublisherEarnings, digest.MakerEarnings),
			"clicks_by_source": sourceJSON(digest.ClicksBySource),
		},
		"projects":      projects,
		"earnings_note": earningsNote,
		"session":       gin.H{"signed_in": !credentialPresented(c)},
	})
}

func (h *AgentEntry) AccountPause(c *gin.Context) {
	h.applyScope(c, true)
}

func (h *AgentEntry) AccountResume(c *gin.Context) {
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "resuming requires a signed-in dashboard session"})
		return
	}
	h.applyScope(c, false)
}

func (h *AgentEntry) applyScope(c *gin.Context, pause bool) {
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	owner, ok := h.scopeOwner(c, actor)
	if !ok {
		return
	}
	scope, id, ok := h.readScope(c)
	if !ok {
		return
	}
	targets, ok := h.scopeTargets(c, owner, scope, id)
	if !ok {
		return
	}
	applied := make([]gin.H, 0, len(targets))
	for _, target := range targets {
		row, err := h.applyOne(c, actor, target, pause)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scope was not updated"})
			return
		}
		applied = append(applied, row)
	}
	c.JSON(http.StatusOK, gin.H{"scope": scope, "id": id, "applied": applied})
}

type scopeTarget struct {
	Kind string
	ID   string
	Mode string
}

func (h *AgentEntry) scopeOwner(c *gin.Context, actor entryActor) (*database.Owner, bool) {
	if actor.Owner != nil {
		return actor.Owner, true
	}
	if actor.UserID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account scope required"})
		return nil, false
	}
	siteID, makerID, err := h.DB.LoadOwnerScope(c.Request.Context(), actor.UserID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "account scope required"})
		return nil, false
	}
	return &database.Owner{ID: actor.UserID, SiteID: siteID, MakerID: makerID}, true
}

func (h *AgentEntry) readScope(c *gin.Context) (string, string, bool) {
	body, err := readJSONBody(c, 1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", "", false
	}
	var req struct {
		Scope string `json:"scope"`
		ID    string `json:"id"`
	}
	if decodeStrict(body, &req) != nil || (req.Scope != "all" && req.Scope != "site" && req.Scope != "offer") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be all, site, or offer"})
		return "", "", false
	}
	if req.Scope == "all" && req.ID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope all takes no id"})
		return "", "", false
	}
	if req.Scope != "all" && !uuidPattern.MatchString(req.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return "", "", false
	}
	return req.Scope, req.ID, true
}

func (h *AgentEntry) scopeTargets(c *gin.Context, owner *database.Owner, scope, id string) ([]scopeTarget, bool) {
	switch scope {
	case "all":
		targets := []scopeTarget{}
		if owner.SiteID != "" {
			policy, _, err := h.sitePolicy(c, owner.SiteID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
				return nil, false
			}
			targets = append(targets, scopeTarget{Kind: "site", ID: owner.SiteID, Mode: policy.Mode})
		}
		if owner.MakerID != "" {
			ids, err := h.DB.MakerOfferIDs(c.Request.Context(), owner.MakerID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "offers unavailable"})
				return nil, false
			}
			for _, offerID := range ids {
				policy, _, err := h.offerPolicy(c, offerID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
					return nil, false
				}
				targets = append(targets, scopeTarget{Kind: "offer", ID: offerID, Mode: policy.Mode})
			}
		}
		return targets, true
	case "site":
		if owner.SiteID != id {
			c.JSON(http.StatusForbidden, gin.H{"error": "site is not on this account"})
			return nil, false
		}
		policy, _, err := h.sitePolicy(c, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
			return nil, false
		}
		return []scopeTarget{{Kind: "site", ID: id, Mode: policy.Mode}}, true
	case "offer":
		_, makerID, _, err := h.DB.OfferFacts(c.Request.Context(), id)
		if err != nil || makerID == "" || makerID != owner.MakerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "offer is not on this account"})
			return nil, false
		}
		policy, _, err := h.offerPolicy(c, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
			return nil, false
		}
		return []scopeTarget{{Kind: "offer", ID: id, Mode: policy.Mode}}, true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be all, site, or offer"})
		return nil, false
	}
}

func (h *AgentEntry) applyOne(c *gin.Context, actor entryActor, target scopeTarget, pause bool) (gin.H, error) {
	if pause {
		var n int
		var err error
		if target.Kind == "site" {
			n, err = h.DB.KillSiteSwitch(c.Request.Context(), target.ID, actor.Kind, actor.UserID, target.Mode)
		} else {
			n, err = h.DB.KillOfferSwitch(c.Request.Context(), target.ID, actor.Kind, actor.UserID, target.Mode)
		}
		if err != nil {
			return nil, err
		}
		return gin.H{"kind": target.Kind, "id": target.ID, "mode": "manual", "kill_switch": true, "paused": n}, nil
	}
	var mode string
	var n int
	var err error
	if target.Kind == "site" {
		mode, n, err = h.DB.ReleaseSiteKillSwitch(c.Request.Context(), target.ID, actor.Kind, actor.UserID)
	} else {
		mode, n, err = h.DB.ReleaseOfferKillSwitch(c.Request.Context(), target.ID, actor.Kind, actor.UserID)
	}
	if err != nil {
		return nil, err
	}
	return gin.H{"kind": target.Kind, "id": target.ID, "mode": mode, "kill_switch": false, "unpaused": n}, nil
}

func (h *AgentEntry) projectMode(c *gin.Context, project database.AccountProject) (string, error) {
	if project.Kind == "site" {
		policy, _, err := h.sitePolicy(c, project.ID)
		return policy.Mode, err
	}
	policy, _, err := h.offerPolicy(c, project.ID)
	return policy.Mode, err
}

func (h *AgentEntry) accountOwner(c *gin.Context) (*database.Owner, bool) {
	if credentialPresented(c) {
		owner, err := h.DB.ActiveAccountKey(c.Request.Context(), services.HashAPIKey(presentedKey(c)))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account scope required"})
			return nil, false
		}
		return &database.Owner{ID: owner.OwnerID, SiteID: owner.SiteID, MakerID: owner.MakerID}, true
	}
	return h.sessionOwner(c)
}

func credentialPresented(c *gin.Context) bool {
	return c.GetHeader("X-API-Key") != "" || c.GetHeader("X-Maker-Key") != "" || strings.TrimSpace(c.GetHeader("Authorization")) != ""
}

func presentedKey(c *gin.Context) string {
	if key := c.GetHeader("X-API-Key"); key != "" {
		return key
	}
	if key := c.GetHeader("X-Maker-Key"); key != "" {
		return key
	}
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	return strings.TrimPrefix(header, "Bearer ")
}

func earningsNumber(amounts []database.MoneyAmount) any {
	if len(amounts) == 0 {
		return nil
	}
	total := 0.0
	for _, amount := range amounts {
		total += amount.Amount
	}
	return total
}

func combineEarnings(parts ...[]database.MoneyAmount) any {
	seen := false
	total := 0.0
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		seen = true
		for _, amount := range part {
			total += amount.Amount
		}
	}
	if !seen {
		return nil
	}
	return total
}

func sourceJSON(items []database.SourceCount) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{"source": item.Source, "clicks": item.Clicks})
	}
	return out
}
