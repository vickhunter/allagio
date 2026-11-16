package handlers

import (
	"context"
	"errors"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"affilio-api/internal/chainpay"
	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// dev.allagio.settle-chain: the payer reports a transaction hash as the
// payout ref, and the host reads the chain to move the payout from sent to
// paid. The host never holds keys, signs, or pays gas.

// chainNotBeforeSkew allows for clock skew between the host and the chain
// when a transfer must not predate the payout.
const chainNotBeforeSkew = 10 * time.Minute

// payToFor lists the payee wallets a payer can pay this payout to. It is
// empty off chain rails and once the payout no longer waits on the payer.
func payToFor(ctx context.Context, db *database.DB, p database.ORPPayout) ([]any, error) {
	if !chainpay.IsChainRail(p.Rail) || (p.State != "payable" && p.State != "sent") || p.ChainStatus == "verified" {
		return []any{}, nil
	}
	wallets, err := db.ORPPayoutWallets(ctx, p.PayeeOwner)
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, network := range chainpay.RailNetworks(p.Rail) {
		address := wallets[network]
		units, ok := chainpay.Units(network, p.AmountMinor, p.Currency)
		if address == "" || !ok {
			continue
		}
		ref := "<tx hash>"
		if p.Rail == chainpay.RailX402 {
			ref = network + ":<tx hash>"
		}
		out = append(out, map[string]any{
			"network":      network,
			"token":        chainpay.Networks[network].Token,
			"address":      address,
			"amount_units": units.String(),
			"ref":          ref,
		})
	}
	return out, nil
}

// payoutDoc is the host's view of one payout for one owner. pay_to is the
// payee's public wallet and goes to the payer only; it never appears in an
// ORP message.
func payoutDoc(ctx context.Context, db *database.DB, dns, ownerID string, p database.ORPPayout, now time.Time) (map[string]any, error) {
	to := p.PayeeParty
	if ownerID == p.PayerOwner {
		to = p.PayerParty
	}
	doc := map[string]any{
		"record": orp.PayoutRecord(dns, to, p.DealID, payoutView(p), now),
	}
	if p.ChainTx != "" {
		chain := map[string]any{"tx": p.ChainTx, "status": p.ChainStatus}
		if p.ChainReason != "" {
			chain["reason"] = p.ChainReason
		}
		if p.ChainCheckedAt != nil {
			chain["checked_at"] = orp.Stamp(*p.ChainCheckedAt)
		}
		doc["chain"] = chain
	}
	if ownerID == p.PayerOwner {
		payTo, err := payToFor(ctx, db, p)
		if err != nil {
			return nil, err
		}
		doc["pay_to"] = payTo
	}
	return doc, nil
}

func payoutView(p database.ORPPayout) orp.PayoutView {
	return orp.PayoutView{
		ID: p.ID, Obligation: p.Obligation, State: p.State,
		AmountMinor: p.AmountMinor, Currency: p.Currency,
		Rail: p.Rail, Basis: p.Basis, HoldUntil: p.HoldUntil, Ref: p.Ref,
	}
}

// checkChainPayout reads the chain for one sent payout and records the
// result. A verified transfer marks it paid and drops a record into both
// inboxes. A nil verifier leaves the payout untouched.
func checkChainPayout(ctx context.Context, db *database.DB, verifier *chainpay.Verifier, dns string, p database.ORPPayout, now time.Time) database.ORPPayout {
	if verifier == nil || p.State != "sent" || p.ChainTx == "" || p.ChainStatus == "verified" || p.ChainStatus == "mismatch" {
		return p
	}
	cut := strings.LastIndex(p.ChainTx, ":")
	if cut < 0 {
		return p
	}
	network, tx := p.ChainTx[:cut], p.ChainTx[cut+1:]
	payTo := p.ChainPayTo
	if payTo == "" {
		wallets, err := db.ORPPayoutWallets(ctx, p.PayeeOwner)
		if err != nil {
			log.Printf("orp chain payout %s: %v", p.ID, err)
			return p
		}
		payTo = wallets[network]
	}
	var units *big.Int
	if u, ok := chainpay.Units(network, p.AmountMinor, p.Currency); ok {
		units = u
	}
	result := verifier.Verify(ctx, chainpay.Check{
		Network:   network,
		Tx:        tx,
		PayTo:     payTo,
		MinUnits:  units,
		NotBefore: p.CreatedAt.Add(-chainNotBeforeSkew),
	})
	updated, ok, err := db.SetORPPayoutChain(ctx, p.ID, p.ChainTx, string(result.Status), result.Reason, now)
	if err != nil || !ok {
		if err != nil {
			log.Printf("orp chain payout %s: %v", p.ID, err)
		}
		return p
	}
	if updated.State == "paid" {
		insertPayoutRecords(ctx, db, dns, updated, now)
	}
	return updated
}

// SweepORPChainPayouts re-reads every sent chain payout still waiting on the
// chain. The daily worker runs it; payout reads and sends check on demand.
func SweepORPChainPayouts(ctx context.Context, db *database.DB, publicBaseURL string, verifier *chainpay.Verifier, now time.Time) (int, error) {
	if verifier == nil {
		return 0, nil
	}
	_, dns := orp.Origin(publicBaseURL, "", "")
	open, err := db.ListORPChainOpen(ctx, "", 100)
	if err != nil {
		return 0, err
	}
	paid := 0
	for _, p := range open {
		if checkChainPayout(ctx, db, verifier, dns, p, now).State == "paid" {
			paid++
		}
	}
	return paid, nil
}

// sweepChainForOwner checks a few of the owner's open chain payouts on an
// inbox read, skipping any read in the last 30 seconds.
func (h *ORPHost) sweepChainForOwner(c *gin.Context, dns, ownerID string) {
	if h.Chain == nil {
		return
	}
	ctx := c.Request.Context()
	open, err := h.Entry.DB.ListORPChainOpen(ctx, ownerID, 3)
	if err != nil {
		log.Printf("orp chain sweep: %v", err)
		return
	}
	now := h.now()
	for _, p := range open {
		if p.ChainCheckedAt != nil && now.Sub(*p.ChainCheckedAt) < 30*time.Second {
			continue
		}
		checkChainPayout(ctx, h.Entry.DB, h.Chain, dns, p, now)
	}
}

// refProblem validates a reported ref for the payout's rail. It returns the
// chain transaction key and the payee wallet to check, both empty off chain
// rails, or a problem message.
func refProblem(ctx context.Context, db *database.DB, p database.ORPPayout, ref string) (chainTx, payTo, message string, err error) {
	if !chainpay.IsChainRail(p.Rail) {
		if utf8.RuneCountInString(ref) > 200 || strings.IndexFunc(ref, unicode.IsControl) >= 0 {
			return "", "", "ref is at most 200 characters with no control characters.", nil
		}
		return "", "", "", nil
	}
	network, tx, ok := chainpay.ParseRef(p.Rail, ref)
	if !ok {
		if p.Rail == chainpay.RailX402 {
			return "", "", "ref on x402 is <caip2 network>:<0x transaction hash>, for example eip155:8453:0x…", nil
		}
		return "", "", "ref on " + p.Rail + " is the 0x transaction hash.", nil
	}
	if _, ok := chainpay.Units(network, p.AmountMinor, p.Currency); !ok {
		return "", "", "A chain rail settles USD payouts in USDC only.", nil
	}
	wallets, err := db.ORPPayoutWallets(ctx, p.PayeeOwner)
	if err != nil {
		return "", "", "", err
	}
	if wallets[network] == "" {
		return "", "", "The payee has no wallet on " + network + ".", nil
	}
	return network + ":" + tx, wallets[network], "", nil
}

// Payout is GET /orp/payouts/:id for the payer or the payee. A sent chain
// payout is re-read first, so polling shows progress.
func (h *ORPHost) Payout(c *gin.Context) {
	_, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	p, found, err := h.Entry.DB.GetORPPayout(ctx, c.Param("id"))
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if !found || (p.PayerOwner != grant.OwnerID && p.PayeeOwner != grant.OwnerID) {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such payout for this sender."}, false)
		return
	}
	now := h.now()
	p = checkChainPayout(ctx, h.Entry.DB, h.Chain, dns, p, now)
	doc, err := payoutDoc(ctx, h.Entry.DB, dns, grant.OwnerID, p, now)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	writeORP(c, http.StatusOK, doc, false)
}

// PayoutSent is POST /orp/payouts/:id/sent: the payer reports the transfer
// with the rail's ref. A payable payout becomes sent; on a chain rail the
// host reads the transaction at once and marks the payout paid when it
// matches. A sent payout that is not yet paid takes a corrected ref.
func (h *ORPHost) PayoutSent(c *gin.Context) {
	_, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok {
		return
	}
	body, problem, ok := readORPBody(c)
	if !ok {
		h.writeProblem(c, dns, "", "", problem, false)
		return
	}
	ref := ""
	if len(strings.TrimSpace(string(body))) > 0 {
		value, problem, ok := orp.DecodeJSON(body)
		if !ok {
			h.writeProblem(c, dns, "", "", problem, false)
			return
		}
		obj, isObj := value.(map[string]any)
		if !isObj {
			h.writeProblem(c, dns, "", "", orp.Invalid("The body is an object with ref.", ""), false)
			return
		}
		for key := range obj {
			if key != "ref" {
				h.writeProblem(c, dns, "", "", orp.Invalid("Only ref is accepted.", "/"+key), false)
				return
			}
		}
		if raw, present := obj["ref"]; present {
			text, isText := raw.(string)
			if !isText {
				h.writeProblem(c, dns, "", "", orp.Invalid("ref is a string.", "/ref"), false)
				return
			}
			ref = strings.TrimSpace(text)
		}
	}
	ctx := c.Request.Context()
	p, found, err := h.Entry.DB.GetORPPayout(ctx, c.Param("id"))
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if !found || (p.PayerOwner != grant.OwnerID && p.PayeeOwner != grant.OwnerID) {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such payout for this sender."}, false)
		return
	}
	if p.PayerOwner != grant.OwnerID {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the payer reports a payout sent."}, false)
		return
	}
	mode, err := h.modeFor(c, p.PayerOwner, database.StepPayout)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if mode == database.ModeOff {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "The payer's payout step is off: the owner marks it sent on the dashboard."}, false)
		return
	}
	if chainpay.IsChainRail(p.Rail) && ref == "" {
		h.writeProblem(c, dns, "", "", orp.Invalid("A chain rail needs the transaction as ref.", "/ref"), false)
		return
	}
	chainTx, payTo, message, err := refProblem(ctx, h.Entry.DB, p, ref)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if message != "" {
		h.writeProblem(c, dns, "", "", orp.Invalid(message, "/ref"), false)
		return
	}
	now := h.now()
	sent, ok, err := h.Entry.DB.RecordORPPayoutRef(ctx, p.ID, ref, chainTx, payTo, now)
	if errors.Is(err, database.ErrORPRefUsed) {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "This transaction already settles another payout."}, false)
		return
	}
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if !ok {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The payout is in the wrong state."}, false)
		return
	}
	insertPayoutRecords(ctx, h.Entry.DB, dns, sent, now)
	sent = checkChainPayout(ctx, h.Entry.DB, h.Chain, dns, sent, now)
	doc, err := payoutDoc(ctx, h.Entry.DB, dns, grant.OwnerID, sent, now)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	writeORP(c, http.StatusOK, doc, false)
}

