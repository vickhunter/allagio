package handlers

import (
	"crypto/subtle"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const ownerApprovalMessage = "An owner link is needed. Read waiting.url on the deal. Sign in as the owner, open that link, and submit the form."

// humanStepOrder is the only step this host gates. Approve and ApprovePost
// call humanStepOn. An ask matches a deal waiting with step_ask, which is the
// list step on an offer and the accept step.
var humanStepOrder = []string{"ask"}

func writeOwnerApproval(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"reason":  "owner_approval",
		"message": ownerApprovalMessage,
	})
}

func humanStepOn(steps []string, waiting map[string]any) bool {
	step, _ := waiting["step"].(string)
	for _, item := range steps {
		if item == step {
			return true
		}
		if item == "ask" && (step == "ask" || reasonHas(waiting, "step_ask")) {
			return true
		}
	}
	return false
}

func reasonHas(waiting map[string]any, reason string) bool {
	reasons, _ := waiting["reasons"].([]any)
	for _, item := range reasons {
		if item == reason {
			return true
		}
	}
	return false
}

func (h *ORPHost) noteApproval(deal *database.ORPDeal, party, step, by string) {
	if by != "agent" && by != "owner" {
		return
	}
	doc := mustMany(*deal)
	for _, row := range doc.Approvals {
		if row.Party == party && row.Step == step && row.By == by {
			return
		}
	}
	doc.Approvals = append(doc.Approvals, orp.Approval{Party: party, Step: step, By: by})
	h.putMany(deal, doc)
}

func (h *ORPHost) signedInOwner(c *gin.Context) (*database.Owner, bool) {
	if h.Entry == nil || h.Entry.DB == nil {
		return nil, false
	}
	cookie, err := c.Cookie(ownerCookieName(h.Entry.BaseURL))
	if err != nil || cookie == "" {
		return nil, false
	}
	owner, err := h.Entry.DB.OwnerSession(c.Request.Context(), services.HashAPIKey(cookie))
	if err != nil || owner == nil || owner.ID == "" {
		return nil, false
	}
	return owner, true
}

func humanApprovalWords(steps []string) string {
	for _, step := range steps {
		if step == "ask" {
			return "An ask waits for the owner. On this host that is a deal waiting with step_ask: the list step on an offer, and the accept step. The agent cannot approve it. Sign in, open the approval link, and submit the form. Opening the link does not approve it. The host records approved_by owner."
		}
	}
	return "An ask is approved by the agent. On this host that is a deal waiting with step_ask: the list step on an offer, and the accept step. The host records approved_by agent. That is not a person's approval."
}

// purposePreview is the prefetch class used for /go. A prefetch does not approve.
func purposePreview(c *gin.Context) bool {
	purpose := strings.Join([]string{
		c.GetHeader("Sec-Fetch-Purpose"),
		c.GetHeader("Sec-Purpose"),
		c.GetHeader("Purpose"),
		c.GetHeader("X-Moz"),
		c.GetHeader("X-Purpose"),
	}, " ")
	return orp.ClickClass(c.Request.Method, "Mozilla", purpose, "") == "preview"
}

func writePreview(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"reason":  "preview",
		"message": "A prefetch does not approve.",
	})
}

// ownerFormCSRF matches the signed-in owner form rule: Origin is this host,
// and the form field or X-CSRF-Token matches the session token.
func ownerFormCSRF(c *gin.Context, base, token string) bool {
	if base == "" || c.GetHeader("Origin") != base {
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	csrf := c.GetHeader("X-CSRF-Token")
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if c.Request.ParseForm() != nil {
			return false
		}
		csrf = c.PostForm("csrf")
	}
	if csrf == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(csrf), []byte(token)) == 1
}

func (h *HostedHandler) humanApprovalSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	steps, err := h.DB.HumanApprovalSteps(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	on := map[string]bool{}
	for _, step := range steps {
		on[step] = true
	}
	var b strings.Builder
	b.WriteString(`<section id="human-approval"><h2>Who approves an ask</h2>`)
	b.WriteString(`<p id="human-approval-words">`)
	b.WriteString(template.HTMLEscapeString(humanApprovalWords(steps)))
	b.WriteString(`</p><form method="post" action="/owner/human-approval">`)
	b.WriteString(`<input type="hidden" name="csrf" value="`)
	b.WriteString(template.HTMLEscapeString(o.CSRF))
	b.WriteString(`">`)
	for _, name := range humanStepOrder {
		b.WriteString(`<label><input type="checkbox" name="`)
		b.WriteString(template.HTMLEscapeString(name))
		b.WriteString(`" value="on"`)
		if on[name] {
			b.WriteString(` checked`)
		}
		b.WriteString(`> `)
		b.WriteString(template.HTMLEscapeString(humanStepLabel(name)))
		b.WriteString(`</label>`)
	}
	b.WriteString(`<button>Save</button></form></section>`)
	return template.HTML(b.String()), nil
}

func humanStepLabel(name string) string {
	switch name {
	case "ask":
		return "Ask: the owner approves a deal waiting with step_ask. On this host that is the list step on an offer, and the accept step. Leave this off and the agent approves it."
	default:
		return ""
	}
}

func (h *HostedHandler) SaveHumanApprovalForm(c *gin.Context) {
	o := ownerFrom(c)
	var steps []string
	for _, name := range humanStepOrder {
		if c.PostForm(name) == "on" {
			steps = append(steps, name)
		}
	}
	if err := h.DB.SaveHumanApprovalSteps(c.Request.Context(), o.ID, steps); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}

func (h *HostedHandler) GetHumanApproval(c *gin.Context) {
	steps, err := h.DB.HumanApprovalSteps(c.Request.Context(), ownerFrom(c).ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

func (h *HostedHandler) PutHumanApproval(c *gin.Context) {
	var body struct {
		Steps []string `json:"steps"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid human approval"})
		return
	}
	if body.Steps == nil {
		body.Steps = []string{}
	}
	if err := h.DB.SaveHumanApprovalSteps(c.Request.Context(), ownerFrom(c).ID, body.Steps); err != nil {
		if errors.Is(err, database.ErrUnknownHumanStep) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown step"})
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	steps, err := h.DB.HumanApprovalSteps(c.Request.Context(), ownerFrom(c).ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}
