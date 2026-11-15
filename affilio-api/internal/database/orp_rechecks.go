package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const orpDealSelect = `id, offer_id, offerer_owner::text, acceptor_owner::text, offerer_party, acceptor_party,
       terms, terms_hash, state, waiting, obligations, bound, parties, tracking, updated_at, created_at`

func scanORPDealRow(row pgx.Row) (ORPDeal, error) {
	var deal ORPDeal
	var offerID, offererOwner *string
	err := row.Scan(
		&deal.ID, &offerID, &offererOwner, &deal.AcceptorOwner, &deal.OffererParty, &deal.AcceptorParty,
		&deal.Terms, &deal.TermsHash, &deal.State, &deal.Waiting, &deal.Obligations, &deal.Bound, &deal.Parties, &deal.Tracking, &deal.UpdatedAt, &deal.CreatedAt,
	)
	if err != nil {
		return ORPDeal{}, err
	}
	if offerID != nil {
		deal.OfferID = *offerID
	}
	if offererOwner != nil {
		deal.OffererOwner = *offererOwner
	}
	return deal, nil
}

// ListORPRecheckDue returns active deals carrying a verified obligation
// whose next_check passed — the P6 recheck set.
func (d *DB) ListORPRecheckDue(ctx context.Context, now time.Time, limit int) ([]ORPDeal, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.Pool.Query(ctx, `
SELECT `+orpDealSelect+`
FROM orp_deals
WHERE state='active'
  AND EXISTS (
    SELECT 1 FROM jsonb_array_elements(obligations) o
    WHERE o->>'state'='verified'
      AND o->>'next_check' IS NOT NULL
      AND (o->>'next_check')::timestamptz <= $1)
ORDER BY updated_at
LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ORPDeal
	for rows.Next() {
		deal, err := scanORPDealRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, deal)
	}
	return out, rows.Err()
}

// ApplyORPRecheck writes one deal's recheck outcome in one transaction: the
// obligation rows, the deal state (disputed on a failed recheck), and the
// receipt/dispute inbox items. A stale read writes nothing.
func (d *DB) ApplyORPRecheck(ctx context.Context, dealID string, obligations []byte, state string, expectUpdatedAt, updatedAt time.Time, inbox []ORPInboxItem) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
UPDATE orp_deals
SET obligations=$2::jsonb, state=$3, updated_at=$4
WHERE id=$1 AND updated_at=$5`, dealID, obligations, state, updatedAt, expectUpdatedAt)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	for _, item := range inbox {
		if item.OwnerID == "" || len(item.Message) == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_inbox(owner_id, message) VALUES($1,$2)`, item.OwnerID, item.Message); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// ReleaseORPKeepPayouts ends the keep hold on payouts minted against basis
// (empty basis covers per-deal payouts) so the next hold advance pays them.
func (d *DB) ReleaseORPKeepPayouts(ctx context.Context, dealID, basis string, now time.Time) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE orp_payouts SET hold_until=$3
WHERE deal_id=$1 AND state='held'
  AND COALESCE(basis_obligation,'')=$2`, dealID, basis, now)
	return err
}

// ReverseORPKeepPayouts cancels payouts minted against a keep obligation
// that failed a recheck. Sent and paid records stay — the dispute resolves
// those.
func (d *DB) ReverseORPKeepPayouts(ctx context.Context, dealID, basis string) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE orp_payouts SET state='reversed'
WHERE deal_id=$1 AND state IN ('held','payable')
  AND COALESCE(basis_obligation,'')=$2`, dealID, basis)
	return err
}

