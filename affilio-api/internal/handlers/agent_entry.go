package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"affilio-api/internal/config"
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
	TokenLimiter   *IPRateLimiter
	// AddressKey is the HMAC key for open-signup client addresses. It is
	// API_KEY_SECRET, or a random per-process key when that secret is unset.
	AddressKey []byte
	// ListenHost is the HOST this process binds. Empty and 0.0.0.0 listen on
	// every interface. A loopback Host from a non-loopback peer is refused
	// unless this is exactly 127.0.0.1, localhost, or ::1.
	ListenHost string
	// Render is true when RENDER=true. Open-signup limits then use the client
	// address Render appended, the same key as landing events.
	Render bool
	// TrustedPlatform is a dedicated client-IP header. When set, it is the
	// open-signup address.
	TrustedPlatform string
	// beforeJoinBearer runs after the step actor is loaded and before the
	// join-bearer check. Tests revoke the token between those two lookups.
	beforeJoinBearer   func()
	DeviceLimiter      *IPRateLimiter
	Now                func() time.Time
	PluginOfflineAfter time.Duration
	StatusLimiter      *IPRateLimiter
	// ClaimsCheck is the resolved CLAIMS_CHECK mode. Listing-card reads use it
	// so a hosted stub reports the not-live claims message.
	ClaimsCheck string
	// AssetsPath is the hosted UI directory. When it contains
	// landing-v9/llms.txt, GET /llms.txt serves that file.
	AssetsPath string
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
	r.POST("/v1/agent-tokens", h.CreateAgentToken)
	r.GET("/v1/agent-tokens", h.ListAgentTokens)
	r.POST("/v1/agent-tokens/revoke", h.RevokeAgentToken)
	r.POST("/v1/agent-tokens/:id/revoke", h.RevokeAgentTokenByID)
	r.GET("/v1/step-modes", h.GetStepModes)
	r.PUT("/v1/step-modes", h.PutStepModes)
	r.GET("/v1/step-approvals/:id", h.GetStepApproval)
	r.GET("/v1/setup/settings", h.GetSetup)
	r.PUT("/v1/setup/settings", h.PutSetup)
	r.GET("/v1/sites/:id/mode", h.GetSiteMode)
	r.PUT("/v1/sites/:id/mode", h.PutSiteMode)
	r.POST("/v1/sites/:id/kill-switch", h.KillSwitch)
	r.POST("/v1/sites/:id/plugin-key", h.IssuePluginKey)
	r.GET("/v1/sites/:id/plugin-key", h.ListPluginKeys)
	r.POST("/v1/sites/:id/plugin-key/rotate", h.RotatePluginKey)
	r.POST("/v1/sites/:id/plugin-key/revoke", h.RevokePluginKey)
	r.POST("/v1/account/pause", h.AccountPause)
	r.POST("/v1/account/resume", h.AccountResume)
	r.GET("/v1/offers/:id/mode", h.GetOfferMode)
	r.PUT("/v1/offers/:id/mode", h.PutOfferMode)
	r.POST("/v1/placements/:id/decide", h.DecidePlacement)
	r.POST("/v1/placements/:id/revoke", h.RevokePlacement)
	r.POST("/v1/placements/:id/restore", h.RestorePlacement)
	r.POST("/v1/placements/:id/plugin-ack", h.PluginAck)
	r.POST("/v1/placements/:id/retry", h.RetryPlugin)
	r.GET("/v1/placements/:id", h.PlacementStatus)
	r.POST("/v1/offers/:id/accept", h.AcceptPublisher)
	r.POST("/v1/listing-cards", h.CreateListing)
	r.GET("/v1/listing-cards/:id", h.GetListing)
	r.POST("/v1/events", h.RecordEvent)
	r.GET("/v1/digest", h.Digest)
	r.GET("/v1/maker/digest", h.MakerDigest)
	r.GET("/v1/account/digest", h.AccountDigest)
	r.GET("/v1/account/projects", h.AccountProjects)
	r.POST("/v1/account-keys", h.CreateAccountKey)
	r.GET("/v1/account-keys", h.ListAccountKeys)
	r.POST("/v1/account-keys/:id/revoke", h.RevokeAccountKey)
	r.GET("/v1/policy/hard-checks", h.HardChecks)
	r.POST("/v1/digest/undo", h.Undo)
	r.GET("/from/:source", h.FromSource)
	r.GET("/llms.txt", h.LLMs)
	r.HEAD("/llms.txt", h.LLMs)
	r.GET("/llms-full.txt", h.LLMsFull)
	r.HEAD("/llms-full.txt", h.LLMsFull)
	r.GET("/skill.md", h.Skill)
	r.HEAD("/skill.md", h.Skill)
	r.GET("/docs/:name", h.Doc)
	r.HEAD("/docs/:name", h.Doc)
}

func (h *AgentEntry) claims() entry.ClaimChecker {
	if h.Claims == nil {
		return entry.StubClaimChecker{}
	}
	return h.Claims
}

// placementClaims runs the page checker at restore and retry when this
// process is on CLAIMS_CHECK=real and the decide checker is the stub or the
// page checker. The page is fetched before the transaction. The recorded
// result is what ClaimsFail reads inside it, so a miss leaves the row
// unchanged.
func (h *AgentEntry) placementClaims(c *gin.Context, offerID string) (entry.ClaimChecker, error) {
	if !claimsReal(h.ClaimsCheck) || !merchantPageChecker(h.Claims) {
		return h.claims(), nil
	}
	src, err := h.DB.OfferClaimSource(c.Request.Context(), offerID)
	if err != nil {
		return nil, err
	}
	reason, _, err := compareOfferClaims(c.Request.Context(), h.DB, src.MakerID, src.ProductID, database.Offer{
		LandingURL:      src.LandingURL,
		Terms:           src.Terms,
		CommissionType:  src.CommissionType,
		CommissionValue: src.CommissionValue,
	})
	if err != nil {
		return nil, err
	}
	if reason == "" {
		return entry.RecordedClaimCheck{}, nil
	}
	return entry.RecordedClaimCheck{Err: entry.ClaimMismatch{Reason: reason}}, nil
}

