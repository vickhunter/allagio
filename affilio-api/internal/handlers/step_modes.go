package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func stepOffMessage(step string) string {
	switch step {
	case database.StepSiteSignup:
		return "Site signup is off for this owner, so this step did not run."
	case database.StepOfferMatch:
		return "Offer match is off for this owner, so this step did not run."
	case database.StepPlacementPublish:
		return "Placement publish is off for this owner, so this step did not run."
	case database.StepLinkChange:
		return "Link change is off for this owner, so this step did not run."
	case database.StepPayout:
		return "Payout actions are off for this owner, so this step did not run."
	default:
		return "This step is off, so it did not run."
	}
}

func stepAskMessage(step string) string {
	switch step {
	case database.StepSiteSignup:
		return "Site signup is waiting for approval."
	case database.StepOfferMatch:
		return "Offer match is waiting for approval."
	case database.StepPlacementPublish:
		return "Placement publish is waiting for approval."
	case database.StepLinkChange:
		return "Link change is waiting for approval."
	case database.StepPayout:
		return "This payout action is waiting for approval."
	default:
		return "This step is waiting for approval."
	}
}

const domainUnverifiedMessage = "This domain is not verified, so placement publish waits for approval."

const placementWaitingAction = "Owner reviews exact copy, disclosure, surface and terms; an operator records their approval."

type publishChoice struct {
	Effective string
	Reason    string
	Message   string
}

// publishChoice picks the placement publish mode. Off never runs.
// An unverified domain forces ask over auto. A stored ask waits for a person.
// A claims checker that is not live also holds an auto placement.
// An empty stored value is the pre-D-088 read (auto). New sites store ask,
// and that stored ask does not flip when claim checks are live.
func choosePublish(stored string, verified, claimsReal bool) publishChoice {
	if stored == "" {
		stored = database.ModeAuto
	}
	if stored == database.ModeOff {
		return publishChoice{Effective: database.ModeOff, Reason: "step_off", Message: stepOffMessage(database.StepPlacementPublish)}
	}
	if !verified {
		return publishChoice{Effective: database.ModeAsk, Reason: "domain_unverified", Message: domainUnverifiedMessage}
	}
	if stored == database.ModeAsk {
		return publishChoice{Effective: database.ModeAsk, Reason: "step_ask", Message: stepAskMessage(database.StepPlacementPublish)}
	}
	if !claimsReal {
		return publishChoice{Effective: database.ModeAsk, Reason: entry.ClaimCheckReason, Message: entry.ClaimCheckMessage}
	}
	return publishChoice{Effective: database.ModeAuto}
}

func stepFields(step, reason, message string) gin.H {
	return gin.H{"reason": reason, "step": step, "message": message}
}

// stepBody is a refusal. error repeats message for clients that only read error.
func stepBody(step, reason, message string) gin.H {
	body := stepFields(step, reason, message)
	body["error"] = message
	return body
}

// heldResponse is a created or queued hold. The request succeeded, so the
// body has status and hold_reason and no error key.
func heldResponse(step, reason, message string) gin.H {
	body := stepFields(step, reason, message)
	body["status"] = "awaiting_approval"
	body["hold_reason"] = reason
	return body
}

// holdStep stops the handler when the step is ask or off.
// Auto returns false and the caller continues. Ask is stored on the operator queue.
func holdStep(c *gin.Context, db *database.DB, step, ownerID, siteID string, payload map[string]any) bool {
	modes, err := db.LoadStepModes(c.Request.Context(), ownerID, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
		return true
	}
	switch modes.Get(step) {
	case database.ModeOff:
		c.JSON(http.StatusForbidden, stepBody(step, "step_off", stepOffMessage(step)))
		return true
	case database.ModeAsk:
		raw, err := json.Marshal(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
			return true
		}
		id, err := db.InsertStepApproval(c.Request.Context(), step, ownerID, siteID, raw)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
			return true
		}
		body := heldResponse(step, "step_ask", stepAskMessage(step))
		body["approval_id"] = id
		c.JSON(http.StatusAccepted, body)
		return true
	default:
		return false
	}
}

// signupGate picks the strictest site_signup mode among owners of this email.
// Off wins over ask. No saved row stays auto, which returns an empty mode.
func signupGate(c *gin.Context, db *database.DB, owners []database.OwnerRef) (ownerID, siteID, mode string, err error) {
	for _, owner := range owners {
		var modes database.StepModes
		modes, err = db.LoadStepModes(c.Request.Context(), owner.ID, owner.SiteID)
		if err != nil {
			return "", "", "", err
		}
		current := modes.Get(database.StepSiteSignup)
		if current == database.ModeOff {
			return owner.ID, owner.SiteID, database.ModeOff, nil
		}
		if current == database.ModeAsk && mode == "" {
			ownerID, siteID, mode = owner.ID, owner.SiteID, database.ModeAsk
		}
	}
	return ownerID, siteID, mode, nil
}

