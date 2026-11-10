package handlers

import (
	"net/url"
	"sort"
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

func ownerDealView(deal database.ORPDeal, ownerID, dns string) ownerDeal {
	view := ownerDeal{ID: deal.ID, State: deal.State, UpdatedAt: deal.UpdatedAt, CreatedAt: deal.CreatedAt}
	if terms, err := decodeObject(deal.Terms); err == nil && terms != nil {
		view.Title = orp.TermsTitle(terms)
		view.Disclosure = orp.DisclosureLabel(terms)
		if kind, _ := terms["kind"].(string); kind != "" {
			view.Kind = kind
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