func merchantPageChecker(claims entry.ClaimChecker) bool {
	if claims == nil {
		return true
	}
	switch claims.(type) {
	case entry.StubClaimChecker, entry.PageClaimChecker:
		return true
	default:
		return false
	}
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
		writeNamedConflict(c, "device_used")
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
	ip := h.joinClientIP(c)
	if h.AccountLimiter != nil && !h.AccountLimiter.Allow(ip) {
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
	owners, lookupErr := h.DB.OwnersByEmail(c.Request.Context(), req.Email)
	if lookupErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	gateOwner, gateSite, gateMode, gateErr := signupGate(c, h.DB, owners)
	if gateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	if gateMode != "" && holdStep(c, h.DB, database.StepSiteSignup, gateOwner, gateSite, map[string]any{
		"kind": "agent_account", "role": req.Role, "name": req.Name, "email": strings.ToLower(req.Email), "domain": strings.ToLower(req.Domain), "source": req.Source,
	}) {
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
	pluginKey := ""
	pluginHash := ""
	pluginPrefix := ""
	if req.Role == "publisher" {
		pluginKey, err = generateAPIKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
			return
		}
		pluginHash = services.HashAPIKey(pluginKey)
		pluginPrefix = services.KeyPrefix(pluginKey)
	}
	account, err := h.DB.CreateOpenAgentAccount(c.Request.Context(), "agent:"+subject, strings.ToLower(req.Email), req.Name, database.NormalizeAttributionSource(req.Source), req.Role, strings.ToLower(req.Domain), services.HashAPIKey(siteKey), services.KeyPrefix(siteKey), services.HashAPIKey(makerKey), services.KeyPrefix(makerKey), pluginHash, pluginPrefix, h.clientAddressHash(ip))
	if errors.Is(err, database.ErrJoinLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	if isUniqueViolation(err) {
		writeNamedConflict(c, "account_exists")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account creation failed"})
		return
	}
	resp := gin.H{"owner_id": account.OwnerID, "agent_only": true, "human_login": false, "role": req.Role}
	if req.Role == "publisher" {
		resp["site_id"] = account.SiteID
		resp["api_key"] = siteKey
		resp["key_header"] = "X-API-Key"
		resp["plugin_key"] = pluginKey
		resp["plugin_key_header"] = "X-Plugin-Key"
		resp["verification_token"] = account.VerificationToken
		resp["verified"] = false
		resp["verified_at"] = nil
	} else {
		resp["maker_id"] = account.MakerID
		resp["api_key"] = makerKey
		resp["key_header"] = "X-Maker-Key"
	}
	h.notifyAccountRegistered(c, account.OwnerID, req.Role, req.Name, strings.ToLower(req.Email))
	c.JSON(http.StatusCreated, resp)
}

// notifyAccountRegistered fires the owner's configured webhook when an agent
// opens an account — the "an agent just registered your product" moment the
// owner should hear about. Best-effort and async; a missing webhook is fine.
func (h *AgentEntry) notifyAccountRegistered(c *gin.Context, ownerID, role, name, email string) {
	if ownerID == "" {
		return
	}
	url, secret, err := h.DB.OwnerNotify(c.Request.Context(), ownerID)
	if err != nil || url == "" || secret == "" {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":  "account.registered",
		"role":  role,
		"name":  name,
		"email": email,
	})
	if err != nil {
		return
	}
	go postSignedWebhook(url, secret, payload)
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
		source, sourceErr := h.DB.OwnerSource(c.Request.Context(), ownerID)
		if sourceErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
			return
		}
		path := c.Query("install_path")
		if !entry.ValidInstallPath(path) {
			path = entry.DefaultInstallPath(source)
		}
		mode := entry.DefaultMode(source)
		h.finishSetup(c, setupJSON(database.SetupSettings{Role: role, InstallPath: path, Mode: mode, ModeShown: mode}, false, human, signedIn, source, h.claims(), false), actor.SiteID)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	source, sourceErr := h.DB.OwnerSource(c.Request.Context(), ownerID)
	if sourceErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	held, holdErr := h.yoloHeld(c, actor.SiteID, true, saved.Mode)
	if holdErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	h.finishSetup(c, setupJSON(saved, true, human, signedIn, source, h.claims(), held), actor.SiteID)
}

type setupCommission struct {
	Type     string   `json:"type"`
	Value    *float64 `json:"value"`
	Currency string   `json:"currency"`
}

