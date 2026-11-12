package orp

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// OutcomeEarned is one pay obligation's payout sum inside a period. The
// database layer fills it from minted payouts; a mixed-currency obligation
// never reaches it.
type OutcomeEarned struct {
	Obligation  string
	AmountMinor int64
	Currency    string
}

// PlanOutcomeRows builds the rows of an outcome body. One row per obligation
// that has a tracking entry, in tracking order, counting clicks whose at
// lands inside [from, to). A repeat click counts nowhere. Earned joins the
// row for its obligation, or appends a zero-click row for a pay obligation
// with no tracking entry.
func PlanOutcomeRows(doc Many, from, to time.Time, earned []OutcomeEarned) []any {
	rows := make([]any, 0, len(doc.Tracking))
	rowOf := map[string]map[string]any{}
	for _, entry := range doc.Tracking {
		if entry.Obligation == "" || rowOf[entry.Obligation] != nil {
			continue
		}
		row := map[string]any{
			"obligation": entry.Obligation,
			"clicks":     map[string]any{"human": 0},
		}
		rows = append(rows, row)
		rowOf[entry.Obligation] = row
	}
	for _, click := range doc.Clicks {
		row, ok := rowOf[click.Obligation]
		if !ok {
			continue
		}
		var class string
		switch click.Class {
		case "human", "bot", "preview", "test":
			class = click.Class
		default:
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, click.At)
		if err != nil || at.Before(from) || !at.Before(to) {
			continue
		}
		clicks, _ := row["clicks"].(map[string]any)
		n, _ := clicks[class].(int)
		clicks[class] = n + 1
	}
	for _, item := range earned {
		if item.Obligation == "" || item.AmountMinor <= 0 || item.Currency == "" {
			continue
		}
		money := map[string]any{"amount_minor": item.AmountMinor, "currency": item.Currency}
		if row, ok := rowOf[item.Obligation]; ok {
			row["earned"] = money
			continue
		}
		row := map[string]any{
			"obligation": item.Obligation,
			"clicks":     map[string]any{"human": 0},
			"earned":     money,
		}
		rows = append(rows, row)
		rowOf[item.Obligation] = row
	}
	if len(rows) > 100 {
		rows = rows[:100]
	}
	return rows
}

// OutcomeBody is the outcome body: the period bounds and aggregate rows.
// final is only written when the deal has ended.
func OutcomeBody(from, to time.Time, rows []any, final bool) map[string]any {
	body := map[string]any{
		"period": map[string]any{"from": Stamp(from), "to": Stamp(to)},
		"rows":   rows,
	}
	if final {
		body["final"] = true
	}
	return body
}

// OutcomeID is the deterministic message id for one party's outcome of one
// period: resending the same period to the same party reuses the id.
func OutcomeID(dealID string, periodTo time.Time, party string) string {
	sum := sha256.Sum256([]byte(dealID + "|" + periodTo.UTC().Format(time.RFC3339Nano) + "|" + party))
	return "m_out_" + hex.EncodeToString(sum[:8])
}

// OutcomeMessage is the host's outcome for one party on a deal.
func OutcomeMessage(dns, to, dealID, version, id string, body map[string]any, now time.Time) map[string]any {
	return map[string]any{
		"orp":  version,
		"type": "outcome",
		"id":   id,
		"ts":   Stamp(now),
		"from": HostParty(dns),
		"to":   to,
		"deal": dealID,
		"body": body,
	}
}
