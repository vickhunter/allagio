package handlers

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type MakersHandler struct {
	DB              *database.DB
	RegisterLimiter *IPRateLimiter
}

type makerRegisterRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type makerRotateKeyRequest struct {
	MakerID string `json:"maker_id"`
	Actor   string `json:"actor"`
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func (h *MakersHandler) Register(c *gin.Context) {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	if h.RegisterLimiter != nil && !h.RegisterLimiter.Allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	var req makerRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Email == "" || len(req.Email) > 255 || !emailPattern.MatchString(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}
	if len(req.Name) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name too long"})
		return
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("makers.register key generation failed email=%s err=%v", req.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	makerID, err := h.DB.CreateMaker(c.Request.Context(), apiKey, req.Email, req.Name)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
		log.Printf("makers.register db create failed email=%s err=%v", req.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register maker"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"maker_id": makerID, "api_key": apiKey})
}

func (h *MakersHandler) RotateKey(c *gin.Context) {
	var req makerRotateKeyRequest
	_ = c.ShouldBindJSON(&req)
	req.MakerID = strings.TrimSpace(req.MakerID)
	if req.MakerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maker_id required"})
		return
	}
	newKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate key"})
		return
	}
	if err := h.DB.RotateMakerAPIKey(c.Request.Context(), req.MakerID, newKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rotation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"maker_id":   req.MakerID,
		"api_key":    newKey,
		"rotated_at": time.Now().UTC().Format(time.RFC3339),
		"note":       "previous key remains valid until manually revoked",
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