type setupPrice struct {
	Value    *float64 `json:"value"`
	Currency string   `json:"currency"`
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
		Role          string           `json:"role"`
		InstallPath   string           `json:"install_path"`
		Mode          string           `json:"mode"`
		ModeShown     string           `json:"mode_shown"`
		Promote       string           `json:"promote"`
		AvoidTopics   []string         `json:"avoid_topics"`
		ProjectName   string           `json:"project_name"`
		ListingURL    string           `json:"listing_url"`
		Commission    *setupCommission `json:"commission"`
		ListingPrice  *setupPrice      `json:"listing_price"`
		SitesChannels []string         `json:"sites_channels"`
		NeverTopics   []string         `json:"never_topics"`
		BoxPosition   string           `json:"box_position"`
	}
	if decodeStrict(body, &req) != nil || (req.Role != "maker" && req.Role != "publisher") || !entry.ValidMode(req.Mode) || !entry.ValidMode(req.ModeShown) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role, install_path, mode, and mode_shown required"})
		return
	}
	if strings.TrimSpace(req.InstallPath) == "" {
		source, sourceErr := h.DB.OwnerSource(c.Request.Context(), owner.ID)
		if sourceErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
			return
		}
		req.InstallPath = entry.DefaultInstallPath(source)
	}
	if !entry.ValidInstallPath(req.InstallPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role, install_path, mode, and mode_shown required"})
		return
	}
	if req.Role == "publisher" && req.BoxPosition != "" && !entry.ValidPosition(req.BoxPosition) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown box position"})
		return
	}
	req.ProjectName = strings.TrimSpace(req.ProjectName)
	req.ListingURL = strings.TrimSpace(req.ListingURL)
	if utf8.RuneCountInString(req.Promote) > 280 || utf8.RuneCountInString(req.ProjectName) > 120 || !validListingURL(req.ListingURL) || !shortList(req.AvoidTopics) || !shortList(req.NeverTopics) || !shortList(req.SitesChannels) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings are too long"})
		return
	}
	commissionText, commissionType, commissionValue, commissionCurrency, priceValue, priceCurrency, moneyErr := normalizeSetupMoney(req.Commission, req.ListingPrice)
	if moneyErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": moneyErr})
		return
	}
	now := time.Now().UTC()
	mode := req.Mode
	existing, getErr := h.DB.GetSetup(c.Request.Context(), owner.ID, req.Role)
	if getErr == nil && existing.Mode != "" {
		mode = existing.Mode
	} else if getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
		return
	}
	saved := database.SetupSettings{
		OwnerID: owner.ID, Role: req.Role, InstallPath: req.InstallPath, Mode: mode, ModeShown: req.ModeShown,
		Promote: req.Promote, AvoidTopics: req.AvoidTopics, Commission: commissionText, ProjectName: req.ProjectName, ListingURL: req.ListingURL,
		CommissionType: commissionType, CommissionValue: commissionValue, CommissionCurrency: commissionCurrency,
		ListingPriceValue: priceValue, ListingPriceCurrency: priceCurrency,
		SitesChannels: req.SitesChannels, NeverTopics: req.NeverTopics, BoxPosition: req.BoxPosition, SavedBy: owner.ID, SavedAt: now,
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
	source, sourceErr := h.DB.OwnerSource(c.Request.Context(), owner.ID)
	if sourceErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
		return
	}
	held, holdErr := h.yoloHeld(c, owner.SiteID, true, saved.Mode)
	if holdErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings were not saved"})
		return
	}
	h.finishSetup(c, setupJSON(saved, true, human, true, source, h.claims(), held), owner.SiteID)
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
		if from == "" {
			next.Mode = saved.Mode
		}
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
			if from == "" {
				next.Mode = saved.Mode
			}
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
	h.writePolicy(c, policy, source)
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
	h.writePolicy(c, policy, source)
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
	if scope == "site" {
		next.SiteID = c.Param("id")
	} else {
		next.OfferID = c.Param("id")
	}
	h.writePolicy(c, next, "policy")
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
	paused, err := h.DB.KillSiteSwitch(c.Request.Context(), c.Param("id"), actor.Kind, actor.UserID, prev.Mode, h.now())
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
	if !ok || sessionOnlyDecision(c) {
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
	offer, err := h.DB.OfferCopy(c.Request.Context(), placement.OfferID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "offer lookup failed"})
		return
	}
	boxes, err := h.DB.CountPlacementBoxes(c.Request.Context(), siteID, placement.SurfaceURL, placement.ID, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	used, err := h.DB.CountPolicyPlacements(c.Request.Context(), siteID, h.now().Add(-30*24*time.Hour), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement lookup failed"})
		return
	}
	policy, source, err := h.sitePolicy(c, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
		return
	}
	rules := rulesOf(policy)
	if source == "policy" {
		rules.UseStoredCap = true
	}
	relevance := services.ScorePlacement(placement.Copy, offer.ProductName, offer.Category, offer.Website)
	decision := entry.EvaluatePlacement(policy.Mode, rules, entry.PlacementFacts{
		Disclosure: placement.Disclosure, RelSponsored: true,
		Copy: placement.Copy, OfferData: serverOfferData(offer),
		ExistingBoxes: boxes, Relevance: relevance, OfferLive: offer.Live,
		RevokeWorks: placement.Status == "pending" || placement.Status == "approved" || placement.Status == "committed",
		Position:    req.Position, MakerID: offer.MakerID, Category: offer.Category, CopyEdited: req.CopyEdited, PlacementsIn30Days: used,
	}, h.claims())
	if len(decision.Failures) > 0 && !decision.NeedsOwner {
		writeCheckDenied(c, decision.Failures)
		return
	}
	if !decision.Allowed {
		body := gin.H{"decision": "needs_owner", "failures": decision.Failures, "surface_url": placement.SurfaceURL}
		if decision.Reason != "" {
			body["reason"] = decision.Reason
			body["message"] = decision.Message
		}
		c.JSON(http.StatusOK, body)
		return
	}
	updated, err := h.DB.RecordPolicyApproval(c.Request.Context(), placement.ID, siteID, actor.UserID, decision.ApprovedBy, decision.MatchedRule, policy.Version, relevance, req.Position)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	if !updated {
		writeNamedConflict(c, "not_approvable")
		return
	}
	approved := approvalJSON(decision, policy.Version)
	approved["surface_url"] = placement.SurfaceURL
	c.JSON(http.StatusOK, approved)
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
	decision := entry.EvaluatePublisher(policy.Mode, rulesOf(policy), entry.PublisherFacts{OfferLive: live, SiteQuality: req.SiteQuality, Niche: req.Niche}, h.claims())
	if len(decision.Failures) > 0 && !decision.NeedsOwner {
		writeCheckDenied(c, decision.Failures)
		return
	}
	if !decision.Allowed {
		body := gin.H{"decision": "needs_owner", "failures": decision.Failures}
		if decision.Reason != "" {
			body["reason"] = decision.Reason
			body["message"] = decision.Message
		}
		c.JSON(http.StatusOK, body)
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
	c.JSON(http.StatusCreated, gin.H{"id": id, "score": upload.Score, "payments_check": upload.PaymentsCheck, "offer_link_health": upload.OfferLinkHealth, "review_reason": nil, "message": nil})
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
	reason, message := reviewForMakerProduct(c.Request.Context(), h.DB, h.ClaimsCheck, actor.MakerID, row.ProductID)
	reviewReason, reviewMessage := nullReview(reason, message)
	c.JSON(http.StatusOK, gin.H{"id": row.ID, "card": json.RawMessage(row.Card), "signals": json.RawMessage(row.Signals), "score": row.Score, "payments_check": row.PaymentsCheck, "offer_link_health": row.OfferLinkHealth, "review_reason": reviewReason, "message": reviewMessage})
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

func (h *AgentEntry) digestCaller(c *gin.Context) (owner *database.Owner, actor entryActor, offerFilter string, keyed bool, ok bool) {
	if !credentialPresented(c) {
		session, good := h.sessionOwner(c)
		return session, entryActor{}, "", false, good
	}
	var good bool
	actor, good = h.actorFrom(c)
	if !good {
		return nil, entryActor{}, "", true, false
	}
	if actor.UserID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account scope required"})
		return nil, actor, "", true, false
	}
	owner = &database.Owner{ID: actor.UserID}
	if actor.OfferID != "" {
		owner.MakerID = actor.MakerID
		return owner, actor, actor.OfferID, true, true
	}
	// A site key that is not an account key reads this site only. LoadOwnerScope
	// also returns the owner's maker, which would attach that maker's offers.
	if actor.SiteID != "" && c.GetString("key_kind") != "account" {
		owner.SiteID = actor.SiteID
		return owner, actor, "", true, true
	}
	siteID, makerID, err := h.DB.LoadOwnerScope(c.Request.Context(), actor.UserID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "account scope required"})
		return nil, actor, "", true, false
	}
	if actor.MakerID != "" && actor.SiteID == "" && c.GetString("key_kind") != "account" {
		owner.MakerID = actor.MakerID
		return owner, actor, "", true, true
	}
	owner.SiteID = siteID
	owner.MakerID = makerID
	if actor.SiteID != "" {
		owner.SiteID = actor.SiteID
	}
	return owner, actor, "", true, true
}

