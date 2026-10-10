package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

func (h *HostedHandler) SiteConnectionStatus(c *gin.Context) {
	if c.GetBool("agent_access") {
		c.Status(http.StatusForbidden)
		return
	}
	linked, err := h.DB.SiteHasOwner(c.Request.Context(), siteIDFrom(c))
	if err != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"connected": linked})
}

func (h *HostedHandler) StartSiteConnection(c *gin.Context) {
	if c.GetBool("agent_access") {
		c.Status(http.StatusForbidden)
		return
	}
	token, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	if err = h.DB.StartSiteOwnerConnection(c.Request.Context(), siteIDFrom(c), services.HashAPIKey(token)); err != nil {
		c.Status(500)
		return
	}
	c.JSON(201, gin.H{"connect_url": h.BaseURL + "/owner/connect-site?token=" + url.QueryEscape(token), "expires_in": 600})
}

func (h *HostedHandler) ReviewSiteConnection(c *gin.Context) {
	token := c.Query("token")
	_, domain, err := h.DB.SiteOwnerConnection(c.Request.Context(), services.HashAPIKey(token))
	if len(token) != 43 || err != nil {
		c.Status(410)
		h.render(c, "Connection link expired", "Return to WordPress and connect your publisher account again.", nil)
		return
	}
	sourceField := ""
	if raw := c.Query("source"); raw != "" {
		sourceField = fmt.Sprintf(`<input type="hidden" name="source" value="%s">`, template.HTMLEscapeString(raw))
	}
	body := template.HTML(fmt.Sprintf(`<section><h2>Connect %s</h2><p>This connection was requested from your WordPress installation. Linking it lets you review distribution placements for this site and connect your own agent.</p><form method="post" action="/owner/connect-site"><input type="hidden" name="csrf" value="%s"><input type="hidden" name="token" value="%s">%s<button>Connect this site to my account</button></form></section>`, template.HTMLEscapeString(domain), template.HTMLEscapeString(ownerFrom(c).CSRF), template.HTMLEscapeString(token), sourceField))
	h.render(c, "Connect your WordPress site", "Confirm the site before continuing.", body)
}

func (h *HostedHandler) CompleteSiteConnection(c *gin.Context) {
	token := c.PostForm("token")
	if len(token) != 43 {
		c.Status(400)
		return
	}
	source := ""
	if raw := c.PostForm("source"); raw != "" {
		source = database.NormalizeAttributionSource(raw)
	}
	if err := h.DB.CompleteSiteOwnerConnection(c.Request.Context(), ownerFrom(c).ID, services.HashAPIKey(token), source); err != nil {
		c.Status(409)
		h.render(c, "Site connection unavailable", "This link expired, was already used, or the site/account is already connected. Return to WordPress to retry.", nil)
		return
	}
	h.render(c, "Your site is connected", "Return to WordPress to find offers and publish approved recommendations.", template.HTML(`<p><a class="button" href="/owner/placements">Open placements</a></p>`))
}
