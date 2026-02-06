package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type SiteKeysHandler struct {
	DB *database.DB
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
	if err := h.DB.RotateSiteAPIKey(c.Request.Context(), req.SiteID, newKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rotation failed"})
		return
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO site_key_rotations (site_id, old_key, new_key, rotated_by, note)
SELECT id, api_key_previous, api_key, $2, $3
FROM sites
WHERE id=$1`, req.SiteID, req.Actor, req.Note)
	c.JSON(http.StatusOK, gin.H{
		"site_id":    req.SiteID,
		"api_key":    newKey,
		"rotated_at": time.Now().UTC().Format(time.RFC3339),
		"rotated_by": req.Actor,
		"note":       "previous key remains valid until manually revoked",
	})
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
