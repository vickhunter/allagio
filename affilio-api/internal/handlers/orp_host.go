package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// ORPHost is the hosted ORP thin layer. It speaks 0.3 and, for many-party
// deals, 0.4. It serves discovery, the protocol text, the schemas, a
// credential-free suggest (with optional join), and a test-mode offer,
// accept, activate, and proof. A 0.3 deal stays awaiting until a party calls
// POST /orp/deals/:id/activate. A 0.4 deal becomes active when every required
// party has signed. The host does not hold money. The discovery name is
// "In testing". This layer claims no conformance level. DCP v0.1 stays the
// live affiliate path. CLAIMS_CHECK is only reported.
type ORPHost struct {
	Entry          *AgentEntry
	SuggestLimiter *IPRateLimiter
	MessageLimiter *IPRateLimiter
	Now            func() time.Time
}

// Mount registers the ORP routes. POST /orp is the message endpoint.
func (h *ORPHost) Mount(r gin.IRouter) {
	r.GET("/.well-known/orp.json", h.Discovery)
	r.POST("/orp", h.Envelope)
	r.POST("/orp/suggest", h.Suggest)
	r.POST(MonetizationSuggestPath, h.Monetize)
	r.POST("/orp/join", h.Join)
	r.GET("/orp/keys/:opaque", h.PartyKeysEndpoint)
	r.POST("/v1/party-claims", h.PartyClaim)
	r.GET("/claim/:code", h.ClaimPage)
	r.POST("/claim/:code", h.ClaimPost)
	r.GET("/orp/spec", h.Spec)
	r.GET("/orp/gaps", h.Gaps)
	r.GET("/orp/schemas", h.SchemaIndex)
	r.GET("/orp/schemas/*name", h.Schema)
	r.GET("/orp/examples", h.ExampleIndex)
	r.GET("/orp/examples/*name", h.Example)
	mountORPDeals(h, r)
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
		joined, problem, ok = h.mint(c, origin, dns, req.Join.AgentName, req.Join.Key)
		if !ok {
			h.writeProblem(c, dns, "", "", problem, true)
			return
		}
	}
	items, empty := orp.Match(dns, req, rows)
	fresh, err := h.freshPitches(c, req.Text)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), true)
		return
	}
	if len(fresh) > 0 {
		items = append(fresh, items...)
		limit := req.Limit
		if limit <= 0 {
			limit = 3
		}
		if len(items) > limit {
			items = items[:limit]
		}
		empty = ""
	}
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
	joined, problem, ok := h.mint(c, origin, dns, req.AgentName, req.Key)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, true)
		return
	}
	writeORP(c, http.StatusCreated, joined, true)
}

// PartyKeysEndpoint is GET /orp/keys/<opaque>. It serves the JWK set a party
// registered at join. A party with no keys is a 404.
func (h *ORPHost) PartyKeysEndpoint(c *gin.Context) {
	opaque := c.Param("opaque")
	if h.Entry == nil || h.Entry.DB == nil || opaque == "" {
		writeORP(c, http.StatusNotFound, map[string]any{"error": "not_found"}, false)
		return
	}
	keys, err := h.Entry.DB.PartyKeys(c.Request.Context(), opaque)
	if err != nil || len(keys) == 0 {
		writeORP(c, http.StatusNotFound, map[string]any{"error": "not_found"}, false)
		return
	}
	writeORP(c, http.StatusOK, map[string]any{"keys": keys}, false)
}

