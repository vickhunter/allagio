package handlers

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

var demoAttributionParam = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// demoGoCode is the sample tracked link on the demo post. Opening it redirects
// and does not record a click.
const demoGoCode = "demo"

const (
	demoSampleDomain = "demo.staging.invalid"
	demoPendingCopy  = "Demo pending placement for the operator view."
	demoApprovedCopy = "Demo placement approved by the operator."
)

const demoStyle = `<style>:root{--paper:#f7f7ef;--panel:#fdfdf7;--ink:#232720;--muted:#697063;--line:#dde0d3;--accent:#c64f38;--action:#2352c9;--action-ink:#f4f3ef;--font-sans:"Avenir Next",Avenir,"Segoe UI",sans-serif}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font-family:var(--font-sans);font-size:16px;line-height:1.5;overflow-wrap:anywhere}main{max-width:40rem;margin:0 auto;padding:28px 18px 48px}h1{font-size:clamp(32px,4vw,48px);line-height:1.1;letter-spacing:-.02em;margin:0 0 12px}.demo-flag{display:inline-flex;align-items:center;margin:0 0 16px;padding:4px 10px;border:1px solid var(--accent);border-radius:999px;color:#a8402c;font-size:12px;font-weight:700;letter-spacing:.08em}.sub{color:var(--muted);margin:0 0 16px}.note{margin:0 0 16px}nav.demo-nav{display:flex;flex-wrap:wrap;align-items:center;gap:8px;width:auto;max-width:100%;margin:0 0 16px}nav.demo-nav a{flex:0 0 auto;width:auto;min-width:44px;max-width:none;min-height:44px;justify-content:center;overflow:visible;padding:8px 12px;text-align:center;white-space:nowrap;overflow-wrap:normal;word-break:normal}a{color:var(--ink);display:inline-flex;align-items:center;min-width:44px;min-height:44px;padding:8px 2px}a.cta{background:var(--action);color:var(--action-ink);text-decoration:none;border-radius:5px;padding:12px 18px;min-width:44px;font-weight:600}</style>`

const demoTagline = "Demo landing page builder: launch a landing page without code."

const demoDocsCopy = "Demo landing page builder. Templates, lead capture, and email signup are the sample features. A template is a starting layout for a page. Lead capture is a form on that page. Email signup is the list the form would join. This demo does not send mail, and it does not store an address. These pages stay public so a readiness check can open a landing page, a pricing page, and this documentation. Pricing says there is no charge. Privacy and terms say the demo stores no personal data. Get started opens a sign-in screen for the demo and does not create an account. Nothing here is a customer count, a rating, or a paid plan. The label DEMO means the page is a sample, not a product you can buy."

// DemoCheckoutURL is empty on the shipped server. A checkout link is rendered
// only when a test sets this to a placeholder URL. There is no secret env for it.
func (h *HostedHandler) mountDemo(public *gin.RouterGroup) {
	public.GET("/demo", func(c *gin.Context) { h.writeDemo(c, "") })
	public.HEAD("/demo", func(c *gin.Context) { h.writeDemo(c, "") })
	public.GET("/demo/:page", func(c *gin.Context) { h.writeDemo(c, c.Param("page")) })
	public.HEAD("/demo/:page", func(c *gin.Context) { h.writeDemo(c, c.Param("page")) })
	public.POST("/demo/operator", h.postDemoOperator)
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
	case "operator":
		h.writeDemoOperator(c, false)
		return
	case "digest":
		h.writeDemoDigest(c)
		return
	case "post":
		h.writeDemoPost(c)
		return
	default:
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument(title, description, body))
}

// postDemoOperator accepts the demo approve button and changes nothing.
// The placement id in the form is ignored. Real approval stays on the
// admin key and the signed-in owner route.
func (h *HostedHandler) postDemoOperator(c *gin.Context) {
	c.Header("X-Robots-Tag", "noindex")
	_ = c.Request.ParseForm()
	h.writeDemoOperator(c, true)
}

func (h *HostedHandler) demoRows(c *gin.Context) []database.DemoPlacementRow {
	if h == nil || h.DB == nil || h.DB.Pool == nil {
		return nil
	}
	rows, err := h.DB.ListDemoPlacements(c.Request.Context())
	if err != nil {
		return nil
	}
	return rows
}

func demoPendingRow(rows []database.DemoPlacementRow) database.DemoPlacementRow {
	for _, row := range rows {
		if row.Status == "pending" {
			return row
		}
	}
	return database.DemoPlacementRow{
		ID:     "demo-pending",
		Status: "pending",
		Copy:   demoPendingCopy,
		Domain: demoSampleDomain,
	}
}

