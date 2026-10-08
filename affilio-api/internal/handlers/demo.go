package handlers

import (
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DemoCheckoutURL is empty on the shipped server. A checkout link is rendered
// only when a test sets this to a placeholder URL. There is no secret env for it.
func (h *HostedHandler) mountDemo(public *gin.RouterGroup) {
	public.GET("/demo", func(c *gin.Context) { h.writeDemo(c, "") })
	public.GET("/demo/:page", func(c *gin.Context) { h.writeDemo(c, c.Param("page")) })
}

func (h *HostedHandler) writeDemo(c *gin.Context, page string) {
	checkout := ""
	if raw := strings.TrimSpace(h.DemoCheckoutURL); raw != "" {
		checkout = `<p><a href="` + html.EscapeString(raw) + `">Checkout</a></p>`
	}
	var body string
	switch page {
	case "":
		body = `<h1>Offer Rails demo</h1><p>A sample product for the readiness check.</p><p><a href="/demo/start">Get started</a></p><p><a href="/demo/pricing">Pricing</a> <a href="/demo/docs">Docs</a> <a href="/demo/privacy">Privacy</a> <a href="/demo/terms">Terms</a></p>` + checkout
	case "start":
		body = `<h1>Get started</h1><p>Create an account for the demo product.</p>`
	case "pricing":
		body = `<h1>Pricing</h1><p>Pro plan: $19/month</p><p><a href="/demo/start">Get started</a></p>` + checkout
	case "docs":
		body = `<h1>Documentation</h1><p>` + strings.Repeat("This demo product explains how a maker prepares a public site for distribution. ", 20) + `</p>`
	case "privacy":
		body = `<h1>Privacy</h1><p>The demo product stores no personal data.</p>`
	case "terms":
		body = `<h1>Terms</h1><p>The demo product is a sample and not a paid service.</p>`
	default:
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!DOCTYPE html><html><body>"+body+"</body></html>"))
}