// holdDashboardStep applies the same ask and off decision as holdStep.
// A JSON client gets that JSON body. A dashboard form gets the owner page,
// with the reason visible, and ask is still stored on the approval queue.
func (h *HostedHandler) holdDashboardStep(c *gin.Context, step, ownerID, siteID string, payload map[string]any) bool {
	if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
		return holdStep(c, h.DB, step, ownerID, siteID, payload)
	}
	modes, err := h.DB.LoadStepModes(c.Request.Context(), ownerID, siteID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return true
	}
	switch modes.Get(step) {
	case database.ModeOff:
		c.Status(http.StatusForbidden)
		h.render(c, "This step is off", stepOffMessage(step), template.HTML(`<p data-reason="step_off">step_off</p>`))
		return true
	case database.ModeAsk:
		raw, err := json.Marshal(payload)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return true
		}
		id, err := h.DB.InsertStepApproval(c.Request.Context(), step, ownerID, siteID, raw)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return true
		}
		c.Status(http.StatusAccepted)
		h.render(c, "Waiting for approval", stepAskMessage(step), template.HTML(`<p data-reason="step_ask">step_ask</p><p data-approval-id="`+template.HTMLEscapeString(id)+`"></p>`))
		return true
	default:
		return false
	}
}

func (h *SitesHandler) holdRegister(c *gin.Context, req siteRegisterRequest) bool {
	key := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if key == "" {
		return false
	}
	site, err := h.DB.GetSiteByAnyKeyHash(c.Request.Context(), services.HashAPIKey(key), services.KeyPrefix(key))
	if err != nil {
		return false
	}
	ownerID := ownerForSite(c, h.DB, site.ID)
	return holdStep(c, h.DB, database.StepSiteSignup, ownerID, site.ID, map[string]any{
		"kind": "register", "domain": req.Domain, "name": req.Name,
	})
}

func ownerForSite(c *gin.Context, db *database.DB, siteID string) string {
	if siteID == "" {
		return ""
	}
	id, err := db.OwnerIDBySite(c.Request.Context(), siteID)
	if err != nil {
		return ""
	}
	return id
}

func (h *AgentEntry) stepActor(c *gin.Context) (ownerID, siteID string, ok bool) {
	if c.GetHeader("X-API-Key") == "" && c.GetHeader("X-Maker-Key") == "" {
		if fields := strings.Fields(c.GetHeader("Authorization")); len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
			token := fields[1]
			grant, err := h.DB.AgentAccountBearer(c.Request.Context(), services.HashAPIKey(token), token, "")
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
				return "", "", false
			}
			if grant.SiteID != "" {
				return ownerForSite(c, h.DB, grant.SiteID), grant.SiteID, true
			}
			ownerID, _ = h.DB.OwnerIDByMaker(c.Request.Context(), grant.MakerID)
			return ownerID, "", true
		}
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return "", "", false
	}
	return actor.UserID, actor.SiteID, true
}

func (h *AgentEntry) GetStepModes(c *gin.Context) {
	ownerID, siteID, ok := h.stepActor(c)
	if !ok {
		return
	}
	modes, err := h.DB.LoadStepModes(c.Request.Context(), ownerID, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
		return
	}
	body := gin.H{
		"owner_id": ownerID,
		"site_id":  siteID,
		"modes": gin.H{
			database.StepSiteSignup:       modes.SiteSignup,
			database.StepOfferMatch:       modes.OfferMatch,
			database.StepPlacementPublish: modes.PlacementPublish,
			database.StepLinkChange:       modes.LinkChange,
			database.StepPayout:           modes.Payout,
		},
	}
	if siteID != "" {
		verified, err := h.DB.SiteVerified(c.Request.Context(), siteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
			return
		}
		choice := choosePublish(modes.PlacementPublish, verified, h.claims().Real())
		body["placement_publish_effective"] = choice.Effective
		if choice.Reason != "" {
			body["placement_publish_reason"] = choice.Reason
			body["placement_publish_message"] = choice.Message
		}
	}
	c.JSON(http.StatusOK, body)
}

func (h *AgentEntry) PutStepModes(c *gin.Context) {
	ownerID, siteID, ok := h.stepActor(c)
	if !ok {
		return
	}
	if ownerID == "" && siteID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner or site required"})
		return
	}
	raw, err := readJSONBody(c, 4096)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var patch map[string]string
	if json.Unmarshal(raw, &patch) != nil || len(patch) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be auto, ask, or off"})
		return
	}
	modes, err := h.DB.LoadStepModes(c.Request.Context(), ownerID, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
		return
	}
	for step, mode := range patch {
		if !modes.Set(step, mode) || !database.ValidStepMode(mode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be auto, ask, or off"})
			return
		}
	}
	if err = h.DB.SaveStepModes(c.Request.Context(), ownerID, siteID, modes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "step mode unavailable"})
		return
	}
	h.GetStepModes(c)
}

func (h *AgentEntry) GetStepApproval(c *gin.Context) {
	ownerID, siteID, ok := h.stepActor(c)
	if !ok {
		return
	}
	row, err := h.DB.GetStepApproval(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval unavailable"})
		return
	}
	if (row.OwnerID != "" && row.OwnerID != ownerID) || (row.OwnerID == "" && row.SiteID != "" && row.SiteID != siteID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
		return
	}
	c.JSON(http.StatusOK, approvalView(row))
}

func approvalView(row database.StepApproval) gin.H {
	body := gin.H{"id": row.ID, "step": row.Step, "status": row.Status, "owner_id": row.OwnerID, "site_id": row.SiteID}
	var payload any
	if json.Unmarshal(row.Payload, &payload) == nil {
		body["payload"] = payload
	}
	if len(row.Result) > 0 && string(row.Result) != "null" {
		var result any
		if json.Unmarshal(row.Result, &result) == nil {
			body["result"] = result
		}
	}
	return body
}