// walletsSection is the payee's receiving addresses on the settings page.
func (h *HostedHandler) walletsSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	wallets, err := h.DB.ORPPayoutWallets(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<section id="payout-wallets"><h2>Payout wallets</h2><p>Where a deal paid on a chain rail (<code>usdc.base</code>, <code>usdc.base-sepolia</code>, <code>x402</code>) sends your USDC. Allagio only reads the chain to confirm the transfer: it holds no keys and moves no money. The address is shown to the payer of a payable payout and never appears in an ORP message.</p><form method="post" action="/owner/payout-wallets">`)
	b.WriteString(`<input type="hidden" name="csrf" value="`)
	b.WriteString(template.HTMLEscapeString(o.CSRF))
	b.WriteString(`">`)
	for _, network := range chainpay.NetworkIDs {
		b.WriteString(`<label>`)
		b.WriteString(template.HTMLEscapeString(chainpay.Networks[network].Name))
		b.WriteString(` (<code>`)
		b.WriteString(template.HTMLEscapeString(network))
		b.WriteString(`</code>)<input type="text" name="wallet_`)
		b.WriteString(template.HTMLEscapeString(network))
		b.WriteString(`" maxlength="42" pattern="0x[0-9a-fA-F]{40}" value="`)
		b.WriteString(template.HTMLEscapeString(wallets[network]))
		b.WriteString(`" placeholder="0x…"></label>`)
	}
	b.WriteString(`<button>Save</button></form></section>`)
	return template.HTML(b.String()), nil
}

// SavePayoutWalletsForm is POST /owner/payout-wallets. An empty field
// removes that network's wallet.
func (h *HostedHandler) SavePayoutWalletsForm(c *gin.Context) {
	o := ownerFrom(c)
	values := map[string]string{}
	for _, network := range chainpay.NetworkIDs {
		address := strings.TrimSpace(c.PostForm("wallet_" + network))
		if address != "" && !chainpay.ValidAddress(address) {
			c.Status(http.StatusBadRequest)
			return
		}
		values[network] = address
	}
	for _, network := range chainpay.NetworkIDs {
		if err := h.DB.SetORPPayoutWallet(c.Request.Context(), o.ID, network, values[network]); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}
