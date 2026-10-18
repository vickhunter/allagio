package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func deviceSignInReturn(u *url.URL) bool {
	if u == nil || u.Path != "/device" || u.RawPath != "" {
		return false
	}
	q := u.Query()
	if len(q) == 0 {
		return true
	}
	codes := q["user_code"]
	if len(q) != 1 || len(codes) != 1 {
		return false
	}
	if codes[0] == "" {
		return true
	}
	_, ok := canonicalUserCode(codes[0])
	return ok
}

func deviceReturnPath(userCode string) string {
	code, ok := canonicalUserCode(userCode)
	if !ok {
		return "/device"
	}
	return "/device?user_code=" + url.QueryEscape(code)
}

func splitDeviceProject(raw string) (scope, id string, ok bool) {
	scope, id, found := strings.Cut(raw, ":")
	if !found || (scope != "site" && scope != "offer") || !uuidPattern.MatchString(id) {
		return "", "", false
	}
	return scope, id, true
}

func (h *HostedHandler) DevicePage(c *gin.Context) {
	owner, err := h.currentOwner(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login?return_to="+url.QueryEscape(deviceReturnPath(c.Query("user_code"))))
		return
	}
	c.Set("owner", owner)
	code, recognized := canonicalUserCode(c.Query("user_code"))
	note := ""
	if c.Query("user_code") != "" && !recognized {
		note = "That code was not recognized. Enter the code your agent showed you."
	}
	h.writeDevicePage(c, owner, code, note, http.StatusOK, true)
}

func (h *HostedHandler) DeviceDecision(c *gin.Context) {
	owner := ownerFrom(c)
	code, good := canonicalUserCode(c.PostForm("user_code"))
	decision := c.PostForm("decision")
	if !good || (decision != "approve" && decision != "deny") {
		h.writeDevicePage(c, owner, "", "Enter the user code from your agent, then approve or deny it.", http.StatusBadRequest, true)
		return
	}
	scope, projectID, projectOK := splitDeviceProject(c.PostForm("project"))
	if decision == "approve" && !projectOK {
		h.writeDevicePage(c, owner, code, "Choose the site or offer this code may use.", http.StatusBadRequest, true)
		return
	}
	if decision == "deny" {
		scope, projectID = "", ""
	}
	_, err := h.DB.DecideDevice(c.Request.Context(), services.HashAPIKey(code), owner.ID, decision, scope, projectID, time.Now().UTC())
	if err == nil {
		if decision == "deny" {
			h.writeDevicePage(c, owner, "", "Denied. The agent will not get a key.", http.StatusOK, false)
			return
		}
		h.writeDevicePage(c, owner, "", "Approved. The agent can pick up its project key.", http.StatusOK, false)
		return
	}
	status, note := deviceDecisionError(err)
	h.writeDevicePage(c, owner, code, note, status, true)
}

func deviceDecisionError(err error) (int, string) {
	switch {
	case errors.Is(err, database.ErrDeviceNotFound):
		return http.StatusNotFound, "That user code was not found."
	case errors.Is(err, database.ErrDeviceExpired):
		return http.StatusBadRequest, "That code has expired."
	case errors.Is(err, database.ErrDeviceUsed):
		refusal, ok := entry.RestoreRefusalByReason("device_used")
		if !ok {
			return http.StatusConflict, "That device code was already used."
		}
		return http.StatusConflict, refusal.Text
	case errors.Is(err, database.ErrProjectScope):
		return http.StatusForbidden, "That project is not on this account."
	default:
		return http.StatusInternalServerError, "The code could not be saved."
	}
}

func (h *HostedHandler) writeDevicePage(c *gin.Context, owner *database.Owner, code, note string, status int, showForm bool) {
	projects, err := h.DB.AccountProjects(c.Request.Context(), owner.ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		h.render(c, "Approve this device", "Projects could not be loaded.", template.HTML(""))
		return
	}
	var body strings.Builder
	if note != "" {
		body.WriteString(`<p role="status">`)
		body.WriteString(template.HTMLEscapeString(note))
		body.WriteString(`</p>`)
	}
	if showForm {
		body.WriteString(`<p>An agent asked you to approve this code. Approving gives that agent a project key for the site or offer you choose. It does not approve a placement.</p>`)
		body.WriteString(`<form method="post" action="/device">`)
		body.WriteString(`<input type="hidden" name="csrf" value="`)
		body.WriteString(template.HTMLEscapeString(owner.CSRF))
		body.WriteString(`">`)
		body.WriteString(`<label>User code <input name="user_code" autocomplete="off" value="`)
		body.WriteString(template.HTMLEscapeString(code))
		body.WriteString(`"></label>`)
		if len(projects) == 0 {
			body.WriteString(`<p>This account has no site or offer yet.</p>`)
		} else {
			body.WriteString(`<label>Project <select name="project">`)
			body.WriteString(`<option value="">Choose a site or offer</option>`)
			for _, project := range projects {
				label := "Site"
				if project.Kind == "offer" {
					label = "Offer"
				}
				body.WriteString(`<option value="`)
				body.WriteString(template.HTMLEscapeString(project.Kind + ":" + project.ID))
				body.WriteString(`">`)
				body.WriteString(template.HTMLEscapeString(label + " · " + project.Name))
				body.WriteString(`</option>`)
			}
			body.WriteString(`</select></label>`)
		}
		body.WriteString(`<button name="decision" value="approve">Approve</button>`)
		body.WriteString(`<button name="decision" value="deny">Deny</button>`)
		body.WriteString(`</form>`)
	}
	c.Status(status)
	h.render(c, "Approve this device", "", template.HTML(body.String()))
}
