package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"
)

// fedFetch is the federation fetch seam. Tests stub it; production uses the
// SSRF-guarded readiness client.
type fedFetch func(ctx context.Context, method, rawURL string, payload []byte) (body string, status int, ok bool)

func defaultFedFetch(ctx context.Context, method, rawURL string, payload []byte) (string, int, bool) {
	e := &services.ReadinessEngine{}
	if method == http.MethodPost {
		return e.PostPublicJSON(ctx, rawURL, payload)
	}
	return e.FetchPublicPage(ctx, rawURL)
}

func (h *ORPHost) fedFetch() fedFetch {
	if h.FederationFetch != nil {
		return h.FederationFetch
	}
	return defaultFedFetch
}

// foreignGrant authenticates an envelope whose from is orp:<otherhost>/<x>.
// There is no bearer: the signature over the envelope is the credential, and
// the party's public keys are fetched from its home host's keys endpoint.
// A verified party gets a synthetic owner row so deals and spaces can point
// at it. handled is false when from is not a foreign party at all; a nil
// grant with handled true means the reply is already written.
func (h *ORPHost) foreignGrant(c *gin.Context, dns string, msg orp.Message, value any) (grant *database.AgentGrant, handled bool) {
	host, opaque, ok := orp.SplitParty(msg.From)
	if !ok || host == "" || host == dns {
		return nil, false
	}
	if !msg.HasSig {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "A foreign party must sign."}, false)
		return nil, true
	}
	raw, _, fetched := h.fedFetch()(c.Request.Context(), http.MethodGet, "https://"+host+"/orp/keys/"+opaque, nil)
	if !fetched {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The party's home host did not answer with keys."}, false)
		return nil, true
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The party's home host answered with no key set."}, false)
		return nil, true
	}
	var keys []map[string]any
	items, _ := doc["keys"].([]any)
	for _, item := range items {
		if key, isMap := item.(map[string]any); isMap {
			keys = append(keys, key)
		}
	}
	if _, verified := orp.VerifyEnvelopeSig(value, keys); !verified {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "bad_signature", Message: "sig does not verify against the party's home keys."}, false)
		return nil, true
	}
	ownerID, err := h.Entry.DB.EnsureFederatedOwner(c.Request.Context(), msg.From, host)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return nil, true
	}
	return &database.AgentGrant{OwnerID: ownerID, Join: true}, true
}

// relayForeign forwards an envelope addressed to orp:<otherhost>/<x> to that
// host's ORP endpoint. The remote verifies the sender's signature, so only
// signed envelopes relay. The reply is relayed verbatim; the remote host owns
// the stored record.
func (h *ORPHost) relayForeign(c *gin.Context, dns string, msg orp.Message, toHost string, rawBody []byte) {
	if !msg.HasSig {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "A foreign party needs a signed envelope. Join again with key."}, false)
		return
	}
	raw, _, fetched := h.fedFetch()(c.Request.Context(), http.MethodGet, "https://"+toHost+"/.well-known/orp.json", nil)
	if !fetched {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The other host did not answer with a discovery document."}, false)
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The other host answered with no discovery document."}, false)
		return
	}
	endpoint, _ := doc["endpoint"].(string)
	if !strings.HasPrefix(endpoint, "https://"+toHost+"/") {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The other host's endpoint is not on that host."}, false)
		return
	}
	resp, status, ok := h.fedFetch()(c.Request.Context(), http.MethodPost, endpoint, rawBody)
	if !ok && status == 0 {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusBadGateway, Code: "unreachable_host", Message: "The other host did not answer."}, false)
		return
	}
	writeORPBytes(c, status, []byte(resp))
}