func (h *AgentEntry) Digest(c *gin.Context) {
	if c.GetHeader("X-Maker-Key") != "" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "maker key cannot read the owner digest",
			"reason":  "forbidden_key_type",
			"message": "Read this maker's offers with GET /v1/maker/digest.",
		})
		return
	}
	owner, _, offerFilter, keyed, ok := h.digestCaller(c)
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
		floor = since.UTC()
	}
	items := []gin.H{}
	placementTotal := 0
	returnedPlacements := 0
	if owner.SiteID != "" {
		placementTotal, err = h.DB.CountDigestPlacements(c.Request.Context(), owner.SiteID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		placements, err := h.DB.DigestPlacements(c.Request.Context(), owner.SiteID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		returnedPlacements = len(placements)
		for _, item := range placements {
			token, err := h.issueUndo(c, owner.ID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			items = append(items, h.digestJSON(item, token))
		}
	}
	offerTotal := 0
	returnedOffers := 0
	if owner.MakerID != "" {
		offerTotal, err = h.DB.CountDigestOffers(c.Request.Context(), owner.MakerID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		offers, err := h.DB.DigestOffers(c.Request.Context(), owner.MakerID, floor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		if err = h.annotateOfferReviews(c, offers); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		for _, item := range offers {
			if offerFilter != "" && item.ID != offerFilter {
				continue
			}
			token, err := h.issueUndo(c, owner.ID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			items = append(items, h.digestJSON(item, token))
			returnedOffers++
		}
	}
	var sinceOut any
	if since != nil {
		sinceOut = since.UTC().Format(time.RFC3339)
	}
	wentLive := 0
	if owner.SiteID != "" {
		n, countErr := h.DB.CountLivePlacements(c.Request.Context(), owner.SiteID)
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
			return
		}
		wentLive = n
	}
	total := placementTotal + offerTotal
	if offerFilter != "" && offerTotal > 0 {
		total = placementTotal + returnedOffers
		offerTotal = returnedOffers
	}
	truncated := placementTotal > returnedPlacements || offerTotal > returnedOffers
	body := gin.H{
		"since": sinceOut, "items": items, "went_live": wentLive, "session": gin.H{"signed_in": !keyed},
		"truncated": truncated, "total": total,
	}
	if claimsHeld(h.ClaimsCheck) {
		body["claims_check"] = "not_checked"
	}
	if err = h.attachDigestMeta(c, owner, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	conversions, err := h.scopedConversions(c, owner.MakerID, owner.SiteID, offerFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	body["conversions"] = conversions
	c.JSON(http.StatusOK, body)
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
	owner, actor, _, keyed, ok := h.digestCaller(c)
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
	now := time.Now().UTC()
	kind, placementID, offerID, err := h.DB.PeekUndo(c.Request.Context(), services.HashAPIKey(req.UndoToken), owner.ID, now)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "undo token not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "undo failed"})
		return
	}
	if keyed {
		if !h.undoInScope(c, actor, kind, placementID, offerID) {
			return
		}
	}
	kind, placementID, offerID, err = h.DB.ConsumeUndo(c.Request.Context(), services.HashAPIKey(req.UndoToken), owner.ID, now)
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

func (h *AgentEntry) undoInScope(c *gin.Context, actor entryActor, kind, placementID, offerID string) bool {
	if c.GetString("key_kind") == "account" {
		return true
	}
	refuse := func() bool {
		c.JSON(http.StatusForbidden, gin.H{"error": "undo is outside this project key"})
		return false
	}
	if actor.OfferID != "" {
		if kind == "offer" && offerID == actor.OfferID {
			return true
		}
		if kind == "placement" && h.placementField(c, placementID, func(p *database.Placement) bool { return p.OfferID == actor.OfferID }) {
			return true
		}
		if !c.Writer.Written() {
			return refuse()
		}
		return false
	}
	if c.GetString("key_kind") == "project" && actor.SiteID != "" {
		if kind == "placement" && h.placementField(c, placementID, func(p *database.Placement) bool { return p.SiteID == actor.SiteID }) {
			return true
		}
		if !c.Writer.Written() {
			return refuse()
		}
		return false
	}
	if actor.MakerID != "" && actor.SiteID == "" {
		target := offerID
		if kind == "placement" {
			placement, err := h.DB.GetPlacementByID(c.Request.Context(), placementID)
			if err != nil {
				return refuse()
			}
			target = placement.OfferID
		}
		if target != "" && h.makerOwnsOffer(c, actor.MakerID, target) {
			return true
		}
		return refuse()
	}
	if actor.SiteID != "" && kind == "placement" && h.placementField(c, placementID, func(p *database.Placement) bool { return p.SiteID == actor.SiteID }) {
		return true
	}
	if !c.Writer.Written() {
		return refuse()
	}
	return false
}

func (h *AgentEntry) placementField(c *gin.Context, placementID string, match func(*database.Placement) bool) bool {
	placement, err := h.DB.GetPlacementByID(c.Request.Context(), placementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, gin.H{"error": "undo is outside this project key"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "undo failed"})
		return false
	}
	if !match(placement) {
		c.JSON(http.StatusForbidden, gin.H{"error": "undo is outside this project key"})
		return false
	}
	return true
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
	if !ok || sessionOnlyDecision(c) {
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
		writeNamedConflict(c, "not_undone")
		return
	}
	userID := actor.UserID
	if actor.Owner != nil {
		userID = actor.Owner.ID
	}
	sameUser := row.UserID != "" && row.UserID == userID
	sameKey := subtle.ConstantTimeCompare([]byte(row.KeyHash), []byte(actorKeyHash(c))) == 1
	if !sameUser || (actor.Kind != "owner_ui" && !sameKey) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the identity that undid this placement can restore it"})
		return
	}
	now := h.now()
	if now.Sub(row.UndoneAt.UTC()) > restoreWindow {
		writeRestoreRefused(c, "window_expired")
		return
	}
	failures, err := h.restoreHardFailures(c, row)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	if len(failures) > 0 {
		writeCheckDenied(c, failures)
		return
	}
	policy, source, err := h.sitePolicy(c, row.SiteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	rules := rulesOf(policy)
	if source == "policy" {
		rules.UseStoredCap = true
	}
	claims, claimErr := h.placementClaims(c, row.OfferID)
	if claimErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	status, approvedBy, block, restored, err := h.DB.RestoreUndonePlacement(c.Request.Context(), c.Param("id"), userID, actorKeyHash(c), actor.Kind, now, entry.CapLimit(rules), claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restore failed"})
		return
	}
	if block == "cap" || block == "claims_from_offer" {
		writeCheckDenied(c, []string{block})
		return
	}
	if block == "paused" {
		writeRestoreRefused(c, "paused")
		return
	}
	if !restored {
		writeRestoreRefused(c, "window_expired")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "approved_by": approvedBy, "restored_by": userID, "actor": actor.Kind})
}

// revokePathAvailable is true when this placement can still be undone, or
// when it is revoked because that undo already ran.
func revokePathAvailable(row database.PlacementUndo) bool {
	switch row.Status {
	case "pending", "approved", "committed":
		return true
	case "revoked":
		return row.UndoneAt != nil && row.StatusBefore != ""
	default:
		return false
	}
}

func (h *AgentEntry) restoreHardFailures(c *gin.Context, row database.PlacementUndo) ([]string, error) {
	offer, err := h.DB.OfferCopy(c.Request.Context(), row.OfferID)
	if err != nil {
		return nil, err
	}
	boxes, err := h.DB.CountPlacementBoxes(c.Request.Context(), row.SiteID, row.SurfaceURL, c.Param("id"), 0)
	if err != nil {
		return nil, err
	}
	policy, source, err := h.sitePolicy(c, row.SiteID)
	if err != nil {
		return nil, err
	}
	rules := rulesOf(policy)
	if source == "policy" {
		rules.UseStoredCap = true
	}
	used, err := h.DB.CountPolicyPlacements(c.Request.Context(), row.SiteID, h.now().Add(-30*24*time.Hour), c.Param("id"))
	if err != nil {
		return nil, err
	}
	decision := entry.EvaluatePlacement(entry.ModeYolo, rules, entry.PlacementFacts{
		Disclosure: row.Disclosure, RelSponsored: true,
		Copy: row.Copy, OfferData: serverOfferData(offer),
		ExistingBoxes: boxes, Relevance: services.ScorePlacement(row.Copy, offer.ProductName, offer.Category, offer.Website),
		OfferLive: offer.Live, RevokeWorks: revokePathAvailable(row),
		PlacementsIn30Days: used,
	}, entry.UncheckedClaims{})
	return decision.Failures, nil
}

func serverOfferData(offer database.OfferCopy) string {
	return strings.TrimSpace(strings.Join([]string{offer.ProductName, offer.Category, offer.Website, offer.Terms, offer.Commission}, "\n"))
}

func (h *AgentEntry) PluginAck(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	// The plugin sends only X-Plugin-Key. A site, project, account, agent, or
	// maker key on this route must not be able to set placed.
	if c.GetHeader("X-API-Key") != "" || c.GetHeader("Authorization") != "" || c.GetHeader("X-Maker-Key") != "" {
		writePluginAckForbidden(c, "plugin_key_required", "Send the plugin key in X-Plugin-Key.", "plugin key required")
		return
	}
	rawKey := strings.TrimSpace(c.GetHeader("X-Plugin-Key"))
	if rawKey == "" {
		writePluginAckForbidden(c, "plugin_key_required", "Send the plugin key in X-Plugin-Key.", "plugin key required")
		return
	}
	siteID, err := h.DB.PluginSite(c.Request.Context(), services.HashAPIKey(rawKey))
	if err != nil || siteID == "" {
		writePluginAckForbidden(c, "plugin_key_required", "Send the plugin key in X-Plugin-Key.", "plugin key required")
		return
	}
	body, err := readJSONBody(c, 65536)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var req struct {
		Action       string `json:"action"`
		Reason       string `json:"reason"`
		RelSponsored bool   `json:"rel_sponsored"`
	}
	if decodeStrict(body, &req) != nil || (req.Action != "placed" && req.Action != "removed" && req.Action != "restored" && req.Action != "failed") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be placed, removed, restored, or failed"})
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Action == "failed" && (req.Reason == "" || utf8.RuneCountInString(req.Reason) > 500) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}
	placement, err := h.DB.GetPlacementByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acknowledgement failed"})
		return
	}
	if placement.SiteID != siteID {
		writePluginAckForbidden(c, "wrong_site", "This placement is not on this site.", "placement is not on this site")
		return
	}
	wait, _, _, _, _, waitErr := h.DB.PlacementPlugin(c.Request.Context(), placement.ID)
	if waitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acknowledgement failed"})
		return
	}
	// A pending or approved row is not live. Refuse before rel_sponsored and
	// one_per_post so the caller sees not_committed rather than a later check.
	if entry.PluginAckNeedsCommit(req.Action, wait, placement.Status) {
		writeNamedConflict(c, "not_committed")
		return
	}
	if req.Action == "placed" || req.Action == "restored" {
		if (wait == "" || wait == "restore") && !req.RelSponsored {
			writeCheckDenied(c, []string{"rel_sponsored"})
			return
		}
		boxes, boxErr := h.DB.CountPlacementBoxes(c.Request.Context(), siteID, placement.SurfaceURL, placement.ID, 0)
		if boxErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "acknowledgement failed"})
			return
		}
		if boxes > 0 {
			writeCheckDenied(c, []string{"one_per_post"})
			return
		}
	}
	acked, err := h.DB.AckPluginPlacement(c.Request.Context(), placement.ID, siteID, req.Action, req.Reason, h.now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acknowledgement failed"})
		return
	}
	if !acked {
		writeNamedConflict(c, "nothing_to_ack")
		return
	}
	state, reason, action, err := h.pluginView(c, placement.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acknowledgement failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plugin_state": state, "plugin_failed_reason": reason, "failed_action": action})
}

