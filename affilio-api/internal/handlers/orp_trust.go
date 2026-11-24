package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

func (h *ORPHost) decline(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, party, ok := h.partyDeal(c, dns, msg, hash, ownerID)
	if !ok {
		return
	}
	if deal.State != "proposed" && deal.State != "awaiting" {
		h.stateConflict(c, dns, msg, hash)
		return
	}
	now := h.now()
	deal.State = "declined"
	deal.Waiting = nil
	deal.UpdatedAt = now.UTC()
	h.finishDeal(c, dns, msg, hash, party, deal, now, database.ORPEffect{
		UpdateDealID: deal.ID, UpdateState: "declined",
		UpdateFrom: []string{"proposed", "awaiting"}, ClearWaiting: true, UpdatedAt: deal.UpdatedAt,
	})
}

func (h *ORPHost) revoke(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	scope, _ := msg.Body["scope"].(string)
	if scope == "offer" {
		h.revokeOffer(c, dns, msg, hash, ownerID)
		return
	}
	deal, party, ok := h.partyDeal(c, dns, msg, hash, ownerID)
	if !ok {
		return
	}
	mode, err := h.modeFor(c, ownerID, database.StepRevoke)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("revoke", "The owner has turned off revoke, so this agent cannot revoke.")), database.ORPEffect{})
		return
	}
	now := h.now()
	hold := func(waiting map[string]any) {
		h.holdDeal(c, dns, msg, hash, party, deal, waiting, []string{"proposed", "awaiting", "active"}, now)
	}
	switch scope {
	case "obligation":
		// SPEC §12: obligations run on an active deal. §15.1: verified work stays verified.
		// Only that obligation's obligor may stop it.
		if deal.State != "active" {
			h.stateConflict(c, dns, msg, hash)
			return
		}
		obligationID, _ := msg.Body["obligation"].(string)
		state, found, dup := obligationSnapshot(deal.Obligations, obligationID)
		if dup {
			h.stateConflict(c, dns, msg, hash)
			return
		}
		if !found {
			h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such obligation for this sender."}), database.ORPEffect{})
			return
		}
		if !revocableObligation(state) {
			message := "The obligation is in the wrong state."
			if state == "verified" {
				message = "Work already verified stays verified."
			}
			h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: message}), database.ORPEffect{})
			return
		}
		obligor, ok := database.ObligationObligor(deal, obligationID)
		if !ok || obligor != party {
			h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the obligor may revoke this obligation."}), database.ORPEffect{})
			return
		}
		if mode == database.ModeAsk {
			origin, _ := h.origin(c)
			waiting := waitingOn(party, "revoke", []any{"step_ask"}, origin, now)
			waiting["scope"] = "obligation"
			waiting["obligation"] = obligationID
			hold(waiting)
			return
		}
		rows, err := markObligation(deal.Obligations, obligationID, "revoked")
		if err != nil || rows == nil {
			h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
			return
		}
		prior := deal.UpdatedAt
		deal.Obligations = rows
		deal.UpdatedAt = now.UTC()
		h.finishDeal(c, dns, msg, hash, party, deal, now, database.ORPEffect{
			UpdateDealID: deal.ID, RevokeObligationID: obligationID, Sender: party,
			ExpectUpdatedAt: prior, UpdatedAt: deal.UpdatedAt,
		})
	default:
		if deal.State != "proposed" && deal.State != "awaiting" && deal.State != "active" {
			h.stateConflict(c, dns, msg, hash)
			return
		}
		if mode == database.ModeAsk {
			origin, _ := h.origin(c)
			waiting := waitingOn(party, "revoke", []any{"step_ask"}, origin, now)
			waiting["scope"] = "deal"
			hold(waiting)
			return
		}
		deal.State = "revoked"
		deal.UpdatedAt = now.UTC()
		h.finishDeal(c, dns, msg, hash, party, deal, now, database.ORPEffect{
			UpdateDealID: deal.ID, UpdateState: "revoked",
			UpdateFrom: []string{"proposed", "awaiting", "active"}, ClearWaiting: true, UpdatedAt: deal.UpdatedAt,
		})
	}
}

func (h *ORPHost) revokeOffer(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	mode, err := h.modeFor(c, ownerID, database.StepRevoke)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("revoke", "The owner has turned off revoke, so this agent cannot revoke.")), database.ORPEffect{})
		return
	}
	offerID, _ := msg.Body["offer"].(string)
	offer, found, err := h.Entry.DB.GetORPOfferOwned(c.Request.Context(), offerID, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if !found || !offer.Listed {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such listed offer for this sender."}), database.ORPEffect{})
		return
	}
	offer.Listed = false
	h.finish(c, msg, hash, http.StatusOK, orp.ListedOffer(dns, msg.From, msg.ID, map[string]any{
		"offer": offerID, "listed": false, "revoked": true,
	}, h.now()), database.ORPEffect{OfferUpdate: &offer})
}

