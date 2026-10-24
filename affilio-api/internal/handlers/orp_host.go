package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// ORPHost is the hosted ORP 0.3 thin layer. It serves discovery, a credential-free
// suggest (with optional join), and an envelope that does not open deals.
// The discovery name is "In testing". This layer claims no conformance level.
// DCP v0.1 stays the live affiliate path. CLAIMS_CHECK is only reported.
type ORPHost struct {
	Entry          *AgentEntry
	SuggestLimiter *IPRateLimiter
	Now            func() time.Time
}

// Mount registers the ORP routes. POST /orp is the discovery endpoint.
func (h *ORPHost) Mount(r gin.IRouter) {
	r.GET("/.well-known/orp.json", h.Discovery)
	r.POST("/orp", h.Envelope)
	r.POST("/orp/suggest", h.Suggest)
	r.POST(MonetizationSuggestPath, h.Monetize)
	r.POST("/orp/join", h.Join)
}

func (h *ORPHost) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *ORPHost) origin(c *gin.Context) (origin, dns string) {
	scheme := "http"
	if c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	}
	host := ""
	base := ""
	if c.Request != nil {
		host = c.Request.Host
	}
	if h.Entry != nil {
		base = h.Entry.BaseURL
	}
	return orp.Origin(base, scheme, host)
}

func (h *ORPHost) claimsMode() string {
	if h.Entry == nil {
		return ""
	}
	return h.Entry.ClaimsCheck
}

// Discovery is GET /.well-known/orp.json. It requires no credential.
func (h *ORPHost) Discovery(c *gin.Context) {
	origin, dns := h.origin(c)
	writeORP(c, http.StatusOK, orp.DiscoveryDocument(origin, dns, h.claimsMode()), true)
}

// Suggest is POST /orp/suggest. One call returns pitches and, when join is
// present, the same bearer POST /v1/agent-tokens would mint. The draft is
// not stored.
func (h *ORPHost) Suggest(c *gin.Context) {
	origin, dns := h.origin(c)
	body, problem, ok := readORPBody(c)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	if h.SuggestLimiter != nil && !h.SuggestLimiter.Allow(h.clientIP(c)) {
		h.writeProblem(c, dns, "", "", rateLimited("Too many suggest calls from this address."), true)
		return
	}
	value, problem, ok := orp.DecodeJSON(body)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	req, problem, ok := orp.ParseSuggest(value)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	rows, err := h.candidates(c)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), true)
		return
	}
	var joined map[string]any
	if req.HasJoin {
		joined, problem, ok = h.mint(c, origin, dns, req.Join.AgentName)
		if !ok {
			h.writeProblem(c, dns, "", "", problem, true)
			return
		}
	}
	items, empty := orp.Match(dns, req, rows)
	writeORP(c, http.StatusOK, orp.SuggestResponse(origin, items, empty, joined), true)
}

// Join is POST /orp/join. An empty object is a complete request.
func (h *ORPHost) Join(c *gin.Context) {
	origin, dns := h.origin(c)
	body, problem, ok := readORPBody(c)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	value, problem, ok := orp.DecodeJSON(body)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	req, problem, ok := orp.ParseJoin(value, "")
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	joined, problem, ok := h.mint(c, origin, dns, req.AgentName)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	writeORP(c, http.StatusCreated, joined, true)
}

// Envelope is POST /orp. A #68-shaped body is invalid_message before any
// credential check and before any write. This testing host does not open deals.
func (h *ORPHost) Envelope(c *gin.Context) {
	_, dns := h.origin(c)
	body, problem, ok := readORPBody(c)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, false)
		return
	}
	value, problem, ok := orp.DecodeJSON(body)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, false)
		return
	}
	msg, problem, ok := orp.ParseEnvelope(value)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, false)
		return
	}
	grant, err := h.Entry.JoinGrant(c)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "No credential or a bad credential."}, false)
		return
	}
	party := orp.MemberParty(dns, grant.OwnerID)
	if msg.From != party {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "from does not match the credential."}, false)
		return
	}
	if msg.HasSig {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "bad_signature", Message: "This testing host publishes no keys and cannot verify sig."}, false)
		return
	}
	if problem, stale := orp.ClockProblem(msg.TS, h.now()); stale {
		h.writeProblem(c, dns, msg.From, msg.ID, problem, false)
		return
	}
	switch {
	case msg.Type == "hello":
		writeORP(c, http.StatusOK, orp.HelloReply(dns, msg.From, msg.ID, party, h.claimsMode(), h.now()), false)
	case msg.Type == "discover":
		h.discover(c, dns, msg)
	case orp.IsHostOnly(msg.Type):
		problem := orp.HostOnly()
		h.writeProblem(c, dns, msg.From, msg.ID, problem, false)
	default:
		h.writeProblem(c, dns, msg.From, msg.ID, orp.RefusesDeal(), false)
	}
}

