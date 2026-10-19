package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type MerchantConnectionsHandler struct {
	DB         *database.DB
	Cipher     *services.ConnectionCipher
	Verifier   services.StripeConnectionVerifier
	BaseURL    string
	Page       *template.Template
	OperatorID string
}

func (h *MerchantConnectionsHandler) viewer() *HostedHandler {
	return &HostedHandler{Page: h.Page, BaseURL: h.BaseURL, OperatorID: h.OperatorID}
}

const webhookSecretRefusal = "Use the signing secret from your Stripe webhook endpoint. It starts with whsec_."

func validWebhookSecret(secret string) bool {
	return strings.HasPrefix(secret, "whsec_") && len(secret) >= 16 && len(secret) <= 512
}

func liveKeyRejected(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.HasPrefix(key, "sk_live_") || strings.HasPrefix(key, "rk_live_") || strings.HasPrefix(key, "pk_live_")
}

func blocksStripeConnect(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "", "none", "stripe":
		return false
	default:
		return true
	}
}

func (h *MerchantConnectionsHandler) connectionBody(name, productID, csrf, refusal string) template.HTML {
	refusalHTML := ""
	if refusal != "" {
		refusalHTML = `<p class="field-error" id="stripe-live-key" role="alert" tabindex="-1">` + template.HTMLEscapeString(refusal) + `</p>`
	}
	return template.HTML(fmt.Sprintf(`<p>This page is for Stripe test mode only. Live keys (sk_live_ and rk_live_) are rejected.</p><p>Connect Stripe for %s. Use a restricted key with account, price and webhook endpoint read permissions. The API key is used once for verification and is not stored.</p><p>Create an endpoint in Stripe for <code class="webhook-url">%s/v1/webhooks/stripe/%s</code> with checkout.session.completed, checkout.session.async_payment_succeeded and charge.refunded events.</p><form method="post"><input type="hidden" name="csrf" value="%s"><label>Restricted API key <input type="password" name="stripe_key" autocomplete="off" required></label><label>One-time price ID <input name="price_id" required></label><label>Webhook endpoint ID <input name="endpoint_id" required></label><label>Webhook signing secret <input type="password" name="signing_secret" autocomplete="off" required></label><button>Verify and connect Stripe</button>%s</form><p>Configuration verification is followed by an attributed test checkout and refund. Subscription billing is outside this draft.</p>`, template.HTMLEscapeString(name), template.HTMLEscapeString(h.BaseURL), template.HTMLEscapeString(productID), template.HTMLEscapeString(csrf), refusalHTML))
}

func (h *MerchantConnectionsHandler) Form(c *gin.Context) {
	if h.Cipher == nil || h.Verifier == nil {
		c.Status(http.StatusServiceUnavailable)
		h.viewer().render(c, "Payment connections are not enabled", "The service operator must finish the payment configuration first.", nil)
		return
	}
	o := ownerFrom(c)
	if o.MakerID == "" || !uuidPattern.MatchString(c.Param("id")) {
		c.Status(404)
		return
	}
	p, err := h.DB.GetProduct(c.Request.Context(), o.MakerID, c.Param("id"))
	if err != nil {
		c.Status(404)
		return
	}
	h.viewer().render(c, "Connect your checkout", "", h.connectionBody(p.Name, p.ID, o.CSRF, ""))
}

func (h *MerchantConnectionsHandler) Connect(c *gin.Context) {
	o := ownerFrom(c)
	productID := c.Param("id")
	if h.Cipher == nil || h.Verifier == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stripe verification is not configured"})
		return
	}
	if o.MakerID == "" || !uuidPattern.MatchString(productID) {
		c.Status(404)
		return
	}
	p, err := h.DB.GetProduct(c.Request.Context(), o.MakerID, productID)
	if err != nil || blocksStripeConnect(p.PaymentProvider) {
		c.Status(404)
		return
	}
	key := c.PostForm("stripe_key")
	if liveKeyRejected(key) {
		c.Status(http.StatusBadRequest)
		h.viewer().render(c, "Connect your checkout", "", h.connectionBody(p.Name, productID, o.CSRF, "Stripe test mode only. Live keys are not accepted."))
		return
	}
	secret := c.PostForm("signing_secret")
	if !validWebhookSecret(secret) {
		c.Status(http.StatusBadRequest)
		h.viewer().render(c, "Connect your checkout", "", h.connectionBody(p.Name, productID, o.CSRF, webhookSecretRefusal))
		return
	}
	check, err := h.Verifier.Verify(c.Request.Context(), key, c.PostForm("price_id"), c.PostForm("endpoint_id"), h.BaseURL+"/v1/webhooks/stripe/"+productID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	encrypted, err := h.Cipher.Encrypt(productID, secret)
	if err != nil {
		c.Status(500)
		return
	}
	created, err := h.DB.CreateVerifiedProductConnection(c.Request.Context(), o.MakerID, database.ProductConnection{ProductID: productID, SecretCiphertext: encrypted, LiveMode: check.LiveMode, Currency: check.Currency}, check.AccountID, check.PriceID, check.EndpointID)
	if err != nil {
		c.Status(500)
		return
	}
	if !created {
		c.JSON(409, gin.H{"error": "connection is disabled or its account, currency, mode or price differs"})
		return
	}
	c.Status(http.StatusCreated)
	h.viewer().render(c, "Stripe configuration verified", "A delivered payment event must still prove the signing secret and checkout attribution.", template.HTML(fmt.Sprintf(`<p>Account: %s · Price: %s · Currency: %s · Live mode: %t</p><p>The verification key was not stored. The webhook signing secret was encrypted.</p><p><a href="/owner/products">Return to your products</a></p>`, template.HTMLEscapeString(check.AccountID), template.HTMLEscapeString(check.PriceID), template.HTMLEscapeString(check.Currency), check.LiveMode)))
}
