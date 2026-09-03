package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type SiteKeysHandler struct {
	DB          *database.DB
	Audit       *services.AuditLogger
	GracePeriod time.Duration
}

type rotateKeyRequest struct {
	SiteID string `json:"site_id"`
	Actor  string `json:"actor"`
	Note   string `json:"note"`
}

func (h *SiteKeysHandler) Rotate(c *gin.Context) {
	var req rotateKeyRequest
	_ = c.ShouldBindJSON(&req)
	if req.SiteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id required"})
		return
	}
	newKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate key"})
		return
	}
	if req.Actor == "" {
		req.Actor = "admin"
	}
	newKeyHash := services.HashAPIKey(newKey)
	newKeyPrefix := services.KeyPrefix(newKey)
	gracePeriod := h.GracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 24 * time.Hour
	}
	if err := h.DB.RotateSiteAPIKey(c.Request.Context(), req.SiteID, newKeyHash, newKeyPrefix, gracePeriod); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rotation failed"})
		return
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO site_key_rotations (site_id, old_key_prefix, new_key_prefix, rotated_by, note)
SELECT id, api_key_prev_prefix, api_key_prefix, $2, $3
FROM sites
WHERE id=$1`, req.SiteID, req.Actor, req.Note)

	if h.Audit != nil {
		h.Audit.Log(database.AuditEvent{
			EventType: "key_rotation",
			Severity:  "info",
			Actor:     req.Actor,
			IPAddress: strings.TrimSpace(c.ClientIP()),
			Endpoint:  c.FullPath(),
			Method:    c.Request.Method,
			Details:   map[string]interface{}{"site_id": req.SiteID, "note": req.Note},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"site_id":        req.SiteID,
		"api_key":        newKey,
		"rotated_at":     time.Now().UTC().Format(time.RFC3339),
		"rotated_by":     req.Actor,
		"grace_period_h": int(gracePeriod.Hours()),
		"note":           "previous key valid for grace period",
	})
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
