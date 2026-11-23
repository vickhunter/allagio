package handlers

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

// ownerDeal is the workspace view of one ORP deal. It carries no terms body:
// the approval page shows terms before an owner decides.
type ownerDeal struct {
	ID           string           `json:"id"`
	State        string           `json:"state"`
	Role         string           `json:"role"`
	Title        string           `json:"title"`
	Kind         string           `json:"kind"`
	Disclosure   string           `json:"disclosure,omitempty"`
	Counterparty string           `json:"counterparty,omitempty"`
	Parties      []ownerDealParty `json:"parties,omitempty"`
	Obligations  []string         `json:"obligations,omitempty"`
	ProofURL     string           `json:"proof_url,omitempty"`
	Terms        map[string]any   `json:"terms,omitempty"`
	WaitingOnYou bool             `json:"waiting_on_you"`
	WaitingStep  string           `json:"waiting_step,omitempty"`
	ApproveURL   string           `json:"approve_url,omitempty"`
	UpdatedAt    time.Time        `json:"updated_at"`
	CreatedAt    time.Time        `json:"created_at"`
}

type ownerDealParty struct {
	ID   string `json:"id"`
	Role string `json:"role,omitempty"`
	You  bool   `json:"you"`
}

// OwnerDeals lists the signed-in owner's ORP deals for the workspace inbox.
// An approve URL is only returned for a deal whose waiting party is this owner;
// the token stays out of JSON for every other deal.
func (h *HostedHandler) OwnerDeals(c *gin.Context) {
	o := ownerFrom(c)
	deals, err := h.DB.ListOwnerORPDeals(c.Request.Context(), o.ID, 50)
	if err != nil {
		c.Status(500)
		return
	}
	dns := ""
	if base, err := url.Parse(h.BaseURL); err == nil {
		dns = base.Host
	}
	out := make([]ownerDeal, 0, len(deals))
	for _, deal := range deals {
		out = append(out, ownerDealView(deal, o.ID, dns))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].WaitingOnYou && !out[j].WaitingOnYou
	})
	c.JSON(200, gin.H{"deals": out})
}

// OwnerDeal is the workspace detail for one deal: the same view as the list
// plus the message timeline this owner's inbox recorded for it.
// A deal the owner does not sit on answers 404, same as an unknown id.
func (h *HostedHandler) OwnerDeal(c *gin.Context) {
	o := ownerFrom(c)
	deal, found, err := h.DB.GetORPDeal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Status(500)
		return
	}
	if !found || !dealHasOwner(deal, o.ID) {
		c.JSON(404, gin.H{"reason": "not_found"})
		return
	}
	dns := ""
	if base, err := url.Parse(h.BaseURL); err == nil {
		dns = base.Host
	}
	messages, err := h.DB.ListORPDealEvents(c.Request.Context(), o.ID, deal.ID, 200)
	if err != nil {
		c.Status(500)
		return
	}
	events := make([]gin.H, 0, len(messages))
	for _, raw := range messages {
		doc, err := decodeObject(raw)
		if err != nil {
			continue
		}
		event := gin.H{
			"type": doc["type"],
			"from": doc["from"],
			"ts":   doc["ts"],
		}
		if body, ok := doc["body"].(map[string]any); ok {
			if state, _ := body["state"].(string); state != "" {
				event["state"] = state
			}
		}
		events = append(events, event)
	}
	view := ownerDealView(deal, o.ID, dns)
	if terms, err := decodeObject(deal.Terms); err == nil {
		view.Terms = terms
	}
	c.JSON(200, gin.H{"deal": view, "events": events})
}

// dealHasOwner reports whether the owner sits on this deal —
// as offerer or acceptor on 0.3, or holding a parties seat on 0.4.
func dealHasOwner(deal database.ORPDeal, ownerID string) bool {
	if len(deal.Parties) != 0 && string(deal.Parties) != "null" {
		if doc, err := decodeObject(deal.Parties); err == nil {
			if seats, _ := doc["seats"].([]any); len(seats) > 0 {
				for _, raw := range seats {
					seat, _ := raw.(map[string]any)
					if owner, _ := seat["owner"].(string); owner == ownerID {
						return true
					}
				}
				return false
			}
		}
	}
	return deal.OffererOwner == ownerID || deal.AcceptorOwner == ownerID
}

