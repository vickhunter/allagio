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
	DB       *database.DB
	Cipher   *services.ConnectionCipher
	Verifier services.StripeConnectionVerifier
	BaseURL  string
	Page     *template.Template
}

func (h *MerchantConnectionsHandler) Form(c *gin.Context) {
	if h.Cipher == nil || h.Verifier == nil {
		c.Status(http.StatusServiceUnavailable)
		(&HostedHandler{Page: h.Page, BaseURL: h.BaseURL}).render(c, "Payment connections are not enabled", "The service operator must finish the payment configuration first.", nil)
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
	body := template.HTML(fmt.Sprintf(`<p>Connect Stripe for %s. Use a restricted key with account, price and webhook endpoint read permissions. The API key is used once for verification and is not stored.</p><p>Create an endpoint in Stripe for <code>%s/v1/webhooks/stripe/%s</code> with checkout.session.completed, checkout.session.async_payment_succeeded and charge.refunded events.</p><form method="post"><input type="hidden" name="csrf" value="%s"><label>Restricted API key <input type="password" name="stripe_key" autocomplete="off" required></label><label>One-time price ID <input name="price_id" required></label><label>Webhook endpoint ID <input name="endpoint_id" required></label><label>Webhook signing secret <input type="password" name="signing_secret" autocomplete="off" required></label><button>Verify and connect Stripe</button></form><p>Configuration verification is followed by an attributed test checkout and refund before a real pilot. Subscription billing is outside this first pilot.</p>`, template.HTMLEscapeString(p.Name), template.HTMLEscapeString(h.BaseURL), p.ID, template.HTMLEscapeString(o.CSRF)))
	(&HostedHandler{Page: h.Page, BaseURL: h.BaseURL}).render(c, "Connect your checkout", "", body)
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
	if err != nil || p.PaymentProvider != "stripe" {
		c.Status(404)
		return
	}
	secret := c.PostForm("signing_secret")
	if !strings.HasPrefix(secret, "whsec_") || len(secret) < 16 || len(secret) > 512 {
		c.JSON(400, gin.H{"error": "valid webhook signing secret required"})
		return
	}
	check, err := h.Verifier.Verify(c.Request.Context(), c.PostForm("stripe_key"), c.PostForm("price_id"), c.PostForm("endpoint_id"), h.BaseURL+"/v1/webhooks/stripe/"+productID)
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
	c.Status(201)
	(&HostedHandler{Page: h.Page, BaseURL: h.BaseURL}).render(c, "Stripe configuration verified", "A delivered payment event must still prove the signing secret and checkout attribution.", template.HTML(fmt.Sprintf(`<p>Account: %s · Price: %s · Currency: %s · Live mode: %t</p><p>The verification key was not stored. The webhook signing secret was encrypted.</p><p><a href="/owner/products">Return to your products</a></p>`, template.HTMLEscapeString(check.AccountID), template.HTMLEscapeString(check.PriceID), template.HTMLEscapeString(check.Currency), check.LiveMode)))
}