func (h *ORPHost) dispute(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	action, _ := msg.Body["action"].(string)
	if action == "resolve" {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the host or its named resolver resolves a dispute."}), database.ORPEffect{})
		return
	}
	deal, party, ok := h.partyDeal(c, dns, msg, hash, ownerID)
	if !ok {
		return
	}
	now := h.now()
	effect := database.ORPEffect{}
	switch action {
	case "open":
		if deal.State != "active" {
			h.stateConflict(c, dns, msg, hash)
			return
		}
		effect = h.moveDeal(&deal, "disputed", []string{"active"}, now)
	case "withdraw":
		if deal.State != "disputed" {
			h.stateConflict(c, dns, msg, hash)
			return
		}
		// SPEC §15.2: the opener may withdraw. The stored messages name that party.
		effect = h.moveDeal(&deal, "active", []string{"disputed"}, now)
		effect.DisputeWithdrawFrom = party
	case "respond":
		if deal.State != "disputed" {
			h.stateConflict(c, dns, msg, hash)
			return
		}
	default:
		h.finish(c, msg, hash, http.StatusBadRequest, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusBadRequest, Code: "invalid_message", Message: "action must be open, respond, withdraw or resolve."}), database.ORPEffect{})
		return
	}
	effect.Inbox = append(h.dealInbox(dns, msg.ID, deal, now), h.forwardInbox(msg, deal)...)
	// A counterparty with no owner (a catalog offerer, or a 0.4 seat) gets no
	// forwardInbox row. The sender's own copy is the durable opener record.
	if rawMsg, err := json.Marshal(msg); err == nil && ownerID != "" {
		effect.Inbox = append(effect.Inbox, database.ORPInboxItem{OwnerID: ownerID, Message: rawMsg})
	}
	h.finishDeal(c, dns, msg, hash, party, deal, now, effect)
}

func (h *ORPHost) reputation(c *gin.Context, dns string, msg orp.Message, hash string) {
	subject, _ := msg.Body["subject"].(string)
	ctx := c.Request.Context()
	now := h.now()
	events, err := h.repEvents(ctx, subject)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	score, comp := orp.Reputation(subject, events, func(party string) []orp.RepEvent {
		list, err := h.repEvents(ctx, party)
		if err != nil {
			return nil
		}
		return list
	}, now)
	body := map[string]any{
		"subject":     subject,
		"algorithm":   "orp-rep-1",
		"score":       nil,
		"computed_at": now.UTC().Format(time.RFC3339),
		"valid_until": now.UTC().Add(24 * time.Hour).Format(time.RFC3339),
		"components": map[string]any{
			"verified":        comp.Verified,
			"failed":          comp.Failed,
			"disputes_lost":   comp.DisputesLost,
			"counterparties":  comp.Counterparties,
			"evidence_weight": comp.EvidenceWeight,
		},
	}
	if score != nil {
		body["score"] = *score
	}
	doc := map[string]any{
		"orp": orp.Version, "type": "reputation", "id": orp.NewMessageID(),
		"ts": orp.Stamp(now), "from": orp.HostParty(dns), "to": msg.From, "re": msg.ID,
		"body": body,
	}
	if msg.ORP == orp.Version04 {
		doc["orp"] = orp.Version04
	}
	h.finish(c, msg, hash, http.StatusOK, doc, database.ORPEffect{})
}

// repEvents builds a party's orp-rep-1 inputs from stored deals: a verified
// obligation earns its method weight, a failed one subtracts 1.
func (h *ORPHost) repEvents(ctx context.Context, subject string) ([]orp.RepEvent, error) {
	deals, err := h.Entry.DB.ListORPDealsForParty(ctx, subject, 200)
	if err != nil {
		return nil, err
	}
	var events []orp.RepEvent
	for _, deal := range deals {
		cps := dealCounterparties(deal, subject)
		var rows []map[string]any
		if err := json.Unmarshal(deal.Obligations, &rows); err != nil {
			continue
		}
		// Stored rows keep id and state. The role lives on the terms, which
		// is what a verified proof_url has to be scored against.
		terms, _ := decodeObject(deal.Terms)
		for _, row := range rows {
			by, _ := row["by"].(string)
			if by == "" {
				id, _ := row["id"].(string)
				if ob := orp.ObligationMap(terms, id); ob != nil {
					by, _ = ob["by"].(string)
				}
			}
			if obligorOf(deal, by) != subject {
				continue
			}
			// P7: an attestation carries no public evidence, so it never
			// enters a reputation event — not even as a zero-weight one.
			// A host test fixture is the same: it can verify a TEST deal
			// and it never moves reputation.
			if method, _ := row["method"].(string); method == "attestation" {
				continue
			}
			if fixture, _ := row["test_fixture"].(bool); fixture {
				continue
			}
			for _, cp := range cps {
				switch row["state"] {
				case "verified":
					w := orp.MethodWeight["link"]
					if method, _ := row["method"].(string); method != "" {
						w = orp.MethodWeight[method]
					}
					events = append(events, orp.RepEvent{Counterparty: cp, Weight: w, At: deal.UpdatedAt})
				case "failed":
					events = append(events, orp.RepEvent{Counterparty: cp, Weight: -1, At: deal.UpdatedAt})
				}
			}
		}
	}
	return events, nil
}

