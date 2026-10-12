package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const pluginKeySessionError = "plugin keys can only be managed from a signed-in dashboard session"

// ownerSessionLifetime matches the eight-hour owner_sessions insert.
// Plugin-key routes ask for a fresh sign-in after pluginKeyReauthAfter.
const (
	ownerSessionLifetime = 8 * time.Hour
	pluginKeyReauthAfter = 15 * time.Minute
)

func writePluginKeyError(c *gin.Context, status int, reason, message, compat string) {
	c.JSON(status, gin.H{"error": compat, "reason": reason, "message": message})
}

func pluginKeySessionAge(expires time.Time) time.Duration {
	remaining := time.Until(expires)
	if remaining >= ownerSessionLifetime {
		return 0
	}
	if remaining <= 0 {
		return ownerSessionLifetime
	}
	return ownerSessionLifetime - remaining
}

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
		writePluginKeyError(c, http.StatusBadRequest, "invalid_site_id", "The site id is not valid.", "invalid site id")
		return "", false
	}
	if credentialPresented(c) {
		writePluginKeyError(c, http.StatusForbidden, "forbidden_key_type", "A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.", pluginKeySessionError)
		return "", false
	}
	token, err := c.Cookie(ownerCookieName(h.BaseURL))
	if err != nil || token == "" {
		writePluginKeyError(c, http.StatusUnauthorized, "reauth_required", "Sign in again to manage plugin keys.", "sign in at /auth/login")
		return "", false
	}
	owner, expires, err := h.DB.OwnerSessionExpiry(c.Request.Context(), services.HashAPIKey(token))
	if err != nil || owner == nil || owner.ID == "" {
		writePluginKeyError(c, http.StatusUnauthorized, "reauth_required", "Sign in again to manage plugin keys.", "sign in at /auth/login")
		return "", false
	}
	if pluginKeySessionAge(expires) > pluginKeyReauthAfter {
		writePluginKeyError(c, http.StatusUnauthorized, "reauth_required", "This session is too old. Sign in again to manage plugin keys.", "sign in at /auth/login")
		return "", false
	}
	if !h.confirmPluginKeyWrite(c, owner.CSRF) {
		return "", false
	}
	if owner.SiteID == "" || owner.SiteID != c.Param("id") {
		writePluginKeyError(c, http.StatusForbidden, "forbidden", pluginKeySessionError, pluginKeySessionError)
		return "", false
	}
	return owner.SiteID, true
}

func (h *AgentEntry) confirmPluginKeyWrite(c *gin.Context, csrf string) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return true
	}
	token := c.GetHeader("X-CSRF-Token")
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if c.Request.ParseForm() != nil {
			writePluginKeyError(c, http.StatusBadRequest, "invalid_payload", "The request could not be read.", "invalid payload")
			return false
		}
		token = c.PostForm("csrf")
	}
	if c.GetHeader("Origin") != h.BaseURL || subtle.ConstantTimeCompare([]byte(token), []byte(csrf)) != 1 {
		writePluginKeyError(c, http.StatusForbidden, "invalid_confirmation", "Confirm this request from the signed-in dashboard.", "invalid owner confirmation")
		return false
	}
	return true
}

func (h *AgentEntry) IssuePluginKey(c *gin.Context) {
	siteID, ok := h.sessionSite(c)
	if !ok {
		return
	}
	raw, issued, err := mintPluginKey(c.Request.Context(), h.DB, siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		writePluginKeyError(c, http.StatusNotFound, "site_not_found", "The site was not found.", "site not found")
		return
	}
	if err != nil {
		writePluginKeyError(c, http.StatusInternalServerError, "plugin_key_failed", "The plugin key was not created.", "plugin key was not created")
		return
	}
	if !issued {
		writePluginKeyError(c, http.StatusConflict, "key_exists", "A plugin key already exists.", "a plugin key already exists")
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
		writePluginKeyError(c, http.StatusInternalServerError, "plugin_key_failed", "Plugin keys could not be loaded.", "plugin keys unavailable")
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
		writePluginKeyError(c, http.StatusInternalServerError, "plugin_key_failed", "The plugin key was not created.", "plugin key was not created")
		return
	}
	err = h.DB.RotatePluginKey(c.Request.Context(), siteID, services.HashAPIKey(raw), services.KeyPrefix(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		writePluginKeyError(c, http.StatusNotFound, "site_not_found", "The site was not found.", "site not found")
		return
	}
	if err != nil {
		writePluginKeyError(c, http.StatusInternalServerError, "plugin_key_failed", "The plugin key was not created.", "plugin key was not created")
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
		writePluginKeyError(c, http.StatusInternalServerError, "plugin_key_failed", "The plugin key was not revoked.", "plugin key was not revoked")
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": n > 0})
}