func (h *ORPHost) discover(c *gin.Context, dns string, msg orp.Message) {
	want, _ := msg.Body["want"].(string)
	if want == "spaces" {
		writeORP(c, http.StatusOK, orp.ResultsReply(dns, msg.From, msg.ID, []any{}, "no_match", h.now()), false)
		return
	}
	rows, err := h.candidates(c)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	text, limit := orp.DiscoverHaystack(msg.Body)
	items, empty := orp.Match(dns, orp.SuggestRequest{Text: text, Haystack: text, Limit: limit}, rows)
	writeORP(c, http.StatusOK, orp.ResultsReply(dns, msg.From, msg.ID, items, empty, h.now()), false)
}

func (h *ORPHost) candidates(c *gin.Context) ([]orp.Candidate, error) {
	if h.Entry == nil || h.Entry.DB == nil {
		return nil, errors.New("no database")
	}
	rows, err := h.Entry.DB.ListSuggestCandidates(c.Request.Context(), 200)
	if err != nil {
		return nil, err
	}
	out := make([]orp.Candidate, len(rows))
	for i, row := range rows {
		out[i] = orp.Candidate{
			ID:              row.ID,
			Source:          row.Source,
			Name:            row.Name,
			Company:         row.Company,
			Blurb:           row.Blurb,
			Website:         row.Website,
			Keywords:        row.Keywords,
			Category:        row.Category,
			CommissionUnit:  row.CommissionUnit,
			CommissionType:  row.CommissionType,
			CommissionValue: row.CommissionValue,
			Currency:        row.Currency,
			CookieDays:      row.CookieDays,
		}
	}
	return out, nil
}

func (h *ORPHost) mint(c *gin.Context, origin, dns, agentName string) (map[string]any, orp.Problem, bool) {
	name := agentName
	if name == "" {
		var err error
		name, err = newAgentHandle()
		if err != nil {
			return nil, internalProblem(), false
		}
	}
	raw, issued, status, _ := h.Entry.MintJoin(c, name)
	if status == http.StatusTooManyRequests {
		return nil, rateLimited("Too many joins from this address."), false
	}
	if status == http.StatusForbidden {
		return nil, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "loopback host is not accepted from this client"}, false
	}
	if status != 0 {
		return nil, internalProblem(), false
	}
	return orp.JoinResponse(origin, dns, issued.OwnerID, raw), orp.Problem{}, true
}

func (h *ORPHost) clientIP(c *gin.Context) string {
	if h.Entry == nil {
		return ""
	}
	return h.Entry.joinClientIP(c)
}

func (h *ORPHost) writeProblem(c *gin.Context, dns, to, re string, problem orp.Problem, cors bool) {
	if problem.Status == 0 {
		problem.Status = http.StatusBadRequest
	}
	if problem.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(problem.RetryAfter))
	}
	writeORP(c, problem.Status, orp.ErrorDocument(dns, to, re, problem, h.now()), cors)
}

func readORPBody(c *gin.Context) ([]byte, orp.Problem, bool) {
	if c.Request.Body == nil {
		return []byte("{}"), orp.Problem{}, true
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, orp.MaxBodyBytes+1))
	if err != nil {
		return nil, orp.Invalid("The body is not JSON.", ""), false
	}
	if len(body) > orp.MaxBodyBytes {
		return nil, orp.Invalid("The body is over 64 KiB.", ""), false
	}
	return body, orp.Problem{}, true
}

func writeORP(c *gin.Context, status int, doc any, cors bool) {
	c.Header("Cache-Control", "no-store")
	if cors {
		c.Header("Access-Control-Allow-Origin", "*")
	}
	c.JSON(status, doc)
}

func rateLimited(message string) orp.Problem {
	return orp.Problem{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: message, RetryAfter: 60}
}

func internalProblem() orp.Problem {
	return orp.Problem{Status: http.StatusInternalServerError, Code: "internal", Message: "The host could not complete this call.", Retryable: true}
}
