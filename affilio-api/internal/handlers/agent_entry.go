package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const userCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// AgentEntry is the D-032 HTTP API: device login, setup, policy, listing cards, and digest.
type AgentEntry struct {
	DB             *database.DB
	BaseURL        string
	Claims         entry.ClaimChecker
	Owner          func(*gin.Context)
	AccountLimiter *IPRateLimiter
	DeviceLimiter  *IPRateLimiter
	Now            func() time.Time
}

type entryActor struct {
	Kind    string
	UserID  string
	SiteID  string
	MakerID string
	OfferID string
	Owner   *database.Owner
}

func (h *AgentEntry) Mount(r gin.IRouter) {
	r.POST("/v1/device/code", h.DeviceCode)
	r.POST("/v1/device/token", h.DeviceToken)
	r.POST("/v1/device/approve", h.ApproveDevice)
	r.POST("/v1/agent-accounts", h.CreateAgentAccount)
	r.GET("/v1/setup/settings", h.GetSetup)
	r.PUT("/v1/setup/settings", h.PutSetup)
	r.GET("/v1/sites/:id/mode", h.GetSiteMode)
	r.PUT("/v1/sites/:id/mode", h.PutSiteMode)
	r.POST("/v1/sites/:id/kill-switch", h.KillSwitch)
	r.POST("/v1/account/pause", h.AccountPause)
	r.POST("/v1/account/resume", h.AccountResume)
	r.GET("/v1/offers/:id/mode", h.GetOfferMode)
	r.PUT("/v1/offers/:id/mode", h.PutOfferMode)
	r.POST("/v1/placements/:id/decide", h.DecidePlacement)
	r.POST("/v1/placements/:id/revoke", h.RevokePlacement)
	r.POST("/v1/placements/:id/restore", h.RestorePlacement)
	r.POST("/v1/offers/:id/accept", h.AcceptPublisher)
	r.POST("/v1/listing-cards", h.CreateListing)
	r.GET("/v1/listing-cards/:id", h.GetListing)
	r.POST("/v1/events", h.RecordEvent)
	r.GET("/v1/digest", h.Digest)
	r.GET("/v1/account/digest", h.AccountDigest)
	r.GET("/v1/account/projects", h.AccountProjects)
	r.POST("/v1/account-keys", h.CreateAccountKey)
	r.GET("/v1/account-keys", h.ListAccountKeys)
	r.POST("/v1/account-keys/:id/revoke", h.RevokeAccountKey)
	r.GET("/v1/policy/hard-checks", h.HardChecks)
	r.POST("/v1/digest/undo", h.Undo)
	r.GET("/from/:source", h.FromSource)
	r.GET("/llms.txt", h.LLMs)
	r.GET("/docs/:name", h.Doc)
}

func (h *AgentEntry) claims() entry.ClaimChecker {
	if h.Claims == nil {
		return entry.StubClaimChecker{}
	}
	return h.Claims
}

func (h *AgentEntry) verificationURI() string {
	if h.BaseURL != "" {
		return strings.TrimRight(h.BaseURL, "/") + "/device"
	}
	return "/device"
}

func (h *AgentEntry) landingLocation() string {
	if h.BaseURL != "" {
		return strings.TrimRight(h.BaseURL, "/") + "/"
	}
	return "/"
}

func (h *AgentEntry) DeviceCode(c *gin.Context) {
	if h.DeviceLimiter != nil && !h.DeviceLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	device, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "code generation failed"})
		return
	}
	user, err := newUserCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "code generation failed"})
		return
	}
	expires := time.Now().Add(database.DeviceTTL)
	if err = h.DB.CreateDeviceAuthorization(c.Request.Context(), services.HashAPIKey(device), services.HashAPIKey(user), expires); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "code generation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"device_code":      device,
		"user_code":        user,
		"verification_uri": h.verificationURI(),
		"interval":         database.DeviceInterval,
		"expires_in":       int(database.DeviceTTL.Seconds()),
	})
}

func (h *AgentEntry) DeviceToken(c *gin.Context) {
	body, err := readJSONBody(c, 4096)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		DeviceCode string `json:"device_code"`
		GrantType  string `json:"grant_type"`
	}
	if decodeStrict(body, &req) != nil || req.DeviceCode == "" || (req.GrantType != "" && req.GrantType != "urn:ietf:params:oauth:grant-type:device_code") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_code required"})
		return
	}
	key, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	key = "or_" + key
	poll, err := h.DB.PollDevice(c.Request.Context(), services.HashAPIKey(req.DeviceCode), key, services.HashAPIKey(key), services.KeyPrefix(key), time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	if poll.Error != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": poll.Error, "interval": poll.Interval, "expires_in": poll.ExpiresIn})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": poll.AccessToken,
		"token_type":   "Bearer",
		"key_header":   poll.KeyHeader,
		"scope":        poll.Scope,
		"expires_in":   poll.TokenExpiresIn,
	})
}

