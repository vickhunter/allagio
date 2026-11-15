package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DealCertificate is GET /orp/deals/:id/certificate — the host-signed proof a
// party can hold: deal id, terms_hash, both party ids, state, and the receipt
// list, signed with the host's Ed25519 key (the JWK the discovery document
// publishes under keys). Terms themselves stay off the response; the hash
// binds them, so whoever holds the terms proves the agreement by recomputing
// the hash. Receipt bodies are verification artifacts already emitted to the
// parties, so they are included verbatim.
func (h *ORPHost) DealCertificate(c *gin.Context) {
	if h.Signer == nil {
		writeORP(c, http.StatusServiceUnavailable, map[string]any{"error": "signing_not_configured"}, false)
		return
	}
	id := c.Param("id")
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), id)
	if err != nil {
		writeORP(c, http.StatusInternalServerError, map[string]any{"error": "read_failed"}, false)
		return
	}
	if !found {
		writeORP(c, http.StatusNotFound, map[string]any{"error": "not_found"}, false)
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now()
	}
	payload := map[string]any{
		"orp":        "0.4",
		"type":       "certificate",
		"deal":       deal.ID,
		"terms_hash": deal.TermsHash,
		"state":      deal.State,
		"offerer":    deal.OffererParty,
		"acceptor":   deal.AcceptorParty,
		"created_at": deal.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": deal.UpdatedAt.UTC().Format(time.RFC3339),
		"receipts":   dealReceipts(deal.Tracking, deal.Parties),
		"issued_at":  now.UTC().Format(time.RFC3339),
	}
	writeORP(c, http.StatusOK, h.Signer.Sign(payload), true)
}

// dealReceipts pulls the link_receipts list out of the tracking document
// (0.3) or the parties document (0.4). Empty slice when the deal has none.
func dealReceipts(docs ...json.RawMessage) []any {
	for _, raw := range docs {
		if len(raw) == 0 {
			continue
		}
		var doc map[string]any
		if json.Unmarshal(raw, &doc) != nil {
			continue
		}
		if receipts, ok := doc["link_receipts"].([]any); ok && len(receipts) > 0 {
			return receipts
		}
	}
	return []any{}
}