func (h *AgentEntry) RetryPlugin(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	if actor.SiteID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "publisher side required"})
		return
	}
	placement, err := h.DB.GetPlacementByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
		return
	}
	if placement.SiteID != actor.SiteID {
		c.JSON(http.StatusForbidden, gin.H{"error": "publisher side required"})
		return
	}
	state, _, action, err := h.pluginView(c, placement.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
		return
	}
	if state != "failed" || (action != "removed" && action != "restored") {
		writeNamedConflict(c, "not_failed")
		return
	}
	if action == "restored" {
		row, err := h.DB.PlacementUndo(c.Request.Context(), placement.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
			return
		}
		failures, err := h.restoreHardFailures(c, row)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
			return
		}
		if len(failures) > 0 {
			writeCheckDenied(c, failures)
			return
		}
	}
	claims := h.claims()
	if action == "restored" {
		var claimErr error
		claims, claimErr = h.placementClaims(c, placement.OfferID)
		if claimErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
			return
		}
	}
	retried, block, err := h.DB.RetryPluginPlacement(c.Request.Context(), placement.ID, actor.SiteID, action, h.now(), claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
		return
	}
	if block == "claims_from_offer" {
		writeCheckDenied(c, []string{block})
		return
	}
	if !retried {
		writeNamedConflict(c, "not_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"plugin_state": "pending", "plugin_failed_reason": "", "failed_action": ""})
}

func (h *AgentEntry) PlacementStatus(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid placement id"})
		return
	}
	actor, ok := h.actorFrom(c)
	if !ok {
		return
	}
	siteID := actor.SiteID
	if siteID == "" {
		if actor.MakerID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "site key required"})
			return
		}
		placement, err := h.DB.GetPlacementByID(c.Request.Context(), c.Param("id"))
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "placement status unavailable"})
			return
		}
		_, makerID, _, err := h.DB.OfferFacts(c.Request.Context(), placement.OfferID)
		if err != nil || makerID != actor.MakerID || (actor.OfferID != "" && actor.OfferID != placement.OfferID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "offer is not on this key"})
			return
		}
		siteID = placement.SiteID
	}
	writePlacementStatus(c, h.DB, h.StatusLimiter, h.now(), h.pluginWindow(), siteID, actor.UserID)
}

