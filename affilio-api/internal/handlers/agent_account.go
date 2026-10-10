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

const earningsNote = "Earnings are recorded test-mode commission only. Publisher amounts are ledger rows joined to test conversions. Maker amounts are test conversion commissions. Live and legacy rows are omitted. Null means no test-mode earnings are recorded for that scope. Click source is the referrer host; a click with no host counts as direct."

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
			"earnings": moneyJSON(project.Earnings), "clicks_by_source": sourceJSON(project.ClicksBySource),
			"items": items,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"totals": gin.H{
			"went_live":        digest.WentLive,
			"clicks":           digest.Clicks,
			"earnings":         accountEarningsJSON(digest.PublisherEarnings, digest.MakerEarnings),
			"clicks_by_source": sourceJSON(digest.ClicksBySource),
		},
		"projects":      projects,
		"earnings_note": earningsNote,
	})
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

func moneyJSON(amounts []database.MoneyAmount) any {
	if len(amounts) == 0 {
		return nil
	}
	out := make([]gin.H, 0, len(amounts))
	for _, amount := range amounts {
		out = append(out, gin.H{"currency": amount.Currency, "amount": amount.Amount})
	}
	return gin.H{"mode": "test", "amounts": out}
}

func accountEarningsJSON(publisher, maker []database.MoneyAmount) any {
	if len(publisher) == 0 && len(maker) == 0 {
		return nil
	}
	return gin.H{"mode": "test", "publisher": moneyJSON(publisher), "maker": moneyJSON(maker)}
}

func sourceJSON(items []database.SourceCount) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{"source": item.Source, "clicks": item.Clicks})
	}
	return out
}