func demoApprovedRow(rows []database.DemoPlacementRow) database.DemoPlacementRow {
	for _, row := range rows {
		if row.ApprovedBy == "operator" {
			return row
		}
	}
	for _, row := range rows {
		if row.Status == "approved" || row.Status == "committed" {
			return row
		}
	}
	return database.DemoPlacementRow{
		ID:         "demo-approved",
		Status:     "approved",
		Copy:       demoApprovedCopy,
		ApprovedBy: "operator",
		Domain:     demoSampleDomain,
		Clicks:     1,
		PaidClicks: 0,
	}
}

func (h *HostedHandler) writeDemoOperator(c *gin.Context, posted bool) {
	rows := h.demoRows(c)
	pending := demoPendingRow(rows)
	approved := demoApprovedRow(rows)
	note := ""
	if posted {
		note = `<p class="note">Nothing was approved.</p>`
	}
	body := demoFlag() + `<h1>Operator approval</h1><p class="sub">Read-only demo. No key and no sign-in. This screen cannot approve a placement.</p>` + note +
		`<section><h2>Pending placement</h2>` + demoPlacementFacts(pending) +
		`<form method="post" action="/demo/operator"><input type="hidden" name="placement_id" value="` + html.EscapeString(pending.ID) + `"><button type="submit">Approve</button></form>` +
		`<p class="note">The approve button is a demo. It does not change a placement.</p></section>` +
		`<section><h2>Approved placement</h2>` + demoPlacementFacts(approved) +
		`<p>approved_by ` + html.EscapeString(approved.ApprovedBy) + `</p></section>` +
		`<p><a href="` + demoHref("/demo", demoAttribution(c)) + `">Back to the demo</a></p>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument("Operator approval · Demo", "Read-only demo of operator approval.", body))
}

func demoPlacementFacts(row database.DemoPlacementRow) string {
	return `<p>Status: ` + html.EscapeString(row.Status) + `</p><p>Site: ` + html.EscapeString(row.Domain) + `</p><p>` + html.EscapeString(row.Copy) + `</p>`
}

func (h *HostedHandler) writeDemoDigest(c *gin.Context) {
	rows := h.demoRows(c)
	if len(rows) == 0 {
		rows = []database.DemoPlacementRow{demoApprovedRow(nil)}
	}
	var cards strings.Builder
	for _, row := range rows {
		cards.WriteString(`<article data-site="` + html.EscapeString(row.Domain) + `"><h2>` + html.EscapeString(row.Copy) + `</h2>`)
		cards.WriteString(`<p>Site: ` + html.EscapeString(row.Domain) + `</p>`)
		cards.WriteString(`<p>clicks <span data-clicks>` + strconv.Itoa(row.Clicks) + `</span></p>`)
		cards.WriteString(`<p>paid_clicks <span data-paid-clicks>` + strconv.Itoa(row.PaidClicks) + `</span></p>`)
		cards.WriteString(`<p>approved_by ` + html.EscapeString(row.ApprovedBy) + `</p></article>`)
	}
	body := demoFlag() + `<h1>Paid-click digest</h1><p class="sub">Demo site only. No key. This page does not read any other site.</p>` + cards.String() +
		`<p><a href="` + demoHref("/demo", demoAttribution(c)) + `">Back to the demo</a></p>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument("Digest · Demo", "Paid-click digest for the demo site.", body))
}

func (h *HostedHandler) demoSponsoredHref(c *gin.Context) string {
	if h == nil || h.DB == nil || h.DB.Pool == nil {
		return "/v1/go/" + demoGoCode
	}
	id, err := h.DB.DemoSponsoredPlacementID(c.Request.Context())
	if err != nil || !uuidPattern.MatchString(id) {
		return "/v1/go/" + demoGoCode
	}
	return "/v1/go/" + id
}

func (h *HostedHandler) writeDemoPost(c *gin.Context) {
	href := h.demoSponsoredHref(c)
	body := demoFlag() + `<h1>Demo post</h1><p class="sub">A sample post on the demo site.</p>` +
		`<p>Sponsored: <a href="` + html.EscapeString(href) + `">Read the offer</a></p>` +
		`<p>Disclosure: This post includes an affiliate link. We may earn a commission if you follow it.</p>` +
		`<p><a href="` + demoHref("/demo", demoAttribution(c)) + `">Back to the demo</a></p>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", demoDocument("Demo post", "Sample post with a sponsored link.", body))
}
