package handlers

import (
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type PaymentConnectionsHandler struct {
	DB     *database.DB
	Cipher *services.ConnectionCipher
}

func (h *PaymentConnectionsHandler) Create(c *gin.Context) {
	if h.Cipher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "connection encryption is not configured"})
		return
	}
	var req struct {
		Secret   string `json:"signing_secret"`
		LiveMode bool   `json:"live_mode"`
		Currency string `json:"currency"`
	}
	productID := c.Param("id")
	if !uuidPattern.MatchString(productID) || bindBoundedJSON(c, &req) != nil || !strings.HasPrefix(req.Secret, "whsec_") || len(req.Secret) < 16 || len(req.Secret) > 512 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid product and signing_secret required"})
		return
	}
	if req.Currency != "USD" && req.Currency != "EUR" && req.Currency != "GBP" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency must be USD, EUR or GBP"})
		return
	}
	ciphertext, err := h.Cipher.Encrypt(productID, req.Secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connection encryption failed"})
		return
	}
	created, err := h.DB.CreateProductConnection(c.Request.Context(), database.ProductConnection{ProductID: productID, SecretCiphertext: ciphertext, LiveMode: req.LiveMode, Currency: req.Currency})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connection failed"})
		return
	}
	if !created {
		c.JSON(http.StatusConflict, gin.H{"error": "Stripe product missing or already connected"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"product_id": productID, "live_mode": req.LiveMode, "currency": req.Currency, "webhook_path": "/v1/webhooks/stripe/" + productID})
}
