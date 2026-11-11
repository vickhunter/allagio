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
		obligationID, _ := msg.Body["obligation"].(string)
		rows, err := markObligation(deal.Obligations, obligationID, "revoked")
		if err != nil {
			h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
			return
		}
		if rows == nil {
			h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such obligation for this sender."}), database.ORPEffect{})
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
		deal.Obligations = rows
		deal.UpdatedAt = now.UTC()
		h.finishDeal(c, dns, msg, hash, party, deal, now, database.ORPEffect{
			UpdateDealID: deal.ID, UpdateObligations: rows, UpdatedAt: deal.UpdatedAt,
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
		effect = h.moveDeal(&deal, "active", []string{"disputed"}, now)
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
		for _, row := range rows {
			by, _ := row["by"].(string)
			if obligorOf(deal, by) != subject {
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
