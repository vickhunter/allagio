package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// sweepPayouts is the settlement heartbeat. The host has no payout clock, so
// the sweep runs on activity: a recorded click, an inbox read, a payout
// answer. It advances holds past hold_until, mints payouts a verified
// obligation earns, applies the payer's settle step, and drops a payout
// record into both parties' inboxes at each state change.
func (h *ORPHost) sweepPayouts(c *gin.Context, dns string) {
	ctx := c.Request.Context()
	now := h.now()
	advanced, err := h.Entry.DB.AdvanceORPHolds(ctx, now)
	if err != nil {
		log.Printf("orp payout sweep: %v", err)
		return
	}
	for _, p := range advanced {
		h.settlePayout(c, dns, p, now)
	}
	deals, err := h.Entry.DB.ListORPActiveDealTerms(ctx, 200)
	if err != nil {
		log.Printf("orp payout sweep: %v", err)
		return
	}
	for _, deal := range deals {
		states := make([]orp.ObligationState, 0, len(deal.States))
		for _, row := range deal.States {
			id, _ := row["id"].(string)
			state, _ := row["state"].(string)
			states = append(states, orp.ObligationState{ID: id, State: state})
		}
		ownerOf := func(party string) string {
			switch party {
			case deal.OffererParty:
				return deal.OffererOwner
			case deal.AcceptorParty:
				return deal.AcceptorOwner
			}
			return ""
		}
		for _, plan := range orp.PlanPayouts(deal.Obligations, states, deal.OffererParty, deal.AcceptorParty) {
			payout := database.ORPPayout{
				ID:              orp.LocalID("po_"),
				DealID:          deal.ID,
				Obligation:      plan.Obligation,
				BasisObligation: plan.BasisObligation,
				PayerOwner:      ownerOf(plan.PayerParty),
				PayerParty:      plan.PayerParty,
				PayeeOwner:      ownerOf(plan.PayeeParty),
				PayeeParty:      plan.PayeeParty,
				AmountMinor:     plan.AmountMinor,
				Currency:        plan.Currency,
				Rail:            plan.Rail,
				Basis:           "receipt",
				State:           "payable",
			}
			if payout.PayerOwner == "" || payout.PayeeOwner == "" {
				continue
			}
			if plan.Hold > 0 {
				until := now.Add(plan.Hold)
				payout.HoldUntil = &until
				payout.State = "held"
			}
			created, err := h.Entry.DB.MintORPPayout(ctx, payout)
			if err != nil {
				log.Printf("orp payout mint: %v", err)
				continue
			}
			if !created {
				continue
			}
			payout.CreatedAt = now
			if payout.State == "payable" {
				h.settlePayout(c, dns, payout, now)
			} else {
				h.inboxPayout(c, dns, payout, now)
			}
		}
	}
}

// settlePayout applies the payer's settle step to a payable payout. auto
// releases it — the test rail moves nothing, so sent records the release.
// ask and off leave it payable for the owner to send on the dashboard.
func (h *ORPHost) settlePayout(c *gin.Context, dns string, p database.ORPPayout, now time.Time) {
	mode, err := h.modeFor(c, p.PayerOwner, database.StepPayout)
	if err != nil {
		log.Printf("orp payout settle mode: %v", err)
		mode = database.ModeAsk
	}
	if mode != database.ModeAuto {
		h.inboxPayout(c, dns, p, now)
		return
	}
	sent, ok, err := h.Entry.DB.SettleORPPayout(c.Request.Context(), p.ID, now)
	if err != nil {
		log.Printf("orp payout settle: %v", err)
		return
	}
	if !ok {
		return
	}
	h.inboxPayout(c, dns, sent, now)
}

func (h *ORPHost) inboxPayout(c *gin.Context, dns string, p database.ORPPayout, now time.Time) {
	insertPayoutRecords(c.Request.Context(), h.Entry.DB, dns, p, now)
}

func insertPayoutRecords(ctx context.Context, db *database.DB, dns string, p database.ORPPayout, now time.Time) {
	view := orp.PayoutView{
		ID: p.ID, Obligation: p.Obligation, State: p.State,
		AmountMinor: p.AmountMinor, Currency: p.Currency,
		Rail: p.Rail, Basis: p.Basis, HoldUntil: p.HoldUntil,
	}
	for _, target := range []struct {
		party string
		owner string
	}{{p.PayerParty, p.PayerOwner}, {p.PayeeParty, p.PayeeOwner}} {
		raw, err := json.Marshal(orp.PayoutRecord(dns, target.party, p.DealID, view, now))
		if err != nil {
			log.Printf("orp payout record: %v", err)
			return
		}
		if err := db.InsertORPInbox(ctx, target.owner, raw); err != nil {
			log.Printf("orp payout inbox: %v", err)
		}
	}
}

