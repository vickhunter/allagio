package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const pluginKeySessionError = "plugin keys can only be managed from a signed-in dashboard session"

// mintPluginKey stores a new plugin key when the site has none active.
// The raw value is returned only when issued is true.
func mintPluginKey(ctx context.Context, db *database.DB, siteID string) (raw string, issued bool, err error) {
	raw, err = generateAPIKey()
	if err != nil {
		return "", false, err
	}
	issued, err = db.IssuePluginKeyIfAbsent(ctx, siteID, services.HashAPIKey(raw), services.KeyPrefix(raw))
	if err != nil || !issued {
		return "", false, err
	}
	return raw, true, nil
}

func (h *AgentEntry) sessionSite(c *gin.Context) (string, bool) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return "", false
	}
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": pluginKeySessionError})
		return "", false
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return "", false
	}
	if owner.SiteID == "" || owner.SiteID != c.Param("id") {
		c.JSON(http.StatusForbidden, gin.H{"error": pluginKeySessionError})
		return "", false
	}
	return owner.SiteID, true
}

func (h *AgentEntry) IssuePluginKey(c *gin.Context) {
	siteID, ok := h.sessionSite(c)
	if !ok {
		return
	}
	raw, issued, err := mintPluginKey(c.Request.Context(), h.DB, siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin key was not created"})
		return
	}
	if !issued {
		c.JSON(http.StatusConflict, gin.H{"error": "a plugin key already exists"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"plugin_key": raw, "key_prefix": services.KeyPrefix(raw)})
}

func (h *AgentEntry) ListPluginKeys(c *gin.Context) {
	siteID, ok := h.sessionSite(c)
	if !ok {
		return
	}
	rows, err := h.DB.ListPluginKeys(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin keys unavailable"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		item := gin.H{
			"id": row.ID, "key_prefix": row.Prefix, "created_at": row.CreatedAt.UTC().Format(time.RFC3339), "revoked": row.RevokedAt != nil,
		}
		if row.RevokedAt != nil {
			item["revoked_at"] = row.RevokedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"keys": out})
}

func (h *AgentEntry) RotatePluginKey(c *gin.Context) {
	siteID, ok := h.sessionSite(c)
	if !ok {
		return
	}
	raw, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin key was not created"})
		return
	}
	err = h.DB.RotatePluginKey(c.Request.Context(), siteID, services.HashAPIKey(raw), services.KeyPrefix(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin key was not created"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plugin_key": raw, "key_prefix": services.KeyPrefix(raw)})
}

func (h *AgentEntry) RevokePluginKey(c *gin.Context) {
	siteID, ok := h.sessionSite(c)
	if !ok {
		return
	}
	n, err := h.DB.RevokeActivePluginKeys(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin key was not revoked"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": n > 0})
}
