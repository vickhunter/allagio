package orp

import (
	"strings"
	"time"
)

// PlannedPayout is one pay obligation ready to become a payout record.
// A pay with after follows that obligation's verification; a pay per deal
// with no after follows every non-pay obligation; a pay per placement with
// no after earns once per verified place obligation. Pay per click,
// per conversion, and rate-only pay plan nothing: they need click counting
// or provider revenue the test pool does not hold.
type PlannedPayout struct {
	Obligation      string
	BasisObligation string
	PayerParty      string
	PayeeParty      string
	AmountMinor     int64
	Currency        string
	Rail            string
	Hold            time.Duration
}

// ObligationState is the stored state row of one deal obligation. Method is
// the receipt method that verified it and Attester the party that attested,
// both empty for other methods (P3: an attestation releases only the
// attester's own money). TestFixture is a host test page: that receipt
// never releases earnings.
type ObligationState struct {
	ID          string
	State       string
	Method      string
	Attester    string
	TestFixture bool
}

// done reports whether the obligation counts as finished for this payer. A
// waived row always counts; a verified row counts for everyone except when
// it was verified by attestation, which counts only for the attester. A
// test-fixture receipt never counts.
func (row ObligationState) done(payer string) bool {
	if row.TestFixture {
		return false
	}
	if row.State == "waived" {
		return true
	}
	if row.State != "verified" {
		return false
	}
	if row.Method == "attestation" && row.Attester != payer {
		return false
	}
	return true
}

// PlanPayouts reads the terms obligations against the deal's obligation
// states. A pay obligation's by is the offerer or acceptor role, so the
// payer is that role's party and the payee is the other side.
func PlanPayouts(obligations []map[string]any, states []ObligationState, offererParty, acceptorParty string) []PlannedPayout {
	stateOf := map[string]ObligationState{}
	for _, row := range states {
		stateOf[row.ID] = row
	}
	nonPayDoneFor := func(payer string) bool {
		for _, ob := range obligations {
			action, _ := ob["action"].(string)
			if action == "pay" {
				continue
			}
			id, _ := ob["id"].(string)
			if !stateOf[id].done(payer) {
				return false
			}
		}
		return true
	}
	var out []PlannedPayout
	for _, ob := range obligations {
		action, _ := ob["action"].(string)
		if action != "pay" {
			continue
		}
		amount, _ := ob["amount"].(map[string]any)
		minor, ok := whole(amount["amount_minor"])
		if !ok || minor <= 0 {
			continue
		}
		currency, _ := amount["currency"].(string)
		if currency == "" {
			continue
		}
		by, _ := ob["by"].(string)
		var payer, payee string
		switch by {
		case "offerer":
			payer, payee = offererParty, acceptorParty
		case "acceptor":
			payer, payee = acceptorParty, offererParty
		default:
			continue
		}
		per, _ := ob["per"].(string)
		after, _ := ob["after"].(string)
		id, _ := ob["id"].(string)
		planned := PlannedPayout{
			Obligation:  id,
			PayerParty:  payer,
			PayeeParty:  payee,
			AmountMinor: int64(minor),
			Currency:    currency,
			Rail:        "manual",
		}
		if rail, _ := ob["rail"].(string); rail != "" {
			planned.Rail = rail
		}
		if hold, _ := ob["hold"].(string); hold != "" {
			if d, ok := ParseISODuration(hold); ok {
				planned.Hold = d
			}
		}
		switch {
		case after != "":
			if !stateOf[after].done(payer) {
				continue
			}
			planned.BasisObligation = after
		case per == "deal":
			if !nonPayDoneFor(payer) {
				continue
			}
		case per == "placement":
			for _, other := range obligations {
				otherAction, _ := other["action"].(string)
				otherID, _ := other["id"].(string)
				if otherAction == "place" && stateOf[otherID].done(payer) {
					row := planned
					row.BasisObligation = otherID
					out = append(out, row)
				}
			}
			continue
		default:
			continue
		}
		out = append(out, planned)
	}
	return out
}

// ParseISODuration reads the ORP duration form P[n]Y[n]M[n]W[n]D(T[n]H[n]M[n]S).
// Years read as 365 days and months as 30; a payout hold is a window, not a
// calendar. ok is false for any other shape.
func ParseISODuration(s string) (time.Duration, bool) {
	if !isoDuration(s) {
		return 0, false
	}
	var total time.Duration
	date, timePart, hasT := strings.Cut(s[1:], "T")
	read := func(part string, scale func(rune) (time.Duration, bool)) bool {
		i := 0
		for i < len(part) {
			j := i
			for j < len(part) && part[j] >= '0' && part[j] <= '9' {
				j++
			}
			if j == i || j >= len(part) {
				i++
				continue
			}
			n := 0
			for _, ch := range part[i:j] {
				n = n*10 + int(ch-'0')
			}
			d, ok := scale(rune(part[j]))
			if !ok {
				return false
			}
			total += time.Duration(n) * d
			i = j + 1
		}
		return true
	}
	if !read(date, func(unit rune) (time.Duration, bool) {
		switch unit {
		case 'Y':
			return 365 * 24 * time.Hour, true
		case 'M':
			return 30 * 24 * time.Hour, true
		case 'W':
			return 7 * 24 * time.Hour, true
		case 'D':
			return 24 * time.Hour, true
		}
		return 0, false
	}) {
		return 0, false
	}
	if hasT && !read(timePart, func(unit rune) (time.Duration, bool) {
		switch unit {
		case 'H':
			return time.Hour, true
		case 'M':
			return time.Minute, true
		case 'S':
			return time.Second, true
		}
		return 0, false
	}) {
		return 0, false
	}
	return total, true
}

// PayoutRecord is the host's payout record message for one party. The body
// matches the spec's PayoutRecord: state, amount, rail, basis, hold_until
// when held, and at for the record instant.
func PayoutRecord(dns, to, dealID string, p PayoutView, now time.Time) map[string]any {
	body := map[string]any{
		"payout":     p.ID,
		"obligation": p.Obligation,
		"state":      p.State,
		"amount": map[string]any{
			"amount_minor": p.AmountMinor,
			"currency":     p.Currency,
		},
		"rail":  p.Rail,
		"basis": p.Basis,
		"at":    Stamp(now),
	}
	if p.HoldUntil != nil && p.State == "held" {
		body["hold_until"] = Stamp(*p.HoldUntil)
	}
	if p.Ref != "" {
		body["ref"] = p.Ref
	}
	return envelope(dns, to, "", dealID, "payout", body, now)
}

// PayoutView is the record a payout message carries. The database row maps
// onto it so this package stays free of storage types.
type PayoutView struct {
	ID          string
	Obligation  string
	State       string
	AmountMinor int64
	Currency    string
	Rail        string
	Basis       string
	HoldUntil   *time.Time
	// Ref is the rail's own transfer reference, once the payer reports it.
	Ref string
}
