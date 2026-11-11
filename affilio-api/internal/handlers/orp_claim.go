package handlers

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

// PartyClaim is POST /v1/party-claims. The bearer's own party mints a
// 24-hour single-use link; the agent hands it to its human, who signs in
// and takes over the party's owner account (GAPS item 12).
func (h *ORPHost) PartyClaim(c *gin.Context) {
	origin, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok || h.Entry == nil || h.Entry.DB == nil {
		return
	}
	if grant.OwnerID == "" {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "This credential has no owner."}, false)
		return
	}
	token, err := randomToken()
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	expires := h.now().Add(24 * time.Hour)
	if err := h.Entry.DB.CreatePartyClaim(c.Request.Context(), grant.OwnerID, services.HashAPIKey(token), expires); err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"claim_url":  origin + "/claim/" + token,
		"expires_at": expires.UTC().Format(time.RFC3339),
	})
}

// ClaimPage is GET /claim/:code. It never mutates: a signed-out visitor
// gets a sign-in link, the party's own owner a short-circuit, and any
// other signed-in person a confirm form.
func (h *ORPHost) ClaimPage(c *gin.Context) {
	_, dns := h.origin(c)
	code := c.Param("code")
	if h.Entry == nil || h.Entry.DB == nil {
		writeORPBytes(c, http.StatusServiceUnavailable, []byte("claim unavailable"))
		return
	}
	state := h.Entry.DB.PartyClaimState(c.Request.Context(), services.HashAPIKey(code), h.now())
	var b strings.Builder
	b.WriteString("<!doctype html><title>Claim this party</title>")
	switch state.State {
	case "missing":
		b.WriteString(`<p>This claim link is not valid.</p>`)
		writeORPBytes(c, http.StatusNotFound, []byte(b.String()))
		return
	case "claimed":
		b.WriteString(`<p>This claim link was already used.</p>`)
		writeORPBytes(c, http.StatusGone, []byte(b.String()))
		return
	case "expired":
		b.WriteString(`<p>This claim link expired. Ask the agent for a fresh one.</p>`)
		writeORPBytes(c, http.StatusGone, []byte(b.String()))
		return
	}
	party := orp.MemberParty(dns, state.OwnerID)
	owner, signed := h.signedInOwner(c)
	b.WriteString(`<h1>Claim `)
	b.WriteString(template.HTMLEscapeString(party))
	b.WriteString(`</h1>`)
	if !signed {
		b.WriteString(`<p>Sign in to claim this party. Your sign-in becomes the party's owner.</p>`)
		b.WriteString(`<p><a href="/auth/login?return_to=/claim/`)
		b.WriteString(template.HTMLEscapeString(code))
		b.WriteString(`">Sign in</a></p>`)
		writeORPBytes(c, http.StatusOK, []byte(b.String()))
		return
	}
	if owner.ID == state.OwnerID {
		b.WriteString(`<p>This party is already yours.</p><p><a href="/owner">Open your workspace</a></p>`)
		writeORPBytes(c, http.StatusOK, []byte(b.String()))
		return
	}
	b.WriteString(`<p>Signed in as `)
	b.WriteString(template.HTMLEscapeString(owner.Email))
	b.WriteString(`. Claiming moves your sign-in onto this party's account; its deals, spaces, and keys become yours.</p>`)
	b.WriteString(`<form method="post" action="/claim/`)
	b.WriteString(template.HTMLEscapeString(code))
	b.WriteString(`"><input type="hidden" name="csrf" value="`)
	b.WriteString(template.HTMLEscapeString(owner.CSRF))
	b.WriteString(`"><button type="submit">Claim this party</button></form>`)
	writeORPBytes(c, http.StatusOK, []byte(b.String()))
}

// ClaimPost is POST /claim/:code. Origin + session CSRF required.
func (h *ORPHost) ClaimPost(c *gin.Context) {
	origin, _ := h.origin(c)
	code := c.Param("code")
	if h.Entry == nil || h.Entry.DB == nil {
		writeORPBytes(c, http.StatusServiceUnavailable, []byte("claim unavailable"))
		return
	}
	owner, signed := h.signedInOwner(c)
	if !signed || !ownerFormCSRF(c, origin, owner.CSRF) {
		writeORPBytes(c, http.StatusForbidden, []byte("forbidden"))
		return
	}
	token, err := randomToken()
	if err != nil {
		writeORPBytes(c, http.StatusInternalServerError, []byte("internal error"))
		return
	}
	csrf, err := randomToken()
	if err != nil {
		writeORPBytes(c, http.StatusInternalServerError, []byte("internal error"))
		return
	}
	_, problem, err := h.Entry.DB.ClaimParty(c.Request.Context(), services.HashAPIKey(code), owner.ID, services.HashAPIKey(token), csrf, h.now())
	if err != nil {
		writeORPBytes(c, http.StatusInternalServerError, []byte("internal error"))
		return
	}
	if problem != "" {
		status := http.StatusGone
		if problem == "taken" {
			status = http.StatusConflict
		}
		writeORPBytes(c, status, []byte("claim "+problem))
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     ownerCookieName(h.Entry.BaseURL),
		Value:    token,
		Path:     "/",
		MaxAge:   8 * 3600,
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.Entry.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusSeeOther, "/owner")
}
