package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// AgentJoinCommand is the one line a stranger's agent runs from a clean folder.
// OFFER_RAILS_URL is the API origin, with no trailing slash.
const AgentJoinCommand = `curl -fsS -X POST "$OFFER_RAILS_URL/v1/agent-tokens" -H 'Content-Type: application/json' -d '{}'`

// CreateAgentToken mints a role-less agent account and a bearer token.
// The body is {} or empty. A role field is refused. The plaintext token is
// returned once and stored only as a hash.
func (h *AgentEntry) CreateAgentToken(c *gin.Context) {
	ip := joinClientIP(c)
	if h.TokenLimiter != nil && !h.TokenLimiter.Allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	body, err := readJSONBody(c, 1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	name, ok := joinTokenName(c, body)
	if !ok {
		return
	}
	raw, err := newAgentJoinSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	subject, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	emailNonce := make([]byte, 8)
	if _, err = rand.Read(emailNonce); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	issued, err := h.DB.CreateAgentJoinToken(c.Request.Context(), services.HashAPIKey(raw), services.KeyPrefix(raw), services.HashAPIKey(ip), h.mcpResource(c), name, "join:"+subject, "join-"+hex.EncodeToString(emailNonce)+"@agents.invalid")
	if errors.Is(err, database.ErrJoinLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"access_token": raw,
		"token_type":   "Bearer",
		"scope":        issued.Scope,
		"expires_in":   joinExpiresIn(issued.ExpiresAt),
		"token_id":     issued.TokenID,
		"owner_id":     issued.OwnerID,
		"agent_only":   true,
		"human_login":  false,
		"roles":        []string{},
	})
}

// ListAgentTokens returns this token's owner, the inferred roles, and the
// token ids. It does not return a secret.
func (h *AgentEntry) ListAgentTokens(c *gin.Context) {
	grant, ok := h.liveJoinGrant(c)
	if !ok {
		return
	}
	rows, err := h.DB.ListAgentJoinTokens(c.Request.Context(), grant.OwnerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tokens unavailable"})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":         row.ID,
			"prefix":     row.Prefix,
			"scope":      row.Scope,
			"expires_at": row.ExpiresAt.UTC().Format(time.RFC3339),
			"revoked":    row.Revoked,
			"created_at": row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"owner_id":    grant.OwnerID,
		"agent_only":  true,
		"human_login": false,
		"roles":       joinRoles(grant.SiteID, grant.MakerID),
		"tokens":      items,
	})
}

// RevokeAgentToken revokes the bearer that called it.
func (h *AgentEntry) RevokeAgentToken(c *gin.Context) {
	raw, ok := presentedBearer(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return
	}
	id, err := h.DB.RevokeAgentJoinToken(c.Request.Context(), services.HashAPIKey(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true, "token_id": id})
}

// RevokeAgentTokenByID revokes one token on the same agent account.
func (h *AgentEntry) RevokeAgentTokenByID(c *gin.Context) {
	grant, ok := h.liveJoinGrant(c)
	if !ok {
		return
	}
	if !uuidPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	revoked, err := h.DB.RevokeAgentJoinTokenID(c.Request.Context(), grant.OwnerID, c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) || !revoked {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true, "token_id": c.Param("id")})
}

func (h *AgentEntry) liveJoinGrant(c *gin.Context) (*database.AgentGrant, bool) {
	raw, ok := presentedBearer(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return nil, false
	}
	grant, err := h.DB.AgentJoinToken(c.Request.Context(), services.HashAPIKey(raw), h.mcpResource(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
		return nil, false
	}
	return grant, true
}

func (h *AgentEntry) mcpResource(c *gin.Context) string {
	base := strings.TrimRight(h.BaseURL, "/")
	if base == "" && c != nil && c.Request != nil && c.Request.Host != "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	if base == "" {
		return "/mcp"
	}
	return base + "/mcp"
}

func joinTokenName(c *gin.Context, body []byte) (string, bool) {
	trimmed := bytesTrim(body)
	if len(trimmed) == 0 {
		return "Agent", true
	}
	var raw map[string]json.RawMessage
	if decodeStrict(trimmed, &raw) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	if _, present := raw["role"]; present {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is not chosen here"})
		return "", false
	}
	for key := range raw {
		if key != "name" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return "", false
		}
	}
	var req struct {
		Name string `json:"name"`
	}
	if decodeStrict(trimmed, &req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "Agent", true
	}
	if utf8.RuneCountInString(req.Name) > 80 || strings.ContainsAny(req.Name, "\r\n") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return "", false
	}
	return req.Name, true
}

func bytesTrim(body []byte) []byte {
	return []byte(strings.TrimSpace(string(body)))
}

func presentedBearer(c *gin.Context) (string, bool) {
	fields := strings.Fields(c.GetHeader("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", false
	}
	return fields[1], true
}

func newAgentJoinSecret() (string, error) {
	raw, err := randomToken()
	if err != nil {
		return "", err
	}
	return "orat_" + raw, nil
}

func joinExpiresIn(expires time.Time) int {
	secs := int(time.Until(expires).Seconds())
	if secs < 0 {
		return 0
	}
	return secs
}

func joinRoles(siteID, makerID string) []string {
	roles := []string{}
	if siteID != "" {
		roles = append(roles, "publisher")
	}
	if makerID != "" {
		roles = append(roles, "maker")
	}
	return roles
}

func joinClientIP(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "unknown"
	}
	return ip
}