func (h *AgentEntry) pluginView(c *gin.Context, id string) (state, failedReason, failedAction string, err error) {
	wait, waitAt, _, failedReason, failedAction, err := h.DB.PlacementPlugin(c.Request.Context(), id)
	if err != nil {
		return "", "", "", err
	}
	return entry.PluginState(wait, waitAt, failedReason != "", h.now(), h.pluginWindow()), failedReason, failedAction, nil
}

func writeRestoreRefused(c *gin.Context, reason string) {
	writeNamedConflict(c, reason)
}

func writeNamedConflict(c *gin.Context, reason string) {
	refusal, ok := entry.RestoreRefusalByReason(reason)
	if !ok {
		refusal, _ = entry.RestoreRefusalByReason("restore_refused")
	}
	c.JSON(http.StatusConflict, gin.H{"reason": refusal.Reason, "message": refusal.Text})
}

func writeCheckDenied(c *gin.Context, failures []string) {
	check, ok := entry.FirstCheck(failures)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "check failed"})
		return
	}
	c.JSON(http.StatusUnprocessableEntity, gin.H{"check_id": check.ID, "check_text": check.Text})
}

// writeClaimsDenied is the 422 for copy the offer does not support. check_id
// and check_text stay the catalog pair. reason, message, and error name the
// unsupported claim. error is the same sentence as message so a client that
// only reads error does not fall back to a generic failure.
func writeClaimsDenied(c *gin.Context, err error) {
	check, ok := entry.FirstCheck([]string{"claims_from_offer"})
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "check failed"})
		return
	}
	reason := entry.ClaimReasonNumberNotInOffer
	message := "The copy states a claim the offer does not support."
	var mismatch entry.ClaimMismatch
	if errors.As(err, &mismatch) && mismatch.Reason != "" {
		reason = mismatch.Reason
		message = mismatch.PublicMessage()
	}
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"check_id": check.ID, "check_text": check.Text,
		"reason": reason, "message": message, "error": message,
	})
}

func (h *AgentEntry) pluginWindow() time.Duration {
	if h.PluginOfflineAfter > 0 {
		return h.PluginOfflineAfter
	}
	return 10 * time.Minute
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
	if scope := strings.TrimSpace(c.GetString("offer_scope_id")); scope != "" && scope != placement.OfferID {
		c.JSON(http.StatusForbidden, gin.H{"error": "maker key is limited to its offer"})
		return "", "", false
	}
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
	raw := c.Param("source")
	counted := database.NormalizeAttributionSource(raw)
	if err := h.DB.CountSourceHit(c.Request.Context(), counted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redirect unavailable"})
		return
	}
	cookieValue := counted
	if tag := landingSourceTag(raw); tag != "" {
		cookieValue = tag
	}
	c.Header("Referrer-Policy", "strict-origin")
	http.SetCookie(c.Writer, &http.Cookie{Name: "or_source", Value: cookieValue, Path: "/", MaxAge: 60 * 60 * 24 * 30, SameSite: http.SameSiteLaxMode, Secure: c.Request.TLS != nil})
	c.Redirect(http.StatusFound, h.landingLocation())
}

func (h *AgentEntry) LLMs(c *gin.Context) {
	h.serveLandingFile(c, "llms.txt", "text/plain; charset=utf-8", llmsBody())
}

func (h *AgentEntry) LLMsFull(c *gin.Context) {
	h.serveLandingFile(c, "llms-full.txt", "text/plain; charset=utf-8", llmsFullDocument())
}

func (h *AgentEntry) Skill(c *gin.Context) {
	h.serveLandingFile(c, "skill.md", "text/markdown; charset=utf-8", skillMD)
}

func (h *AgentEntry) serveLandingFile(c *gin.Context, name, contentType, fallback string) {
	setPublicDocHeaders(c)
	body := []byte(fallback)
	if h.AssetsPath != "" {
		file, err := os.ReadFile(filepath.Join(h.AssetsPath, "landing-v9", name))
		if err == nil {
			body = file
		} else if !os.IsNotExist(err) {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	writeMethodBody(c, http.StatusOK, contentType, []byte(config.ExpandPublicBase(string(body), h.BaseURL)))
}

func (h *AgentEntry) Doc(c *gin.Context) {
	setPublicDocHeaders(c)
	body, ok := agentDoc(c.Param("name"))
	if !ok {
		raw, _ := json.Marshal(gin.H{"error": "document not found"})
		writeMethodBody(c, http.StatusNotFound, "application/json; charset=utf-8", raw)
		return
	}
	writeMethodBody(c, http.StatusOK, "text/markdown; charset=utf-8", []byte(config.ExpandPublicBase(body, h.BaseURL)))
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
			policy.SiteID = siteID
			if saved.BoxPosition != "" {
				policy.AllowedPositions = []string{saved.BoxPosition}
			}
			return policy, "setup", nil
		}
	}
	policy = defaultPolicy(entry.ModeManual)
	policy.SiteID = siteID
	return policy, "default", nil
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

func (h *AgentEntry) finishSetup(c *gin.Context, body gin.H, siteID string) {
	siteURL, err := h.siteURL(c, siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings unavailable"})
		return
	}
	body["change_url"] = h.changeURL()
	body["site_url"] = siteURL
	c.JSON(http.StatusOK, body)
}

func (h *AgentEntry) changeURL() string {
	base := strings.TrimRight(h.BaseURL, "/")
	if base == "" {
		return "/owner"
	}
	return base + "/owner"
}

func (h *AgentEntry) siteURL(c *gin.Context, siteID string) (string, error) {
	if siteID == "" {
		return "", nil
	}
	site, err := h.DB.GetSiteByID(c.Request.Context(), siteID)
	if err != nil {
		return "", err
	}
	return siteHome(site.Domain), nil
}

func (h *AgentEntry) attachDigestMeta(c *gin.Context, owner *database.Owner, body gin.H) error {
	pause, err := h.DB.DigestPause(c.Request.Context(), owner.ID, owner.SiteID, owner.MakerID)
	if err != nil {
		return err
	}
	since, err := h.DB.LastDigestOpen(c.Request.Context(), owner.ID)
	if err != nil {
		return err
	}
	floor := time.Unix(0, 0).UTC()
	if since != nil {
		floor = since.UTC()
	}
	wentSince, err := h.DB.CountWentLiveSince(c.Request.Context(), owner.SiteID, floor)
	if err != nil {
		return err
	}
	siteURL, err := h.siteURL(c, owner.SiteID)
	if err != nil {
		return err
	}
	body["paused"] = gin.H{"account": pause.Account, "site": pause.Site, "offer": pause.Offer}
	body["went_live_since_last"] = wentSince
	body["change_url"] = h.changeURL()
	body["site_url"] = siteURL
	body["available_modes"] = entry.AvailableModes()
	if !h.claims().Real() {
		body["approval_reason"] = entry.ClaimCheckReason
		body["approval_message"] = entry.ClaimCheckMessage
		body["effective_mode"] = entry.ModeManual
	}
	return nil
}

func normalizeSetupMoney(commission *setupCommission, price *setupPrice) (text, kind string, value *float64, currency string, priceValue *float64, priceCurrency, problem string) {
	if commission != nil {
		kind = commission.Type
		if kind != "percent" && kind != "fixed" {
			return "", "", nil, "", nil, "", "commission type must be percent or fixed"
		}
		if commission.Value == nil || *commission.Value < 0 {
			return "", "", nil, "", nil, "", "commission value is required"
		}
		if kind == "percent" && *commission.Value > 100 {
			return "", "", nil, "", nil, "", "commission percent must be from 0 to 100"
		}
		var ok bool
		currency, ok = normalizeCurrency(commission.Currency)
		if !ok {
			return "", "", nil, "", nil, "", "commission currency must be a 3-letter code"
		}
		if kind == "fixed" && currency == "" {
			return "", "", nil, "", nil, "", "currency is required when commission type is fixed"
		}
		amount := *commission.Value
		value = &amount
		if kind == "percent" {
			text = strconv.FormatFloat(amount, 'f', -1, 64) + "%"
		} else {
			text = strconv.FormatFloat(amount, 'f', -1, 64) + " " + currency
		}
	}
	if price != nil {
		if price.Value == nil || *price.Value < 0 {
			return "", "", nil, "", nil, "", "listing_price value is required"
		}
		var ok bool
		priceCurrency, ok = normalizeCurrency(price.Currency)
		if !ok || priceCurrency == "" {
			return "", "", nil, "", nil, "", "listing_price currency is required"
		}
		if currency != "" && priceCurrency != currency {
			return "", "", nil, "", nil, "", "listing_price currency must match commission"
		}
		amount := *price.Value
		priceValue = &amount
	}
	return text, kind, value, currency, priceValue, priceCurrency, ""
}

func normalizeCurrency(raw string) (string, bool) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		return "", true
	}
	if len(raw) != 3 {
		return "", false
	}
	for _, r := range raw {
		if r < 'A' || r > 'Z' {
			return "", false
		}
	}
	return raw, true
}