// Envelope is POST /orp. A #68-shaped body is invalid_message before any
// credential check and before any write. A standing accept opens a test deal
// in awaiting. A directed offer opens proposed, and the counterparty accept
// stores awaiting. Activate is the only step from awaiting to active.
// Execute on a deal that is not active is state_conflict.
// This host does not hold money.
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
		if sniffedORP(value) == orp.Version04 {
			if problem.Status == 0 {
				problem.Status = http.StatusBadRequest
			}
			doc := orp.ErrorDocument(dns, "", "", problem, h.now())
			doc["orp"] = orp.Version04
			writeORP(c, problem.Status, doc, false)
			return
		}
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
	hash := orp.ContentHash(value)
	if h.seen(c, dns, msg, hash) {
		return
	}
	if msg.HasSig || orp.SignedType(msg.Type) {
		keys, err := h.Entry.DB.PartyKeys(c.Request.Context(), grant.OwnerID)
		if err != nil {
			h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
			return
		}
		if msg.HasSig {
			if len(keys) == 0 {
				h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "bad_signature", Message: "This party has no registered key. Join again with key to send signed envelopes."}, false)
				return
			}
			if _, ok := orp.VerifyEnvelopeSig(value, keys); !ok {
				h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "bad_signature", Message: "sig does not verify against the party key."}, false)
				return
			}
		} else if len(keys) > 0 {
			h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "This party registered a key, so " + msg.Type + " must carry sig."}, false)
			return
		}
	}
	if problem, stale := orp.ClockProblem(msg.TS, h.now()); stale {
		h.writeProblem(c, dns, msg.From, msg.ID, problem, false)
		return
	}
	if h.MessageLimiter != nil && !h.MessageLimiter.Allow(grant.OwnerID) {
		h.writeProblem(c, dns, msg.From, msg.ID, rateLimited("Too many messages from this party."), false)
		return
	}
	switch {
	case msg.Type == "hello":
		h.finish(c, msg, hash, http.StatusOK, orp.HelloReplyFor(msg.ORP, dns, msg.From, msg.ID, party, h.claimsMode(), h.now()), database.ORPEffect{})
	case msg.Type == "discover":
		h.discover(c, dns, msg, hash)
	case msg.Type == "space":
		h.space(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "request":
		h.request(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "offer":
		h.offer(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "accept":
		h.accept(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "counter":
		h.counter(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "execute":
		h.execute(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "leave":
		h.leave04(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "seat":
		h.seat04(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "decline":
		h.decline(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "revoke":
		h.revoke(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "dispute":
		h.dispute(c, dns, msg, hash, grant.OwnerID)
	case msg.Type == "reputation":
		h.reputation(c, dns, msg, hash)
	case msg.Type == "receipt":
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the host issues a receipt."}), database.ORPEffect{})
	case orp.IsHostOnly(msg.Type):
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.HostOnly()), database.ORPEffect{})
	default:
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.RefusesDeal()), database.ORPEffect{})
	}
}

func (h *ORPHost) discover(c *gin.Context, dns string, msg orp.Message, hash string) {
	want, _ := msg.Body["want"].(string)
	if want == "spaces" {
		h.discoverSpaces(c, dns, msg, hash)
		return
	}
	rows, err := h.candidates(c)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	text, limit := orp.DiscoverHaystack(msg.Body)
	standing, err := h.standingItems(c, text)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	catalog, empty := orp.Match(dns, orp.SuggestRequest{Text: text, Haystack: text, Limit: limit}, rows)
	items := append(standing, catalog...)
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == 0 && empty == "" {
		empty = "no_match"
	}
	if len(items) > 0 {
		empty = ""
	}
	doc := orp.ResultsReply(dns, msg.From, msg.ID, items, empty, h.now())
	if msg.ORP == orp.Version04 {
		doc["orp"] = orp.Version04
	}
	h.finish(c, msg, hash, http.StatusOK, doc, database.ORPEffect{})
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

func (h *ORPHost) mint(c *gin.Context, origin, dns, agentName string, partyKey map[string]any) (map[string]any, orp.Problem, bool) {
	name := agentName
	if name == "" {
		var err error
		name, err = newAgentHandle()
		if err != nil {
			return nil, internalProblem(), false
		}
	}
	raw, issued, status, _ := h.Entry.MintJoin(c, name, partyKey)
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
	doc := orp.ErrorDocument(dns, to, re, problem, h.now())
	if strings.Contains(problem.Message, "ORP 0.4.") {
		doc["orp"] = orp.Version04
	}
	writeORP(c, problem.Status, doc, cors)
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

func sniffedORP(value any) string {
	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	version, _ := obj["orp"].(string)
	return version
}

func obligorBy(deal database.ORPDeal, by string) string {
	if strings.HasPrefix(by, "orp:") {
		return by
	}
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		return ""
	}
	for _, seat := range doc.Seats {
		if seat.Role == by {
			return seat.ID
		}
	}
	return ""
}

func rateLimited(message string) orp.Problem {
	return orp.Problem{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: message, RetryAfter: 60}
}

func internalProblem() orp.Problem {
	return orp.Problem{Status: http.StatusInternalServerError, Code: "internal", Message: "The host could not complete this call.", Retryable: true}
}
