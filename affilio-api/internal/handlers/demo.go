package handlers

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var demoAttributionParam = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const demoStyle = `<style>:root{--paper:#f7f7ef;--panel:#fdfdf7;--ink:#232720;--muted:#697063;--line:#dde0d3;--accent:#c64f38;--action:#2352c9;--action-ink:#f4f3ef;--font-sans:"Avenir Next",Avenir,"Segoe UI",sans-serif}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font-family:var(--font-sans);font-size:16px;line-height:1.5;overflow-wrap:anywhere}main{max-width:40rem;margin:0 auto;padding:28px 18px 48px}h1{font-size:clamp(32px,4vw,48px);line-height:1.1;letter-spacing:-.02em;margin:0 0 12px}.demo-flag{display:inline-flex;align-items:center;margin:0 0 16px;padding:4px 10px;border:1px solid var(--accent);border-radius:999px;color:#a8402c;font-size:12px;font-weight:700;letter-spacing:.08em}.sub{color:var(--muted);margin:0 0 16px}.note{margin:0 0 16px}nav.demo-nav{display:flex;flex-wrap:nowrap;align-items:stretch;width:180px;max-width:180px;margin:0 0 16px}nav.demo-nav a{flex:1 0 44px;width:45px;min-width:44px;max-width:45px;justify-content:center;overflow:hidden;padding:8px 0;text-align:center}a{color:var(--ink);display:inline-flex;align-items:center;min-width:44px;min-height:44px;padding:8px 2px}a.cta{background:var(--action);color:var(--action-ink);text-decoration:none;border-radius:5px;padding:12px 18px;min-width:44px;font-weight:600}</style>`

const demoTagline = "Demo landing page builder: launch a landing page without code."

const demoDocsCopy = "Demo landing page builder. Templates, lead capture, and email signup are the sample features. A template is a starting layout for a page. Lead capture is a form on that page. Email signup is the list the form would join. This demo does not send mail, and it does not store an address. These pages stay public so a readiness check can open a landing page, a pricing page, and this documentation. Pricing says there is no charge. Privacy and terms say the demo stores no personal data. Get started opens a sign-in screen for the demo and does not create an account. Nothing here is a customer count, a rating, or a paid plan. The label DEMO means the page is a sample, not a product you can buy."

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

func mergeDemoReferral(returnTo, ref, via string) string {
	u, err := url.Parse(returnTo)
	if err != nil || u == nil {
		return returnTo
	}
	q := u.Query()
	changed := false
	if demoAttributionParam.MatchString(ref) {
		q.Set("ref", ref)
		changed = true
	}
	if demoAttributionParam.MatchString(via) {
		q.Set("via", via)
		changed = true
	}
	if !changed {
		return returnTo
	}
	u.RawQuery = q.Encode()
	return u.RequestURI()
}

func demoHref(path, query string) string {
	if query != "" {
		path += "?" + query
	}
	return html.EscapeString(path)
}

func demoFlag() string {
	return `<p class="demo-flag">DEMO</p>`
}

func demoDocument(title, description, body string) []byte {
	return []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex"><title>` + html.EscapeString(title) + `</title><meta name="description" content="` + html.EscapeString(description) + `">` + demoStyle + `</head><body><main>` + body + `</main></body></html>`)
}

func (h *HostedHandler) writeDemo(c *gin.Context, page string) {
	c.Header("X-Robots-Tag", "noindex")
	checkout := ""
	if raw := strings.TrimSpace(h.DemoCheckoutURL); raw != "" {
		checkout = `<p><a href="` + html.EscapeString(raw) + `">Checkout</a></p>`
	}
	query := demoAttribution(c)
	back := `<p><a href="` + demoHref("/demo", query) + `">Back to the demo</a></p>`
	start := `<p><a class="cta" href="` + demoHref("/demo/start", query) + `">Get started</a></p>`
	login := demoHref("/auth/login", query)
	links := `<nav class="demo-nav" aria-label="Demo"><a href="` + demoHref("/demo/pricing", query) + `">Pricing</a><a href="` + demoHref("/demo/docs", query) + `">Docs</a><a href="` + demoHref("/demo/privacy", query) + `">Privacy</a><a href="` + demoHref("/demo/terms", query) + `">Terms</a></nav>`
	var title, description, body string
	switch page {
	case "":
		title = "Demo landing page builder"
		description = demoTagline
		body = demoFlag() + `<h1>Demo landing page builder</h1><p class="sub">` + demoTagline + `</p><p class="note">Sample features: templates, lead capture, and email signup. This page is a demo, not a product you can buy.</p>` + start + links + checkout
	case "start":
		title = "Get started · Demo landing page builder"
		description = "Demo landing page builder. Sign in for the demo."
		body = demoFlag() + `<h1>Get started</h1><p class="sub">Demo landing page builder. This screen only points at sign-in for the demo. It does not create an account.</p>` + back + `<p><a href="` + login + `">Sign in</a></p><p><a href="` + login + `">Register</a></p>`
	case "pricing":
		title = "Pricing · Demo landing page builder"
		description = "Demo landing page builder. Demo only: there is no real price."
		body = demoFlag() + `<h1>Pricing</h1><p class="sub">Demo landing page builder. Templates, lead capture, and email signup are the sample features.</p><p class="note">Demo only. There is no real price and nothing to pay.</p>` + start + back + checkout
	case "docs":
		title = "Documentation · Demo landing page builder"
		description = "Demo landing page builder. Templates, lead capture, and email signup."
		body = demoFlag() + `<h1>Documentation</h1><p class="sub">` + demoDocsCopy + `</p>` + back
	case "privacy":
		title = "Privacy · Demo landing page builder"
		description = "Demo landing page builder. This demo stores no personal data."
		body = demoFlag() + `<h1>Privacy</h1><p class="sub">Demo landing page builder. This demo stores no personal data. It does not collect names, emails, or addresses.</p>` + back
	case "terms":
		title = "Terms · Demo landing page builder"
		description = "Demo landing page builder. This demo is not a paid service."
		body = demoFlag() + `<h1>Terms</h1><p class="sub">Demo landing page builder. This demo is not a paid service and not a contract.</p>` + back
	default:
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument(title, description, body))
}
