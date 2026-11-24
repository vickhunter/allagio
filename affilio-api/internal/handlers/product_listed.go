package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

// productsListedSection is the owner-page control for find_partners. Listing
// stays on until the owner turns it off. The form matches the other owner
// posts: method post, hidden csrf, and the owner session checks both.
func (h *HostedHandler) productsListedSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	if o == nil || o.MakerID == "" {
		return "", nil
	}
	products, err := h.DB.ListProducts(c.Request.Context(), o.MakerID)
	if err != nil {
		return "", err
	}
	flags, err := h.DB.ProductListedFlags(c.Request.Context(), o.MakerID)
	if err != nil {
		return "", err
	}
	if len(products) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section id="product-listing"><h2>Products</h2>`)
	csrf := template.HTMLEscapeString(o.CSRF)
	for _, p := range products {
		id := template.HTMLEscapeString(p.ID)
		b.WriteString(`<div class="deal"><h3>`)
		b.WriteString(template.HTMLEscapeString(p.Name))
		b.WriteString(`</h3><p><a href="/owner/products/`)
		b.WriteString(id)
		b.WriteString(`/connection">Connect Stripe</a></p>`)
		b.WriteString(`<form method="post" action="/owner/products/`)
		b.WriteString(id)
		b.WriteString(`/listed"><input type="hidden" name="csrf" value="`)
		b.WriteString(csrf)
		b.WriteString(`"><label><input type="checkbox" name="listed" value="on"`)
		if flags[p.ID] {
			b.WriteString(` checked`)
		}
		b.WriteString(`> Listed in find_partners</label> <button>Save listing</button></form></div>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String()), nil
}

// SetProductListed is the owner toggle. A product that is not this owner's
// is a 404. An unchecked box posts no listed field and turns listing off.
func (h *HostedHandler) SetProductListed(c *gin.Context) {
	o := ownerFrom(c)
	id := strings.ToLower(strings.TrimSpace(c.Param("id")))
	if o.MakerID == "" || !uuidPattern.MatchString(id) {
		c.Status(http.StatusNotFound)
		return
	}
	ok, err := h.DB.SetProductListed(c.Request.Context(), o.ID, id, c.PostForm("listed") == "on")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner")
}