func ownerDealView(deal database.ORPDeal, ownerID, dns string) ownerDeal {
	view := ownerDeal{
		ID:        deal.ID,
		State:     deal.State,
		ProofURL:  "/orp/proof/" + url.PathEscape(deal.ID),
		UpdatedAt: deal.UpdatedAt,
		CreatedAt: deal.CreatedAt,
	}
	if terms, err := decodeObject(deal.Terms); err == nil && terms != nil {
		view.Title = orp.TermsTitle(terms)
		view.Disclosure = orp.DisclosureLabel(terms)
		if kind, _ := terms["kind"].(string); kind != "" {
			view.Kind = kind
		}
		if rawObls, _ := terms["obligations"].([]any); len(rawObls) > 0 {
			view.Obligations = summarizeObligations(rawObls)
		}
	}
	if len(deal.Parties) != 0 && string(deal.Parties) != "null" {
		if doc, err := decodeObject(deal.Parties); err == nil {
			seats, _ := doc["seats"].([]any)
			for _, raw := range seats {
				seat, _ := raw.(map[string]any)
				party := ownerDealParty{}
				party.ID, _ = seat["id"].(string)
				party.Role, _ = seat["role"].(string)
				seatOwner, _ := seat["owner"].(string)
				party.You = seatOwner == ownerID
				view.Parties = append(view.Parties, party)
			}
		}
		view.Role = "party"
		for _, party := range view.Parties {
			if party.You {
				view.Role = party.Role
			}
		}
	} else {
		if deal.OffererOwner == ownerID {
			view.Role = "offerer"
			view.Counterparty = deal.AcceptorParty
		} else {
			view.Role = "acceptor"
			view.Counterparty = deal.OffererParty
		}
	}
	if waiting, err := decodeObject(deal.Waiting); err == nil && waiting != nil {
		step, _ := waiting["step"].(string)
		party, _ := waiting["party"].(string)
		if waitingOwner, ok := orp.OwnerFromParty(dns, party); ok && waitingOwner == ownerID {
			view.WaitingOnYou = true
			view.WaitingStep = step
			if escalation, _ := waiting["escalation"].(string); escalation != "" {
				view.ApproveURL = "/orp/approve/" + escalation
			}
		}
	}
	return view
}

func summarizeObligations(rawObls []any) []string {
	out := make([]string, 0, len(rawObls))
	for _, raw := range rawObls {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		label, _ := item["action"].(string)
		if label == "" {
			label, _ = item["type"].(string)
		}
		if label == "" {
			label = "obligation"
		}
		var parts []string
		parts = append(parts, label)
		if amount, ok := item["amount"]; ok && amount != nil {
			if formatted := formatObligationAmount(amount, item); formatted != "" {
				parts = append(parts, formatted)
			}
		} else if surface, _ := item["surface"].(string); surface != "" {
			parts = append(parts, surface)
		} else if dest, _ := item["destination"].(string); dest != "" {
			parts = append(parts, dest)
		}
		out = append(out, strings.Join(parts, " · "))
	}
	return out
}

func formatObligationAmount(amount any, item map[string]any) string {
	if m, ok := amount.(map[string]any); ok {
		currency, _ := m["currency"].(string)
		if minor, ok := numberToFloat(m["amount_minor"]); ok {
			val := fmt.Sprintf("%.2f", minor/100)
			if currency != "" {
				return val + " " + currency
			}
			return val
		}
		if v, ok := numberToFloat(m["value"]); ok {
			val := fmt.Sprintf("%g", v)
			if currency != "" {
				return val + " " + currency
			}
			return val
		}
		return ""
	}
	currency, _ := item["currency"].(string)
	if currency != "" {
		return fmt.Sprintf("%v %s", amount, currency)
	}
	return fmt.Sprintf("%v", amount)
}

func numberToFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
