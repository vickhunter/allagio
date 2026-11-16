package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/chainpay"
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
		keepUntil := map[string]time.Time{}
		for _, row := range deal.States {
			id, _ := row["id"].(string)
			state, _ := row["state"].(string)
			method, _ := row["method"].(string)
			attester, _ := row["attester"].(string)
			states = append(states, orp.ObligationState{ID: id, State: state, Method: method, Attester: attester})
			// A verified obligation still inside its keep rechecks holds the
			// money that depends on it until the final check passes (P6).
			if next, _ := row["next_check"].(string); next != "" {
				if until, err := time.Parse(time.RFC3339, str(row["keep_until"])); err == nil {
					keepUntil[id] = until
				}
			}
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
			var keepHold time.Time
			if until, ok := keepUntil[plan.BasisObligation]; ok {
				keepHold = until
			}
			if plan.BasisObligation == "" {
				for _, until := range keepUntil {
					if until.After(keepHold) {
						keepHold = until
					}
				}
			}
			if keepHold.After(now) {
				payout.HoldUntil = &keepHold
				payout.State = "held"
			} else if plan.Hold > 0 {
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
	// A chain rail is paid from the payer's own wallet: the host cannot
	// drive it, so the payout waits for the payer's ref.
	if mode != database.ModeAuto || chainpay.IsChainRail(p.Rail) {
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
	view := payoutView(p)
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
	view := payoutView(updated)
	h.finish(c, msg, hash, http.StatusOK, orp.PayoutRecord(dns, msg.From, msg.Deal, view, now), database.ORPEffect{})
}

// payoutsSection lists deal payouts on the owner settings page. A payable
// payout waits for the owner's send; the rest show their state.
func (h *HostedHandler) payoutsSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	wallets, err := h.walletsSection(c, o)
	if err != nil {
		return "", err
	}
	payouts, err := h.DB.ORPPayoutsForOwner(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	if len(payouts) == 0 {
		return wallets, nil
	}
	var b strings.Builder
	b.WriteString(string(wallets))
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
		if p.Ref != "" {
			b.WriteString(` · ref <code>`)
			b.WriteString(template.HTMLEscapeString(p.Ref))
			b.WriteString(`</code>`)
		}
		if p.ChainStatus != "" {
			b.WriteString(` · chain `)
			b.WriteString(template.HTMLEscapeString(p.ChainStatus))
			if p.ChainReason != "" {
				b.WriteString(` (`)
				b.WriteString(template.HTMLEscapeString(p.ChainReason))
				b.WriteString(`)`)
			}
		}
		chain := chainpay.IsChainRail(p.Rail)
		awaitsRef := p.State == "payable" || (chain && p.State == "sent" && p.ChainStatus != "verified")
		if awaitsRef && p.PayerOwner == o.ID {
			if chain {
				payTo, err := payToFor(c.Request.Context(), h.DB, p)
				if err != nil {
					return "", err
				}
				if len(payTo) == 0 {
					b.WriteString(`</p><p class="caption">The payee has not set a wallet for this rail yet.`)
				}
				for _, item := range payTo {
					target, _ := item.(map[string]any)
					b.WriteString(`</p><p class="caption">Pay `)
					b.WriteString(template.HTMLEscapeString(formatMinor(p.AmountMinor, "USDC")))
					b.WriteString(` on `)
					b.WriteString(template.HTMLEscapeString(target["network"].(string)))
					b.WriteString(` to <code>`)
					b.WriteString(template.HTMLEscapeString(target["address"].(string)))
					b.WriteString(`</code>, then paste the transaction as <code>`)
					b.WriteString(template.HTMLEscapeString(target["ref"].(string)))
					b.WriteString(`</code>.`)
				}
			}
			b.WriteString(`</p><form method="post" action="/owner/orp-payouts/`)
			b.WriteString(template.HTMLEscapeString(p.ID))
			b.WriteString(`/sent"><input type="hidden" name="csrf" value="`)
			b.WriteString(template.HTMLEscapeString(o.CSRF))
			b.WriteString(`"><label>Transfer ref<input type="text" name="ref" maxlength="200"`)
			if chain {
				b.WriteString(` required placeholder="0x… transaction hash"`)
			} else {
				b.WriteString(` placeholder="optional"`)
			}
			b.WriteString(`></label><button>Mark sent</button></form><p>`)
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
	ctx := c.Request.Context()
	p, found, err := h.DB.GetORPPayout(ctx, c.Param("id"))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	chain := chainpay.IsChainRail(p.Rail)
	resend := chain && p.State == "sent" && p.ChainStatus != "verified"
	if !found || p.PayerOwner != o.ID || (p.State != "payable" && !resend) {
		c.Status(http.StatusNotFound)
		return
	}
	ref := strings.TrimSpace(c.PostForm("ref"))
	if chain && ref == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	chainTx, payTo, message, err := refProblem(ctx, h.DB, p, ref)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if message != "" {
		c.String(http.StatusBadRequest, message)
		return
	}
	now := time.Now().UTC()
	var sent database.ORPPayout
	var ok bool
	if ref == "" {
		sent, ok, err = h.DB.SettleORPPayout(ctx, p.ID, now)
	} else {
		sent, ok, err = h.DB.RecordORPPayoutRef(ctx, p.ID, ref, chainTx, payTo, now)
	}
	if errors.Is(err, database.ErrORPRefUsed) {
		c.String(http.StatusConflict, "This transaction already settles another payout.")
		return
	}
	if err != nil || !ok {
		c.Status(http.StatusInternalServerError)
		return
	}
	base, err := url.Parse(strings.TrimRight(h.BaseURL, "/"))
	if err == nil {
		insertPayoutRecords(ctx, h.DB, base.Host, sent, now)
		checkChainPayout(ctx, h.DB, h.Chain, base.Host, sent, now)
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}