func validListingURL(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 2000 {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func (h *AgentEntry) yoloHeld(c *gin.Context, siteID string, saved bool, mode string) (bool, error) {
	if !saved || mode != entry.ModeYolo || siteID == "" || h.claims().Real() {
		return false, nil
	}
	has, err := h.DB.SiteHasApprovedPlacement(c.Request.Context(), siteID)
	if err != nil {
		return false, err
	}
	return !has, nil
}

func setupJSON(s database.SetupSettings, saved, human, signedIn bool, source string, claims entry.ClaimChecker, holdYolo bool) gin.H {
	mode := s.Mode
	preselected := entry.DefaultMode(source)
	// A saved yolo, including one stored only as mode_shown because an earlier
	// mode was kept, is what the next read preselects. The source default is
	// used only when yolo was not saved.
	if s.Mode == entry.ModeYolo || s.ModeShown == entry.ModeYolo {
		preselected = entry.ModeYolo
	}
	reason, message := "", ""
	if claims == nil || !claims.Real() {
		if mode == entry.ModeYolo || mode == entry.ModeAuto || s.ModeShown == entry.ModeYolo {
			if mode == entry.ModeYolo || mode == entry.ModeAuto {
				mode = entry.ModeManual
			}
			reason = entry.ClaimCheckReason
			message = entry.ClaimCheckMessage
			if holdYolo && s.Mode == entry.ModeYolo {
				reason = entry.YoloAfterFirstReason
				message = entry.YoloAfterFirstMessage
			}
		}
	}
	body := gin.H{
		"saved": saved, "role": s.Role, "install_path": s.InstallPath, "mode": mode, "mode_shown": s.ModeShown,
		"mode_preselected": preselected, "available_modes": entry.AvailableModes(),
		"promote": s.Promote, "avoid_topics": listOrEmpty(s.AvoidTopics), "commission": commissionJSON(s),
		"project_name": s.ProjectName, "listing_url": s.ListingURL, "listing_price": listingPriceJSON(s),
		"sites_channels": listOrEmpty(s.SitesChannels), "never_topics": listOrEmpty(s.NeverTopics), "box_position": s.BoxPosition,
		"human_login": human, "session": gin.H{"signed_in": signedIn},
	}
	if reason != "" {
		body["approval_reason"] = reason
		body["approval_message"] = message
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

func commissionJSON(s database.SetupSettings) any {
	if s.CommissionType == "" || s.CommissionValue == nil {
		return nil
	}
	return gin.H{"type": s.CommissionType, "value": *s.CommissionValue, "currency": s.CommissionCurrency}
}

func listingPriceJSON(s database.SetupSettings) any {
	if s.ListingPriceValue == nil {
		return nil
	}
	return gin.H{"value": *s.ListingPriceValue, "currency": s.ListingPriceCurrency}
}

func (h *AgentEntry) digestJSON(item database.DigestItem, token string) gin.H {
	pluginState := ""
	if item.Kind == "placement" {
		pluginState = entry.PluginState(item.PluginWait, item.PluginWaitAt, item.PluginFailedReason != "", h.now(), h.pluginWindow())
	}
	ruleID, ruleText := entry.PresentRule(item.MatchedRule)
	body := gin.H{
		"kind": item.Kind, "id": item.ID, "summary": item.Summary, "went_live_at": item.WentLiveAt.UTC().Format(time.RFC3339), "undo_token": token,
		"plugin_state": pluginState, "live_now": item.CountsAsLive,
		"post_title": filledDigestTitle(item), "surface_url": item.SurfaceURL, "position": filledDigestPosition(item.Position), "link_text": item.LinkText, "label": item.Label,
		"clicks": item.Clicks, "paid_clicks": item.PaidClicks, "approved_by": item.ApprovedBy, "undone_by": item.UndoneBy,
		"policy_rule_id": ruleID, "policy_rule_text": ruleText, "rules_version": item.PolicyVersion,
	}
	if item.Kind == "placement" {
		body["plugin_failed_reason"] = item.PluginFailedReason
		body["failed_action"] = item.FailedAction
		body["message"] = item.Summary
	}
	if item.Kind == "offer" {
		status := item.Status
		if status == "" {
			status = "listed"
		}
		body["status"] = status
		if item.ReviewReason == "" {
			body["review_reason"] = nil
			body["message"] = nil
		} else {
			body["review_reason"] = item.ReviewReason
			body["message"] = item.ReviewMessage
		}
		attachClaimsCheck(body, h.ClaimsCheck, status, item.ReviewReason)
	}
	return body
}

func (h *AgentEntry) annotateOfferReviews(c *gin.Context, items []database.DigestItem) error {
	listedByMaker := map[string]int{}
	for i := range items {
		if items[i].Kind != "offer" {
			continue
		}
		if items[i].Status == "" {
			items[i].Status = "listed"
		}
		if items[i].Status != "submitted" {
			items[i].ReviewReason = ""
			items[i].ReviewMessage = ""
			continue
		}
		if claimsHeld(h.ClaimsCheck) {
			items[i].ReviewReason = reviewReasonClaims
			items[i].ReviewMessage = reviewClaimsNotLive
			continue
		}
		offer, err := h.DB.GetOfferByID(c.Request.Context(), items[i].ID)
		if err != nil {
			return err
		}
		var makerID string
		if err = h.DB.Pool.QueryRow(c.Request.Context(), `SELECT p.maker_id::text FROM products p WHERE p.id=$1`, offer.ProductID).Scan(&makerID); err != nil {
			return err
		}
		product, err := h.DB.GetProduct(c.Request.Context(), makerID, offer.ProductID)
		if err != nil {
			return err
		}
		listed, ok := listedByMaker[makerID]
		if !ok {
			listed, err = h.DB.CountListedOffers(c.Request.Context(), makerID)
			if err != nil {
				return err
			}
			listedByMaker[makerID] = listed
		}
		items[i].ReviewReason, items[i].ReviewMessage = listingReview(c.Request.Context(), h.DB, h.ClaimsCheck, *offer, *product, listed)
	}
	return nil
}

func writePluginAckForbidden(c *gin.Context, reason, message, compat string) {
	c.JSON(http.StatusForbidden, gin.H{"error": compat, "reason": reason, "message": message})
}

func filledDigestTitle(item database.DigestItem) string {
	return strings.TrimSpace(item.PostTitle)
}

func (h *AgentEntry) MakerDigest(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing maker key"})
		return
	}
	middleware.MakerKeyAuth(h.DB)(c)
	if c.IsAborted() {
		return
	}
	makerID := c.GetString("maker_id")
	if makerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing maker key"})
		return
	}
	offerID := strings.TrimSpace(c.Query("offer_id"))
	scoped := c.GetString("offer_scope_id")
	if offerID != "" {
		if !uuidPattern.MatchString(offerID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
			return
		}
		if scoped != "" && scoped != offerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "key is scoped to a different offer", "reason": "forbidden", "message": "This key can read only its own offers."})
			return
		}
		_, ownerMaker, _, err := h.DB.OfferFacts(c.Request.Context(), offerID)
		if err != nil || ownerMaker != makerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "offer is not on this account", "reason": "forbidden", "message": "This key can read only its own offers."})
			return
		}
	} else if scoped != "" {
		offerID = scoped
	}
	total, err := h.DB.CountMakerDigestOffers(c.Request.Context(), makerID, offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	offers, err := h.DB.MakerDigestOffers(c.Request.Context(), makerID, offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	if err = h.annotateOfferReviews(c, offers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	ownerID := ""
	if id, err := h.DB.OwnerIDByMaker(c.Request.Context(), makerID); err == nil {
		ownerID = id
	} else if !errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	items := make([]gin.H, 0, len(offers))
	for _, item := range offers {
		token := ""
		if ownerID != "" {
			issued, err := h.issueUndo(c, ownerID, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
				return
			}
			token = issued
		}
		items = append(items, h.digestJSON(item, token))
	}
	conversions, err := h.loadConversions(c, makerID, "", offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest unavailable"})
		return
	}
	body := gin.H{
		"items": items, "session": gin.H{"signed_in": false},
		"truncated": total > len(items), "total": total,
		"conversions": conversions,
	}
	if claimsHeld(h.ClaimsCheck) {
		body["claims_check"] = "not_checked"
	}
	c.JSON(http.StatusOK, body)
}

func (h *AgentEntry) loadConversions(c *gin.Context, makerID, siteID, offerID string) ([]gin.H, error) {
	if h.DB == nil {
		return []gin.H{}, nil
	}
	rows, err := h.DB.ListConversionReports(c.Request.Context(), makerID, siteID, offerID, 50)
	if err != nil {
		return nil, err
	}
	return conversionsJSON(rows), nil
}

func (h *AgentEntry) scopedConversions(c *gin.Context, makerID, siteID, offerID string) ([]gin.H, error) {
	out := []gin.H{}
	seen := map[string]bool{}
	add := func(rows []gin.H) {
		for _, row := range rows {
			id, _ := row["id"].(string)
			if id != "" && seen[id] {
				continue
			}
			if id != "" {
				seen[id] = true
			}
			out = append(out, row)
		}
	}
	if siteID != "" {
		rows, err := h.loadConversions(c, "", siteID, "")
		if err != nil {
			return nil, err
		}
		add(rows)
	}
	if makerID != "" {
		rows, err := h.loadConversions(c, makerID, "", offerID)
		if err != nil {
			return nil, err
		}
		add(rows)
	}
	return out, nil
}

func filledDigestPosition(position string) string {
	if entry.ValidPosition(position) {
		return position
	}
	return "end"
}

func (h *AgentEntry) writePolicy(c *gin.Context, policy database.ApprovalPolicy, source string) {
	body := policyJSON(policy, source)
	mode, reason, message := h.presentMode(policy.Mode)
	if policy.Mode == entry.ModeYolo {
		body["mode_preselected"] = entry.ModeYolo
		if policy.SiteID != "" && !h.claims().Real() {
			has, err := h.DB.SiteHasApprovedPlacement(c.Request.Context(), policy.SiteID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "mode unavailable"})
				return
			}
			if !has {
				mode = entry.ModeManual
				reason = entry.YoloAfterFirstReason
				message = entry.YoloAfterFirstMessage
			}
		}
	}
	body["mode"] = mode
	if reason != "" {
		body["approval_reason"] = reason
		body["approval_message"] = message
	}
	c.JSON(http.StatusOK, body)
}

func sessionOnlyDecision(c *gin.Context) bool {
	if credentialPresented(c) || c.GetBool("agent_access") || c.GetString("key_kind") == "project" || c.GetString("key_kind") == "account" {
		c.JSON(http.StatusForbidden, gin.H{"error": "decide and restore require a signed-in dashboard session"})
		return true
	}
	return false
}

func (h *AgentEntry) presentMode(mode string) (effective, reason, message string) {
	if h.claims().Real() || (mode != entry.ModeYolo && mode != entry.ModeAuto) {
		return mode, "", ""
	}
	return entry.ModeManual, entry.ClaimCheckReason, entry.ClaimCheckMessage
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
