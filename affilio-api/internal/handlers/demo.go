package handlers

import (
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var demoAttributionParam = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// DemoCheckoutURL is empty on the shipped server. A checkout link is rendered
// only when a test sets this to a placeholder URL. There is no secret env for it.
func (h *HostedHandler) mountDemo(public *gin.RouterGroup) {
	public.GET("/demo", func(c *gin.Context) { h.writeDemo(c, "") })
	public.HEAD("/demo", func(c *gin.Context) { h.writeDemo(c, "") })
	public.GET("/demo/:page", func(c *gin.Context) { h.writeDemo(c, c.Param("page")) })
	public.HEAD("/demo/:page", func(c *gin.Context) { h.writeDemo(c, c.Param("page")) })
}

func demoAttribution(c *gin.Context) string {
	values := make([]string, 0, 2)
	for _, key := range []string{"ref", "via"} {
		raw := c.Query(key)
		if demoAttributionParam.MatchString(raw) {
			values = append(values, key+"="+raw)
		}
	}
	return strings.Join(values, "&")
}

func demoHref(path, query string) string {
	if query != "" {
		path += "?" + query
	}
	return html.EscapeString(path)
}

func demoDocument(title, body string) []byte {
	return []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + html.EscapeString(title) + `</title><style>body{margin:0;padding:24px;font-family:system-ui,sans-serif;font-size:16px;line-height:1.5;overflow-wrap:anywhere}a.cta{display:inline-flex;align-items:center;justify-content:center;min-width:44px;min-height:44px;padding:12px 18px;box-sizing:border-box}</style></head><body>` + body + `</body></html>`)
}

func (h *HostedHandler) writeDemo(c *gin.Context, page string) {
	checkout := ""
	if raw := strings.TrimSpace(h.DemoCheckoutURL); raw != "" {
		checkout = `<p><a href="` + html.EscapeString(raw) + `">Checkout</a></p>`
	}
	query := demoAttribution(c)
	back := `<p><a href="` + demoHref("/demo", query) + `">Back to the demo</a></p>`
	start := `<p><a class="cta" href="` + demoHref("/demo/start", query) + `">Get started</a></p>`
	var title, body string
	switch page {
	case "":
		title = "Offer Rails demo"
		body = `<h1>Offer Rails demo</h1><p>A sample product for the readiness check.</p>` + start + `<p><a href="` + demoHref("/demo/pricing", query) + `">Pricing</a> <a href="` + demoHref("/demo/docs", query) + `">Docs</a> <a href="` + demoHref("/demo/privacy", query) + `">Privacy</a> <a href="` + demoHref("/demo/terms", query) + `">Terms</a></p>` + checkout
	case "start":
		title = "Get started · Offer Rails"
		body = `<h1>Get started</h1><p>Create an account for the demo product.</p>` + back + `<p><a href="/auth/login">Sign in</a></p><p><a href="/auth/login">Register</a></p>`
	case "pricing":
		title = "Pricing · Offer Rails"
		body = `<h1>Pricing</h1><p>Pro plan: $19/month</p>` + start + back + checkout
	case "docs":
		title = "Documentation · Offer Rails"
		body = `<h1>Documentation</h1><p>` + strings.Repeat("This demo product explains how a maker prepares a public site for distribution. ", 20) + `</p>` + back
	case "privacy":
		title = "Privacy · Offer Rails"
		body = `<h1>Privacy</h1><p>The demo product stores no personal data.</p>` + back
	case "terms":
		title = "Terms · Offer Rails"
		body = `<h1>Terms</h1><p>The demo product is a sample and not a paid service.</p>` + back
	default:
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument(title, body))
}
