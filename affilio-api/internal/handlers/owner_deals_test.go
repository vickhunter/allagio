package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"affilio-api/internal/database"
)

const dealTestDNS = "example.test"

func dealFixture(t *testing.T, mutate func(*database.ORPDeal)) database.ORPDeal {
	t.Helper()
	deal := database.ORPDeal{
		ID:            "deal-1",
		OffererOwner:  "11111111-1111-1111-1111-111111111111",
		AcceptorOwner: "22222222-2222-2222-2222-222222222222",
		OffererParty:  "orp:" + dealTestDNS + "/11111111-1111-1111-1111-111111111111",
		AcceptorParty: "orp:other.test/agent-9",
		State:         "awaiting",
		UpdatedAt:     time.Now(),
		CreatedAt:     time.Now(),
	}
	deal.Terms = json.RawMessage(`{"title":"Cross-post spring launch","kind":"repost"}`)
	if mutate != nil {
		mutate(&deal)
	}
	return deal
}

func TestOwnerDealViewOffererSeesCounterparty(t *testing.T) {
	deal := dealFixture(t, nil)
	view := ownerDealView(deal, deal.OffererOwner, dealTestDNS)
	if view.Role != "offerer" {
		t.Fatalf("role = %q", view.Role)
	}
	if view.Counterparty != deal.AcceptorParty {
		t.Fatalf("counterparty = %q", view.Counterparty)
	}
	if view.Title != "Cross-post spring launch" || view.Kind != "repost" {
		t.Fatalf("title/kind = %q/%q", view.Title, view.Kind)
	}
	if view.WaitingOnYou || view.ApproveURL != "" {
		t.Fatal("approve surface leaked without a waiting row")
	}
}

func TestOwnerDealViewWaitingOwnerGetsApproveURL(t *testing.T) {
	owner := "22222222-2222-2222-2222-222222222222"
	deal := dealFixture(t, func(d *database.ORPDeal) {
		d.Waiting = json.RawMessage(`{"party":"orp:` + dealTestDNS + `/` + owner + `","step":"accept","escalation":"esc-123"}`)
	})
	view := ownerDealView(deal, owner, dealTestDNS)
	if !view.WaitingOnYou {
		t.Fatal("waiting owner not flagged")
	}
	if view.WaitingStep != "accept" || view.ApproveURL != "/orp/approve/esc-123" {
		t.Fatalf("waiting = %q url = %q", view.WaitingStep, view.ApproveURL)
	}
	if view.Role != "acceptor" || view.Counterparty != deal.OffererParty {
		t.Fatalf("role/counterparty = %q/%q", view.Role, view.Counterparty)
	}
}

func TestOwnerDealViewWaitingOnOtherHidesToken(t *testing.T) {
	owner := "22222222-2222-2222-2222-222222222222"
	deal := dealFixture(t, func(d *database.ORPDeal) {
		d.Waiting = json.RawMessage(`{"party":"orp:other.test/agent-9","step":"execute","escalation":"esc-secret"}`)
	})
	view := ownerDealView(deal, owner, dealTestDNS)
	if view.WaitingOnYou {
		t.Fatal("foreign waiting party marked as owner")
	}
	if view.ApproveURL != "" {
		t.Fatal("escalation token leaked to non-waiting owner")
	}
}

func TestOwnerDealViewPartiesSeats(t *testing.T) {
	owner := "33333333-3333-3333-3333-333333333333"
	deal := dealFixture(t, func(d *database.ORPDeal) {
		d.Parties = json.RawMessage(`{"seats":[{"id":"orp:` + dealTestDNS + `/` + owner + `","owner":"` + owner + `","role":"acceptor","signed":true},{"id":"orp:other.test/agent-9","role":"offerer","signed":false}]}`)
	})
	view := ownerDealView(deal, owner, dealTestDNS)
	if len(view.Parties) != 2 {
		t.Fatalf("parties = %d", len(view.Parties))
	}
	if !view.Parties[0].You || view.Parties[1].You {
		t.Fatal("you flag wrong")
	}
	if view.Role != "acceptor" {
		t.Fatalf("role = %q", view.Role)
	}
}