// obligorOf maps an obligation's "by" to a party id on a 0.3 or 0.4 deal.
func obligorOf(deal database.ORPDeal, by string) string {
	if len(deal.Parties) == 0 {
		switch by {
		case "offerer":
			return deal.OffererParty
		case "acceptor":
			return deal.AcceptorParty
		}
		return by
	}
	return obligorBy(deal, by)
}

// dealCounterparties returns the other parties on a deal for one subject.
func dealCounterparties(deal database.ORPDeal, subject string) []string {
	var all []string
	if len(deal.Parties) == 0 {
		all = []string{deal.OffererParty, deal.AcceptorParty}
	} else if doc, err := orp.DecodeMany(deal.Parties); err == nil {
		for _, seat := range doc.Seats {
			all = append(all, seat.ID)
		}
	}
	out := []string{}
	for _, party := range all {
		if party != "" && party != subject {
			out = append(out, party)
		}
	}
	return out
}

func (h *ORPHost) partyDeal(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) (database.ORPDeal, string, bool) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return database.ORPDeal{}, "", false
	}
	party := orp.MemberParty(dns, ownerID)
	if !found || !h.onDealParty(party, deal) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return database.ORPDeal{}, "", false
	}
	return deal, party, true
}

func (h *ORPHost) stateConflict(c *gin.Context, dns string, msg orp.Message, hash string) {
	h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
}

func (h *ORPHost) moveDeal(deal *database.ORPDeal, state string, from []string, now time.Time) database.ORPEffect {
	deal.State = state
	deal.UpdatedAt = now.UTC()
	return database.ORPEffect{
		UpdateDealID: deal.ID, UpdateState: state, UpdateFrom: from, UpdatedAt: deal.UpdatedAt,
	}
}

func (h *ORPHost) finishDeal(c *gin.Context, dns string, msg orp.Message, hash, party string, deal database.ORPDeal, now time.Time, effect database.ORPEffect) {
	doc, err := h.dealMessage(dns, party, msg.ID, deal, now)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if len(effect.Inbox) == 0 {
		effect.Inbox = h.dealInbox(dns, msg.ID, deal, now)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	h.finishRaw(c, msg, hash, http.StatusOK, raw, effect)
}

// forwardInbox relays the incoming message itself to every other party, so a
// dispute's claim, statement and evidence reach the counterparty unread by us.
func (h *ORPHost) forwardInbox(msg orp.Message, deal database.ORPDeal) []database.ORPInboxItem {
	raw, err := json.Marshal(msg)
	if err != nil {
		return nil
	}
	var out []database.ORPInboxItem
	for _, cp := range dealCounterparties(deal, msg.From) {
		owner := h.ownerOf(deal, cp)
		if owner != "" {
			out = append(out, database.ORPInboxItem{OwnerID: owner, Message: raw})
		}
	}
	return out
}

func (h *ORPHost) ownerOf(deal database.ORPDeal, party string) string {
	if len(deal.Parties) == 0 {
		switch party {
		case deal.OffererParty:
			return deal.OffererOwner
		case deal.AcceptorParty:
			return deal.AcceptorOwner
		}
		return ""
	}
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		return ""
	}
	for _, seat := range doc.Seats {
		if seat.ID == party {
			return seat.Owner
		}
	}
	return ""
}

func obligationSnapshot(raw json.RawMessage, obligationID string) (state string, found, dup bool) {
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return "", false, false
	}
	for _, row := range rows {
		if row["id"] != obligationID {
			continue
		}
		if found {
			return "", true, true
		}
		found = true
		state, _ = row["state"].(string)
	}
	return state, found, false
}

func revocableObligation(state string) bool {
	return state == "" || state == "pending" || state == "claimed"
}

// markObligation sets one obligation's state; nil rows mean no such id.
func markObligation(raw json.RawMessage, obligationID, state string) (json.RawMessage, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i]["id"] == obligationID {
			rows[i]["state"] = state
			out, err := json.Marshal(rows)
			return out, err
		}
	}
	return nil, nil
}