func (h *AgentEntry) ApproveDevice(c *gin.Context) {
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	body, err := readJSONBody(c, 4096)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		UserCode  string `json:"user_code"`
		Decision  string `json:"decision"`
		Scope     string `json:"scope"`
		ProjectID string `json:"project_id"`
	}
	if decodeStrict(body, &req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	code, good := canonicalUserCode(req.UserCode)
	if !good || (req.Decision != "approve" && req.Decision != "deny") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_code and decision required"})
		return
	}
	if req.Decision == "approve" && (req.Scope != "site" && req.Scope != "offer" || !uuidPattern.MatchString(req.ProjectID)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope and project_id required"})
		return
	}
	at, err := h.DB.DecideDevice(c.Request.Context(), services.HashAPIKey(code), owner.ID, req.Decision, req.Scope, req.ProjectID, time.Now())
	if errors.Is(err, database.ErrDeviceNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user code not found"})
		return
	}
	if errors.Is(err, database.ErrDeviceExpired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expired"})
		return
	}
	if errors.Is(err, database.ErrDeviceUsed) {
		c.JSON(http.StatusConflict, gin.H{"error": "user code already used"})
		return
	}
	if errors.Is(err, database.ErrProjectScope) {
		c.JSON(http.StatusForbidden, gin.H{"error": "project is not on this account"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	if req.Decision == "deny" {
		c.JSON(http.StatusOK, gin.H{"status": "denied"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "approved", "human_login": true, "owner_id": owner.ID, "human_login_at": at.UTC().Format(time.RFC3339)})
}

func (h *AgentEntry) CreateAgentAccount(c *gin.Context) {
	if h.AccountLimiter != nil && !h.AccountLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	body, err := readJSONBody(c, 4096)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		Role   string `json:"role"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Domain string `json:"domain"`
		Source string `json:"source"`
	}
	if decodeStrict(body, &req) != nil || (req.Role != "publisher" && req.Role != "maker") || !validEmail(req.Email) || utf8.RuneCountInString(req.Name) < 1 || utf8.RuneCountInString(req.Name) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role, name, and email required"})
		return
	}
	if req.Role == "publisher" && (req.Domain == "" || strings.Contains(req.Domain, " ") || len(req.Domain) > 255) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain required"})
		return
	}
	subject, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	siteKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	makerKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	account, err := h.DB.CreateAgentOnlyAccount(c.Request.Context(), "agent:"+subject, strings.ToLower(req.Email), req.Name, database.NormalizeAttributionSource(req.Source), req.Role, strings.ToLower(req.Domain), services.HashAPIKey(siteKey), services.KeyPrefix(siteKey), makerKey)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "account already exists"})
		return
	}
	resp := gin.H{"owner_id": account.OwnerID, "agent_only": true, "human_login": false, "role": req.Role}
	if req.Role == "publisher" {
		resp["site_id"] = account.SiteID
		resp["api_key"] = siteKey
		resp["key_header"] = "X-API-Key"
	} else {
		resp["maker_id"] = account.MakerID
		resp["api_key"] = makerKey
		resp["key_header"] = "X-Maker-Key"
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *AgentEntry) AgentSignals(c *gin.Context) {
	signals, err := h.DB.AccountSignals(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "signals unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"human_login": gin.H{"count": len(signals.HumanIDs), "owner_ids": signals.HumanIDs},
		"agent_only":  gin.H{"count": len(signals.AgentIDs), "owner_ids": signals.AgentIDs},
	})
}

func (h *AgentEntry) GetSetup(c *gin.Context) {
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	ownerID := actor.UserID
	if actor.Owner != nil {
		ownerID = actor.Owner.ID
	}
	if ownerID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "owner not found"})
		return
	}
	role := c.Query("role")
	if role == "" {
		if actor.MakerID != "" && actor.SiteID == "" {
			role = "maker"
		} else {
			role = "publisher"
		}
	}
	if role != "maker" && role != "publisher" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be maker or publisher"})
		return
	}
	human, err := h.DB.OwnerHumanLogin(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	signedIn := actor.Kind == "owner_ui"
	saved, err := h.DB.GetSetup(c.Request.Context(), ownerID, role)
	if errors.Is(err, pgx.ErrNoRows) {
		path := c.Query("install_path")
		if path == "" {
			path = "cli"
		}
		mode := entry.DefaultMode(path)
		c.JSON(http.StatusOK, setupJSON(database.SetupSettings{Role: role, InstallPath: path, Mode: mode, ModeShown: mode}, false, human, signedIn))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	c.JSON(http.StatusOK, setupJSON(saved, true, human, signedIn))
}

func (h *AgentEntry) PutSetup(c *gin.Context) {
	if credentialPresented(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "setup is saved from a signed-in dashboard session"})
		return
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	body, err := readJSONBody(c, 8192)
	if err != nil || rejectUntrusted(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		Role          string   `json:"role"`
		InstallPath   string   `json:"install_path"`
		Mode          string   `json:"mode"`
		ModeShown     string   `json:"mode_shown"`
		Promote       string   `json:"promote"`
		AvoidTopics   []string `json:"avoid_topics"`
		Commission    string   `json:"commission"`
		SitesChannels []string `json:"sites_channels"`
		NeverTopics   []string `json:"never_topics"`
		BoxPosition   string   `json:"box_position"`
	}
	if decodeStrict(body, &req) != nil || (req.Role != "maker" && req.Role != "publisher") || !entry.ValidInstallPath(req.InstallPath) || !entry.ValidMode(req.Mode) || !entry.ValidMode(req.ModeShown) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role, install_path, mode, and mode_shown required"})
		return
	}
	if req.Role == "publisher" && req.BoxPosition != "" && !entry.ValidPosition(req.BoxPosition) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown box position"})
		return
	}
	if utf8.RuneCountInString(req.Promote) > 280 || utf8.RuneCountInString(req.Commission) > 32 || !shortList(req.AvoidTopics) || !shortList(req.NeverTopics) || !shortList(req.SitesChannels) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings are too long"})
		return
	}
	now := time.Now().UTC()
	saved := database.SetupSettings{
		OwnerID: owner.ID, Role: req.Role, InstallPath: req.InstallPath, Mode: req.Mode, ModeShown: req.ModeShown,
		Promote: req.Promote, AvoidTopics: req.AvoidTopics, Commission: req.Commission, SitesChannels: req.SitesChannels,
		NeverTopics: req.NeverTopics, BoxPosition: req.BoxPosition, SavedBy: owner.ID, SavedAt: now,
	}
	if err = h.DB.SaveSetup(c.Request.Context(), saved); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
		return
	}
	if err = h.applySetupMode(c, owner, saved); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode was not applied"})
		return
	}
	human, err := h.DB.OwnerHumanLogin(c.Request.Context(), owner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
		return
	}
	c.JSON(http.StatusOK, setupJSON(saved, true, human, true))
}

func (h *AgentEntry) applySetupMode(c *gin.Context, owner *database.Owner, saved database.SetupSettings) error {
	if saved.Role == "publisher" && owner.SiteID != "" {
		prev, err := h.DB.GetSitePolicy(c.Request.Context(), owner.SiteID)
		from := ""
		if errors.Is(err, pgx.ErrNoRows) {
			prev = defaultPolicy(saved.Mode)
			err = nil
		} else if err == nil {
			from = prev.Mode
		}
		if err != nil {
			return err
		}
		next := prev
		next.Mode = saved.Mode
		if next.MinRelevance == 0 {
			next.MinRelevance = 40
		}
		if next.MaxPlacementsPer30Days == 0 {
			next.MaxPlacementsPer30Days = 3
		}
		if saved.BoxPosition != "" {
			next.AllowedPositions = []string{saved.BoxPosition}
		}
		_, err = h.DB.UpsertSitePolicy(c.Request.Context(), owner.SiteID, next, "owner_ui", owner.ID, from)
		return err
	}
	if saved.Role == "maker" && owner.MakerID != "" {
		ids, err := h.DB.MakerOfferIDs(c.Request.Context(), owner.MakerID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			prev, err := h.DB.GetOfferPolicy(c.Request.Context(), id)
			from := ""
			if errors.Is(err, pgx.ErrNoRows) {
				prev = defaultPolicy(saved.Mode)
				err = nil
			} else if err == nil {
				from = prev.Mode
			}
			if err != nil {
				return err
			}
			next := prev
			next.Mode = saved.Mode
			if next.MinRelevance == 0 {
				next.MinRelevance = 40
			}
			if next.MaxPlacementsPer30Days == 0 {
				next.MaxPlacementsPer30Days = 3
			}
			if _, err = h.DB.UpsertOfferPolicy(c.Request.Context(), id, next, "owner_ui", owner.ID, from); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *AgentEntry) GetSiteMode(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok || !h.siteAllowed(c, actor, c.Param("id")) {
		return
	}
	policy, source, err := h.sitePolicy(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	c.JSON(http.StatusOK, policyJSON(policy, source))
}

func (h *AgentEntry) PutSiteMode(c *gin.Context) {
	h.putMode(c, "site")
}

func (h *AgentEntry) GetOfferMode(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok || !h.offerAllowed(c, actor, c.Param("id")) {
		return
	}
	policy, source, err := h.offerPolicy(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	c.JSON(http.StatusOK, policyJSON(policy, source))
}

func (h *AgentEntry) PutOfferMode(c *gin.Context) {
	h.putMode(c, "offer")
}

func (h *AgentEntry) putMode(c *gin.Context, scope string) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	if scope == "site" && !h.siteAllowed(c, actor, c.Param("id")) {
		return
	}
	if scope == "offer" && !h.offerAllowed(c, actor, c.Param("id")) {
		return
	}
	body, err := readJSONBody(c, 8192)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if rejectUntrusted(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "untrusted text cannot change policy"})
		return
	}
	var req modeBody
	if decodeStrict(body, &req) != nil || !entry.ValidMode(req.Mode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be manual, auto, or yolo"})
		return
	}
	var prev database.ApprovalPolicy
	if scope == "site" {
		prev, _, err = h.sitePolicy(c, c.Param("id"))
	} else {
		prev, _, err = h.offerPolicy(c, c.Param("id"))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	next, err := applyModeBody(prev, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if (actor.Kind == "agent_tool" || credentialPresented(c)) && (!entry.AgentMaySetMode(prev.Mode, next.Mode) || rulesOf(next).LooserThan(rulesOf(prev))) {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent tools can only tighten a mode"})
		return
	}
	var version int
	if scope == "site" {
		version, err = h.DB.UpsertSitePolicy(c.Request.Context(), c.Param("id"), next, actor.Kind, actor.UserID, prev.Mode)
	} else {
		version, err = h.DB.UpsertOfferPolicy(c.Request.Context(), c.Param("id"), next, actor.Kind, actor.UserID, prev.Mode)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode was not saved"})
		return
	}
	next.Version = version
	c.JSON(http.StatusOK, policyJSON(next, "policy"))
}

func (h *AgentEntry) KillSwitch(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok || !h.siteAllowed(c, actor, c.Param("id")) {
		return
	}
	body, err := readJSONBody(c, 1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	enabled := true
	if len(bytes.TrimSpace(body)) > 0 {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if decodeStrict(body, &req) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
	}
	prev, _, err := h.sitePolicy(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kill switch failed"})
		return
	}
	if !enabled {
		if credentialPresented(c) || actor.Kind != "owner_ui" {
			c.JSON(http.StatusForbidden, gin.H{"error": "turning the kill switch off requires a signed-in dashboard session"})
			return
		}
		mode, unpaused, err := h.DB.ReleaseSiteKillSwitch(c.Request.Context(), c.Param("id"), actor.Kind, actor.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "kill switch failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"mode": mode, "kill_switch": false, "paused": 0, "unpaused": unpaused})
		return
	}
	if (actor.Kind == "agent_tool" || credentialPresented(c)) && !entry.AgentMaySetMode(prev.Mode, entry.ModeManual) {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent tools can only tighten a mode"})
		return
	}
	paused, err := h.DB.KillSiteSwitch(c.Request.Context(), c.Param("id"), actor.Kind, actor.UserID, prev.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kill switch failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mode": "manual", "kill_switch": true, "paused": paused})
}

func (h *AgentEntry) DecidePlacement(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	body, err := readJSONBody(c, 8192)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		Relevance    int    `json:"relevance"`
		Position     string `json:"position"`
		RelSponsored bool   `json:"rel_sponsored"`
		CopyEdited   bool   `json:"copy_edited"`
		OfferData    string `json:"offer_data"`
	}
	if decodeStrict(body, &req) != nil || !entry.ValidPosition(req.Position) || req.Relevance < 0 || req.Relevance > 100 || utf8.RuneCountInString(req.OfferData) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "relevance, position, and rel_sponsored required"})
		return
	}
	siteID := actor.SiteID
	if siteID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "publisher scope required"})
		return
	}
	placement, err := h.DB.GetPlacement(c.Request.Context(), siteID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	live, makerID, category, err := h.DB.OfferFacts(c.Request.Context(), placement.OfferID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "offer lookup failed"})
		return
	}
	boxes, err := h.DB.CountPlacementBoxes(c.Request.Context(), siteID, placement.SurfaceURL, placement.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	used, err := h.DB.CountPolicyPlacements(c.Request.Context(), siteID, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	policy, _, err := h.sitePolicy(c, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	decision := entry.EvaluatePlacement(policy.Mode, rulesOf(policy), entry.PlacementFacts{
		Disclosure: placement.Disclosure, RelSponsored: req.RelSponsored, Copy: placement.Copy, OfferData: req.OfferData,
		ExistingBoxes: boxes, Relevance: req.Relevance, OfferLive: live, RevokeWorks: placement.Status == "pending" || placement.Status == "approved" || placement.Status == "committed",
		Position: req.Position, MakerID: makerID, Category: category, CopyEdited: req.CopyEdited, PlacementsIn30Days: used,
	}, h.claims())
	if len(decision.Failures) > 0 && !decision.NeedsOwner {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"decision": "denied", "failures": decision.Failures})
		return
	}
	if !decision.Allowed {
		c.JSON(http.StatusOK, gin.H{"decision": "needs_owner", "failures": decision.Failures})
		return
	}
	updated, err := h.DB.RecordPolicyApproval(c.Request.Context(), placement.ID, siteID, actor.UserID, decision.ApprovedBy, decision.MatchedRule, policy.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusConflict, gin.H{"error": "placement cannot be approved"})
		return
	}
	c.JSON(http.StatusOK, approvalJSON(decision, policy.Version))
}

func (h *AgentEntry) AcceptPublisher(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok || !h.offerAllowed(c, actor, c.Param("id")) {
		return
	}
	body, err := readJSONBody(c, 4096)
	if err != nil || rejectUntrusted(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		SiteID      string `json:"site_id"`
		SiteQuality int    `json:"site_quality"`
		Niche       string `json:"niche"`
	}
	if decodeStrict(body, &req) != nil || !uuidPattern.MatchString(req.SiteID) || req.SiteQuality < 0 || req.SiteQuality > 100 || utf8.RuneCountInString(req.Niche) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id, site_quality, and niche required"})
		return
	}
	live, _, _, err := h.DB.OfferFacts(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
		return
	}
	policy, _, err := h.offerPolicy(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	decision := entry.EvaluatePublisher(policy.Mode, rulesOf(policy), entry.PublisherFacts{OfferLive: live, SiteQuality: req.SiteQuality, Niche: req.Niche})
	if len(decision.Failures) > 0 && !decision.NeedsOwner {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"decision": "denied", "failures": decision.Failures})
		return
	}
	if !decision.Allowed {
		c.JSON(http.StatusOK, gin.H{"decision": "needs_owner", "failures": decision.Failures})
		return
	}
	if err = h.DB.RecordPublisherAcceptance(c.Request.Context(), c.Param("id"), req.SiteID, decision.ApprovedBy, decision.MatchedRule, policy.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acceptance failed"})
		return
	}
	c.JSON(http.StatusOK, approvalJSON(decision, policy.Version))
}

func (h *AgentEntry) CreateListing(c *gin.Context) {
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	if actor.MakerID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "maker scope required"})
		return
	}
	body, err := readJSONBody(c, 8192)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listing card rejected"})
		return
	}
	upload, err := entry.ParseListingUpload(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listing card rejected"})
		return
	}
	if upload.ProductID != "" {
		owned, err := h.DB.MakerOwnsProduct(c.Request.Context(), actor.MakerID, upload.ProductID)
		if err != nil || !owned {
			c.JSON(http.StatusForbidden, gin.H{"error": "product is not on this account"})
			return
		}
	}
	card, _ := json.Marshal(upload.Card)
	signals, _ := json.Marshal(upload.Signals)
	id, err := h.DB.InsertListingCard(c.Request.Context(), database.ListingCardRow{
		MakerID: actor.MakerID, OwnerID: actor.UserID, ProductID: upload.ProductID, Card: card, Signals: signals,
		Score: upload.Score, PaymentsCheck: upload.PaymentsCheck, OfferLinkHealth: upload.OfferLinkHealth,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "listing card was not stored"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "score": upload.Score, "payments_check": upload.PaymentsCheck, "offer_link_health": upload.OfferLinkHealth})
}

func (h *AgentEntry) GetListing(c *gin.Context) {
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	if !uuidPattern.MatchString(c.Param("id")) || actor.MakerID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing card not found"})
		return
	}
	row, err := h.DB.GetListingCard(c.Request.Context(), c.Param("id"), actor.MakerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing card not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": row.ID, "card": json.RawMessage(row.Card), "signals": json.RawMessage(row.Signals), "score": row.Score, "payments_check": row.PaymentsCheck, "offer_link_health": row.OfferLinkHealth})
}

func (h *AgentEntry) RecordEvent(c *gin.Context) {
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	body, err := readJSONBody(c, 1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if decodeStrict(body, &req) != nil || (req.Kind != "digest_opened" && req.Kind != "dashboard_opened") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be digest_opened or dashboard_opened"})
		return
	}
	at := time.Now().UTC()
	if err = h.DB.InsertOwnerEvent(c.Request.Context(), req.Kind, owner.ID, "", "", at); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event was not stored"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"kind": req.Kind, "user_id": owner.ID, "recorded_at": at.Format(time.RFC3339)})
}

func (h *AgentEntry) Digest(c *gin.Context) {
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	since, err := h.DB.LastDigestOpen(c.Request.Context(), owner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	floor := time.Unix(0, 0).UTC()
	if since != nil {
		floor = *since
	}
	items := []gin.H{}
	if owner.SiteID != "" {
		placements, err := h.DB.DigestPlacements(c.Request.Context(), owner.SiteID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		for _, item := range placements {
			token, err := h.issueUndo(c, owner.ID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			items = append(items, digestJSON(item, token))
		}
	}
	if owner.MakerID != "" {
		offers, err := h.DB.DigestOffers(c.Request.Context(), owner.MakerID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		for _, item := range offers {
			token, err := h.issueUndo(c, owner.ID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			items = append(items, digestJSON(item, token))
		}
	}
	var sinceOut any
	if since != nil {
		sinceOut = since.UTC().Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, gin.H{"since": sinceOut, "items": items, "session": gin.H{"signed_in": true}})
}

func (h *AgentEntry) issueUndo(c *gin.Context, ownerID string, item database.DigestItem) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	placementID, offerID := "", ""
	if item.Kind == "placement" {
		placementID = item.ID
	} else {
		offerID = item.ID
	}
	err = h.DB.StoreUndoToken(c.Request.Context(), services.HashAPIKey(token), ownerID, item.Kind, placementID, offerID, time.Now().Add(7*24*time.Hour))
	return token, err
}

func (h *AgentEntry) Undo(c *gin.Context) {
	owner, ok := h.sessionOwner(c)
	if !ok {
		return
	}
	body, err := readJSONBody(c, 2048)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		UndoToken string `json:"undo_token"`
	}
	if decodeStrict(body, &req) != nil || req.UndoToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "undo_token required"})
		return
	}
	kind, placementID, offerID, err := h.DB.ConsumeUndo(c.Request.Context(), services.HashAPIKey(req.UndoToken), owner.ID, time.Now().UTC())
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "undo token not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "undo failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "undone", "kind": kind, "placement_id": placementID, "offer_id": offerID, "user_id": owner.ID})
}

func (h *AgentEntry) RevokePlacement(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	reason, ok := revokeReason(c)
	if !ok {
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	placement, err := h.DB.GetPlacementByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revocation failed"})
		return
	}
	side, userID, ok := h.revokeSide(c, actor, placement)
	if !ok {
		return
	}
	if reason == "undo" && userID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "undo requires an owner"})
		return
	}
	now := h.now()
	updated, err := h.DB.RevokePlacementForSide(c.Request.Context(), placement.ID, side, userID, actorKeyHash(c), actor.Kind, reason, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revocation failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	body := gin.H{"status": "revoked", "revoked_by_side": side, "undo": reason == "undo"}
	if reason == "undo" {
		body["undone_at"] = now.Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, body)
}

const restoreWindow = 6 * time.Second

func (h *AgentEntry) RestorePlacement(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	row, err := h.DB.PlacementUndo(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	if row.Status != "revoked" || row.UndoneAt == nil || row.StatusBefore == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "placement was not undone"})
		return
	}
	userID := actor.UserID
	if actor.Owner != nil {
		userID = actor.Owner.ID
	}
	if subtle.ConstantTimeCompare([]byte(row.KeyHash), []byte(actorKeyHash(c))) != 1 || row.UserID == "" || row.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the identity that undid this placement can restore it"})
		return
	}
	now := h.now()
	if now.Sub(row.UndoneAt.UTC()) > restoreWindow {
		c.JSON(http.StatusConflict, gin.H{"error": "restore window closed"})
		return
	}
	failures, err := h.restoreHardFailures(c, row)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	if len(failures) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"decision": "denied", "failures": failures})
		return
	}
	status, approvedBy, restored, err := h.DB.RestoreUndonePlacement(c.Request.Context(), c.Param("id"), userID, actorKeyHash(c), actor.Kind, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	if !restored {
		c.JSON(http.StatusConflict, gin.H{"error": "restore window closed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "approved_by": approvedBy, "restored_by": userID, "actor": actor.Kind})
}

func (h *AgentEntry) restoreHardFailures(c *gin.Context, row database.PlacementUndo) ([]string, error) {
	live, _, _, err := h.DB.OfferFacts(c.Request.Context(), row.OfferID)
	if err != nil {
		return nil, err
	}
	boxes, err := h.DB.CountPlacementBoxes(c.Request.Context(), row.SiteID, row.SurfaceURL, c.Param("id"))
	if err != nil {
		return nil, err
	}
	decision := entry.EvaluatePlacement(entry.ModeYolo, entry.AutoRules{}, entry.PlacementFacts{
		Disclosure: row.Disclosure, RelSponsored: true, Copy: row.Copy,
		ExistingBoxes: boxes, Relevance: 100, OfferLive: live, RevokeWorks: true, Position: "end",
	}, h.claims())
	return decision.Failures, nil
}

func (h *AgentEntry) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func actorKeyHash(c *gin.Context) string {
	if !credentialPresented(c) {
		return ""
	}
	return services.HashAPIKey(presentedKey(c))
}

func revokeReason(c *gin.Context) (string, bool) {
	body, err := readJSONBody(c, 1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return "", true
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if decodeStrict(body, &req) != nil || (req.Reason != "" && req.Reason != "undo") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason must be undo"})
		return "", false
	}
	return req.Reason, true
}

func (h *AgentEntry) revokeSide(c *gin.Context, actor entryActor, placement *database.Placement) (string, string, bool) {
	userID := actor.UserID
	if actor.Owner != nil {
		userID = actor.Owner.ID
		if actor.Owner.SiteID != "" && actor.Owner.SiteID == placement.SiteID {
			return "publisher", userID, true
		}
		if h.makerOwnsOffer(c, actor.Owner.MakerID, placement.OfferID) {
			return "maker", userID, true
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "placement is not on this account"})
		return "", "", false
	}
	if actor.SiteID != "" && actor.SiteID == placement.SiteID {
		return "publisher", userID, true
	}
	if h.makerOwnsOffer(c, actor.MakerID, placement.OfferID) {
		return "maker", userID, true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "placement is not on this account"})
	return "", "", false
}

func (h *AgentEntry) makerOwnsOffer(c *gin.Context, makerID, offerID string) bool {
	if makerID == "" {
		return false
	}
	_, offerMaker, _, err := h.DB.OfferFacts(c.Request.Context(), offerID)
	return err == nil && offerMaker == makerID
}

func (h *AgentEntry) FromSource(c *gin.Context) {
	source := database.NormalizeAttributionSource(c.Param("source"))
	if err := h.DB.CountSourceHit(c.Request.Context(), source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redirect unavailable"})
		return
	}
	c.Redirect(http.StatusFound, h.landingLocation())
}

func (h *AgentEntry) LLMs(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(llmsText))
}

func (h *AgentEntry) Doc(c *gin.Context) {
	body, ok := agentDoc(c.Param("name"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(body))
}

func (h *AgentEntry) sessionOwner(c *gin.Context) (*database.Owner, bool) {
	if h.Owner == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign in required"})
		return nil, false
	}
	h.Owner(c)
	if c.IsAborted() {
		return nil, false
	}
	return ownerFrom(c), true
}

func (h *AgentEntry) actorFrom(c *gin.Context) (entryActor, bool) {
	if c.GetHeader("X-API-Key") != "" {
		middleware.APIKeyAuth(h.DB, nil)(c)
		if c.IsAborted() {
			return entryActor{}, false
		}
		siteID := c.GetString("site_id")
		userID, _ := h.DB.OwnerIDBySite(c.Request.Context(), siteID)
		return entryActor{Kind: "agent_tool", UserID: userID, SiteID: siteID}, true
	}
	if c.GetHeader("X-Maker-Key") != "" {
		middleware.MakerKeyAuth(h.DB)(c)
		if c.IsAborted() {
			return entryActor{}, false
		}
		makerID := c.GetString("maker_id")
		userID, _ := h.DB.OwnerIDByMaker(c.Request.Context(), makerID)
		return entryActor{Kind: "agent_tool", UserID: userID, MakerID: makerID, OfferID: c.GetString("offer_scope_id")}, true
	}
	owner, ok := h.sessionOwner(c)
	if !ok {
		return entryActor{}, false
	}
	return entryActor{Kind: "owner_ui", UserID: owner.ID, SiteID: owner.SiteID, MakerID: owner.MakerID, Owner: owner}, true
}

func (h *AgentEntry) siteAllowed(c *gin.Context, actor entryActor, siteID string) bool {
	if actor.Kind == "owner_ui" && actor.SiteID == siteID {
		return true
	}
	if actor.Kind == "agent_tool" && actor.SiteID == siteID {
		return true
	}
	if !c.IsAborted() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "site is not on this account"})
	}
	return false
}

func (h *AgentEntry) offerAllowed(c *gin.Context, actor entryActor, offerID string) bool {
	if actor.OfferID != "" && actor.OfferID != offerID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "key is scoped to a different offer"})
		return false
	}
	if actor.MakerID == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "maker scope required"})
		return false
	}
	_, makerID, _, err := h.DB.OfferFacts(c.Request.Context(), offerID)
	if err != nil || makerID != actor.MakerID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "offer is not on this account"})
		return false
	}
	return true
}

func (h *AgentEntry) sitePolicy(c *gin.Context, siteID string) (database.ApprovalPolicy, string, error) {
	policy, err := h.DB.GetSitePolicy(c.Request.Context(), siteID)
	if err == nil {
		return policy, "policy", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return database.ApprovalPolicy{}, "", err
	}
	ownerID, ownerErr := h.DB.OwnerIDBySite(c.Request.Context(), siteID)
	if ownerErr == nil {
		saved, setupErr := h.DB.GetSetup(c.Request.Context(), ownerID, "publisher")
		if setupErr == nil {
			policy = defaultPolicy(saved.Mode)
			if saved.BoxPosition != "" {
				policy.AllowedPositions = []string{saved.BoxPosition}
			}
			return policy, "setup", nil
		}
	}
	return defaultPolicy(entry.ModeManual), "default", nil
}

func (h *AgentEntry) offerPolicy(c *gin.Context, offerID string) (database.ApprovalPolicy, string, error) {
	policy, err := h.DB.GetOfferPolicy(c.Request.Context(), offerID)
	if err == nil {
		return policy, "policy", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return database.ApprovalPolicy{}, "", err
	}
	_, makerID, _, factErr := h.DB.OfferFacts(c.Request.Context(), offerID)
	if factErr == nil {
		ownerID, ownerErr := h.DB.OwnerIDByMaker(c.Request.Context(), makerID)
		if ownerErr == nil {
			saved, setupErr := h.DB.GetSetup(c.Request.Context(), ownerID, "maker")
			if setupErr == nil {
				return defaultPolicy(saved.Mode), "setup", nil
			}
		}
	}
	return defaultPolicy(entry.ModeManual), "default", nil
}

func defaultPolicy(mode string) database.ApprovalPolicy {
	return database.ApprovalPolicy{Mode: mode, MinRelevance: 40, MaxPlacementsPer30Days: 3}
}

type modeBody struct {
	Mode                   string   `json:"mode"`
	AllowMakers            []string `json:"allow_makers"`
	DenyMakers             []string `json:"deny_makers"`
	AllowCategories        []string `json:"allow_categories"`
	DenyCategories         []string `json:"deny_categories"`
	MinRelevance           *int     `json:"min_relevance"`
	AllowedPositions       []string `json:"allowed_positions"`
	MaxPlacementsPer30Days *int     `json:"max_placements_per_30_days"`
	CopyEditsAllowed       *bool    `json:"copy_edits_allowed"`
	MinSiteQuality         *int     `json:"min_site_quality"`
	AllowedNiches          []string `json:"allowed_niches"`
}

func applyModeBody(prev database.ApprovalPolicy, req modeBody) (database.ApprovalPolicy, error) {
	next := prev
	next.Mode = req.Mode
	if req.AllowMakers != nil {
		next.AllowMakers = req.AllowMakers
	}
	if req.DenyMakers != nil {
		next.DenyMakers = req.DenyMakers
	}
	if req.AllowCategories != nil {
		next.AllowCategories = req.AllowCategories
	}
	if req.DenyCategories != nil {
		next.DenyCategories = req.DenyCategories
	}
	if req.AllowedPositions != nil {
		for _, position := range req.AllowedPositions {
			if !entry.ValidPosition(position) {
				return next, errors.New("unknown position")
			}
		}
		next.AllowedPositions = req.AllowedPositions
	}
	if req.AllowedNiches != nil {
		next.AllowedNiches = req.AllowedNiches
	}
	if req.MinRelevance != nil {
		value, err := entry.NormalizeMinRelevance(*req.MinRelevance, true)
		if err != nil {
			return next, err
		}
		next.MinRelevance = value
	} else if next.MinRelevance == 0 {
		next.MinRelevance = 40
	}
	if req.MaxPlacementsPer30Days != nil {
		if *req.MaxPlacementsPer30Days < 0 || *req.MaxPlacementsPer30Days > 100 {
			return next, errors.New("max placements must be from 0 to 100")
		}
		next.MaxPlacementsPer30Days = *req.MaxPlacementsPer30Days
	} else if next.MaxPlacementsPer30Days == 0 {
		next.MaxPlacementsPer30Days = 3
	}
	if req.CopyEditsAllowed != nil {
		next.CopyEditsAllowed = *req.CopyEditsAllowed
	}
	if req.MinSiteQuality != nil {
		if *req.MinSiteQuality < 0 || *req.MinSiteQuality > 100 {
			return next, errors.New("min site quality must be from 0 to 100")
		}
		next.MinSiteQuality = *req.MinSiteQuality
	}
	return next, nil
}

func rulesOf(p database.ApprovalPolicy) entry.AutoRules {
	return entry.AutoRules{
		AllowMakers: p.AllowMakers, DenyMakers: p.DenyMakers, AllowCategories: p.AllowCategories, DenyCategories: p.DenyCategories,
		MinRelevance: p.MinRelevance, AllowedPositions: p.AllowedPositions, MaxPlacementsPer30Days: p.MaxPlacementsPer30Days,
		CopyEditsAllowed: p.CopyEditsAllowed, MinSiteQuality: p.MinSiteQuality, AllowedNiches: p.AllowedNiches,
	}
}

func policyJSON(p database.ApprovalPolicy, source string) gin.H {
	return gin.H{
		"mode": p.Mode, "version": p.Version, "source": source,
		"allow_makers": listOrEmpty(p.AllowMakers), "deny_makers": listOrEmpty(p.DenyMakers),
		"allow_categories": listOrEmpty(p.AllowCategories), "deny_categories": listOrEmpty(p.DenyCategories),
		"min_relevance": p.MinRelevance, "allowed_positions": listOrEmpty(p.AllowedPositions),
		"max_placements_per_30_days": p.MaxPlacementsPer30Days, "copy_edits_allowed": p.CopyEditsAllowed,
		"min_site_quality": p.MinSiteQuality, "allowed_niches": listOrEmpty(p.AllowedNiches),
		"kill_switch": p.KillSwitch,
	}
}

func setupJSON(s database.SetupSettings, saved, human, signedIn bool) gin.H {
	body := gin.H{
		"saved": saved, "role": s.Role, "install_path": s.InstallPath, "mode": s.Mode, "mode_shown": s.ModeShown,
		"mode_preselected": entry.DefaultMode(s.InstallPath),
		"promote":          s.Promote, "avoid_topics": listOrEmpty(s.AvoidTopics), "commission": s.Commission,
		"sites_channels": listOrEmpty(s.SitesChannels), "never_topics": listOrEmpty(s.NeverTopics), "box_position": s.BoxPosition,
		"human_login": human, "session": gin.H{"signed_in": signedIn},
	}
	if saved {
		body["saved_by"] = s.SavedBy
		body["saved_at"] = s.SavedAt.UTC().Format(time.RFC3339)
	}
	return body
}

func approvalJSON(decision entry.Decision, version int) gin.H {
	ruleID, ruleText := entry.PresentRule(decision.MatchedRule)
	return gin.H{
		"decision": "approved", "approved_by": decision.ApprovedBy, "policy_version": version, "matched_rule": decision.MatchedRule,
		"policy_rule_id": ruleID, "policy_rule_text": ruleText,
	}
}

func digestJSON(item database.DigestItem, token string) gin.H {
	return gin.H{"kind": item.Kind, "id": item.ID, "summary": item.Summary, "went_live_at": item.WentLiveAt.UTC().Format(time.RFC3339), "undo_token": token}
}

func listOrEmpty(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func shortList(items []string) bool {
	if len(items) > 20 {
		return false
	}
	for _, item := range items {
		if utf8.RuneCountInString(item) > 64 || strings.TrimSpace(item) == "" {
			return false
		}
	}
	return true
}

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	return strings.Contains(email, "@") && len(email) <= 255 && !strings.ContainsAny(email, " \t")
}

var untrustedPolicyKeys = map[string]bool{
	"offer_copy": true, "offer_text": true, "page_content": true, "feedback": true,
	"copy": true, "context": true, "published_text": true, "content": true,
	"what_broke": true, "what_worked": true, "task_tried": true, "free_text": true,
}

func rejectUntrusted(body []byte) bool {
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return true
	}
	for key := range raw {
		if untrustedPolicyKeys[strings.ToLower(key)] {
			return true
		}
	}
	return false
}

func readJSONBody(c *gin.Context, limit int64) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	return io.ReadAll(c.Request.Body)
}

func decodeStrict(body []byte, dest any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}

func newUserCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i, b := range buf {
		out[i] = userCodeAlphabet[int(b)%len(userCodeAlphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:]), nil
}

func canonicalUserCode(raw string) (string, bool) {
	raw = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), "-", ""))
	if len(raw) != 8 {
		return "", false
	}
	for _, r := range raw {
		if !strings.ContainsRune(userCodeAlphabet, r) {
			return "", false
		}
	}
	return raw[:4] + "-" + raw[4:], true
}