// payout is the payee's answer to a sent payout. confirm marks it paid;
// not_received keeps it sent so a dispute can still open by due.
func (h *ORPHost) payout(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	h.sweepPayouts(c, dns)
	payoutID, _ := msg.Body["payout"].(string)
	answer, _ := msg.Body["answer"].(string)
	p, found, err := h.Entry.DB.GetORPPayout(c.Request.Context(), payoutID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if !found || p.DealID != msg.Deal {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such payout for this sender."}), database.ORPEffect{})
		return
	}
	if msg.From != p.PayeeParty {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the payee answers a payout."}), database.ORPEffect{})
		return
	}
	if p.State != "sent" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The payout is in the wrong state."}), database.ORPEffect{})
		return
	}
	now := h.now()
	updated, ok, err := h.Entry.DB.AnswerORPPayout(c.Request.Context(), p.ID, answer, now)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if !ok {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The payout is in the wrong state."}), database.ORPEffect{})
		return
	}
	if updated.State == "paid" {
		h.inboxPayout(c, dns, updated, now)
	}
	view := orp.PayoutView{
		ID: updated.ID, Obligation: updated.Obligation, State: updated.State,
		AmountMinor: updated.AmountMinor, Currency: updated.Currency,
		Rail: updated.Rail, Basis: updated.Basis, HoldUntil: updated.HoldUntil,
	}
	h.finish(c, msg, hash, http.StatusOK, orp.PayoutRecord(dns, msg.From, msg.Deal, view, now), database.ORPEffect{})
}

// payoutsSection lists deal payouts on the owner settings page. A payable
// payout waits for the owner's send; the rest show their state.
func (h *HostedHandler) payoutsSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	payouts, err := h.DB.ORPPayoutsForOwner(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	if len(payouts) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section id="deal-payouts"><h2>Deal payouts</h2><p>Money your deals owe or owe you. ORP records the state; the rail moves the money.</p>`)
	for _, p := range payouts {
		b.WriteString(`<div class="deal"><h3><code>`)
		b.WriteString(template.HTMLEscapeString(p.ID))
		b.WriteString(`</code></h3><p>`)
		b.WriteString(template.HTMLEscapeString(formatMinor(p.AmountMinor, p.Currency)))
		b.WriteString(` · `)
		b.WriteString(template.HTMLEscapeString(p.Rail))
		b.WriteString(` · `)
		b.WriteString(template.HTMLEscapeString(p.State))
		if p.State == "payable" && p.PayerOwner == o.ID {
			b.WriteString(`</p><form method="post" action="/owner/orp-payouts/`)
			b.WriteString(template.HTMLEscapeString(p.ID))
			b.WriteString(`/sent"><input type="hidden" name="csrf" value="`)
			b.WriteString(template.HTMLEscapeString(o.CSRF))
			b.WriteString(`"><button>Mark sent</button></form><p>`)
		}
		b.WriteString(`</p></div>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String()), nil
}

func formatMinor(minor int64, currency string) string {
	whole := minor / 100
	frac := minor % 100
	if frac == 0 {
		return itoa64(whole) + " " + currency
	}
	return itoa64(whole) + "." + twoDigits(frac) + " " + currency
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + itoa64(n)
	}
	return itoa64(n)
}

// SendORPPayout is the owner's settle action on a payable payout: the rail
// is manual, so sent is a record that the transfer went out.
func (h *HostedHandler) SendORPPayout(c *gin.Context) {
	o := ownerFrom(c)
	p, found, err := h.DB.GetORPPayout(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !found || p.PayerOwner != o.ID || p.State != "payable" {
		c.Status(http.StatusNotFound)
		return
	}
	now := time.Now().UTC()
	sent, ok, err := h.DB.SettleORPPayout(c.Request.Context(), p.ID, now)
	if err != nil || !ok {
		c.Status(http.StatusInternalServerError)
		return
	}
	base, err := url.Parse(strings.TrimRight(h.BaseURL, "/"))
	if err == nil {
		insertPayoutRecords(c.Request.Context(), h.DB, base.Host, sent, now)
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}
