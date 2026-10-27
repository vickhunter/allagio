package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const ownerApprovalMessage = "An owner link is needed. Read waiting.url on the deal. Sign in as the owner and open that link."

var humanStepOrder = []string{"ask", "list", "negotiate", "accept", "execute", "amend", "settle", "share", "revoke"}

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
	ask := false
	named := make([]string, 0, len(steps))
	for _, step := range steps {
		if step == "ask" {
			ask = true
			continue
		}
		named = append(named, step)
	}
	switch {
	case !ask && len(named) == 0:
		return "An ask is approved by the agent. The host records approved_by agent. That is not a person's approval."
	case ask && len(named) == 0:
		return "An ask waits for the owner. The agent cannot approve it. Sign in and open the approval link on the deal. The host records approved_by owner."
	case ask:
		return "An ask waits for the owner. The agent cannot approve it. Sign in and open the approval link on the deal. The host records approved_by owner. These steps also wait for the owner: " + strings.Join(named, ", ") + "."
	default:
		return "These steps wait for the owner: " + strings.Join(named, ", ") + ". The agent cannot approve them. Sign in and open the approval link on the deal. The host records approved_by owner. Any other ask is approved by the agent. The host records approved_by agent. That is not a person's approval."
	}
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
		return "Ask: the owner approves an ask. Leave this off and the agent approves it."
	default:
		return name + ": the owner approves this step."
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
