package database

import (
	"context"
	"time"

	"affilio-api/internal/orp"
)

// ORPOutcomeDue is one deal the outcome sweep owes a message and the period
// it covers. Final marks the closing send on an ended deal.
type ORPOutcomeDue struct {
	Deal       ORPDeal
	PeriodFrom time.Time
	PeriodTo   time.Time
	Final      bool
}

const orpOutcomeDealCols = `d.id, d.offer_id, d.offerer_owner::text, d.acceptor_owner::text,
	d.offerer_party, d.acceptor_party, d.terms, d.terms_hash, d.state,
	d.waiting, d.obligations, d.bound, d.parties, d.tracking,
	d.updated_at, d.created_at`

// ListORPOutcomeDue returns deals the outcome sweep owes a message: every
// active deal that has tracking or a pay obligation whose last period ended
// at least a week ago or never ran, and every ended deal that got at least
// one non-final outcome but no final yet. The first period of a deal starts
// at its created_at; there is no earlier activation timestamp.
func (d *DB) ListORPOutcomeDue(ctx context.Context, now time.Time, limit int) ([]ORPOutcomeDue, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx, `
SELECT `+orpOutcomeDealCols+`, last.period_to, false AS final
FROM orp_deals d
LEFT JOIN LATERAL (
    SELECT max(period_to) AS period_to FROM orp_outcomes WHERE deal_id=d.id
) last ON true
WHERE d.state='active'
  AND (d.tracking IS NOT NULL OR d.parties IS NOT NULL
       OR d.terms->'obligations' @> '[{"action":"pay"}]'::jsonb)
  AND (last.period_to IS NULL OR last.period_to <= $1::timestamptz - interval '7 days')
UNION ALL
SELECT `+orpOutcomeDealCols+`, last.period_to, true AS final
FROM orp_deals d
CROSS JOIN LATERAL (
    SELECT max(period_to) AS period_to, bool_or(final) AS has_final
    FROM orp_outcomes WHERE deal_id=d.id
) last
WHERE d.state IN ('completed','expired','revoked')
  AND last.period_to IS NOT NULL
  AND NOT COALESCE(last.has_final, false)
ORDER BY created_at
LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ORPOutcomeDue{}
	for rows.Next() {
		var item ORPOutcomeDue
		var offerID, offererOwner *string
		var lastTo *time.Time
		if err = rows.Scan(
			&item.Deal.ID, &offerID, &offererOwner, &item.Deal.AcceptorOwner,
			&item.Deal.OffererParty, &item.Deal.AcceptorParty,
			&item.Deal.Terms, &item.Deal.TermsHash, &item.Deal.State,
			&item.Deal.Waiting, &item.Deal.Obligations, &item.Deal.Bound,
			&item.Deal.Parties, &item.Deal.Tracking,
			&item.Deal.UpdatedAt, &item.Deal.CreatedAt,
			&lastTo, &item.Final,
		); err != nil {
			return nil, err
		}
		if offerID != nil {
			item.Deal.OfferID = *offerID
		}
		if offererOwner != nil {
			item.Deal.OffererOwner = *offererOwner
		}
		item.PeriodFrom = item.Deal.CreatedAt
		if lastTo != nil {
			item.PeriodFrom = *lastTo
		}
		item.PeriodTo = now
		out = append(out, item)
	}
	return out, rows.Err()
}

// ORPOutcomeEarned sums payouts minted inside one period on a deal, per pay
// obligation. Reversed and failed payouts do not count, and an obligation
// whose payouts mix currencies is skipped: one row cannot name two monies.
func (d *DB) ORPOutcomeEarned(ctx context.Context, dealID string, from, to time.Time) ([]orp.OutcomeEarned, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT obligation, SUM(amount_minor), MIN(currency)
FROM orp_payouts
WHERE deal_id=$1 AND created_at >= $2 AND created_at < $3
  AND state NOT IN ('reversed','failed')
GROUP BY obligation
HAVING COUNT(DISTINCT currency) = 1
ORDER BY obligation`, dealID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []orp.OutcomeEarned{}
	for rows.Next() {
		var item orp.OutcomeEarned
		if err = rows.Scan(&item.Obligation, &item.AmountMinor, &item.Currency); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// RecordORPOutcome stores the period marker and the per-party inbox copies
// of one outcome in a single transaction. The (deal_id, period_to) primary
// key and the one-final-per-deal index make a repeated sweep a no-op:
// created is false and no inbox rows are written.
func (d *DB) RecordORPOutcome(ctx context.Context, dealID string, from, to time.Time, final bool, inbox []ORPInboxItem) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
INSERT INTO orp_outcomes(deal_id, period_from, period_to, final)
VALUES($1,$2,$3,$4)
ON CONFLICT DO NOTHING`, dealID, from, to, final)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	for _, item := range inbox {
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_inbox(owner_id, message) VALUES($1::uuid,$2)`, item.OwnerID, item.Message); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
