package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrORPRefUsed is returned when a chain transaction already settled
// another payout: verified on the chain, or on a payout the payee confirmed.
// A ref that settled nothing reserves nothing.
var ErrORPRefUsed = errors.New("orp payout: transaction already used")

// RecordORPPayoutRef stores the payer's transfer reference. A payable payout
// moves to sent. A sent payout not yet confirmed takes a corrected ref, so a
// payer can fix a wrong hash. chainTx is empty on a rail the host does not
// read; otherwise the chain check starts pending against payTo, the payee
// wallet in force now.
func (d *DB) RecordORPPayoutRef(ctx context.Context, id, ref, chainTx, payTo string, now time.Time) (ORPPayout, bool, error) {
	var tx *string
	status := (*string)(nil)
	if chainTx != "" {
		used, err := d.orpChainTxUsed(ctx, chainTx, id)
		if err != nil {
			return ORPPayout{}, false, err
		}
		if used {
			return ORPPayout{}, false, ErrORPRefUsed
		}
		tx = &chainTx
		pending := "pending"
		status = &pending
	}
	p, err := scanORPPayout(d.Pool.QueryRow(ctx, `
UPDATE orp_payouts SET state='sent', sent_at=COALESCE(sent_at,$2), ref=$3,
  chain_tx=$4, chain_status=$5, chain_reason=NULL, chain_checked_at=NULL,
  chain_pay_to=NULLIF($6,'')
WHERE id=$1 AND (state='payable' OR (state='sent' AND COALESCE(chain_status,'')<>'verified'))
RETURNING `+orpPayoutCols, id, now, ref, tx, status, payTo))
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPPayout{}, false, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ORPPayout{}, false, ErrORPRefUsed
	}
	return p, err == nil, err
}

// SetORPPayoutChain records one chain read on a sent payout. verified moves
// it to paid; any other status keeps it sent. A transaction another payout
// settled first is a mismatch with reason tx_used.
func (d *DB) SetORPPayoutChain(ctx context.Context, id, chainTx, status, reason string, now time.Time) (ORPPayout, bool, error) {
	if status == "verified" {
		used, err := d.orpChainTxUsed(ctx, chainTx, id)
		if err != nil {
			return ORPPayout{}, false, err
		}
		if used {
			return d.setORPPayoutChain(ctx, id, chainTx, "mismatch", "tx_used", now)
		}
	}
	p, ok, err := d.setORPPayoutChain(ctx, id, chainTx, status, reason, now)
	var pgErr *pgconn.PgError
	if status == "verified" && errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return d.setORPPayoutChain(ctx, id, chainTx, "mismatch", "tx_used", now)
	}
	return p, ok, err
}

// orpChainTxUsed reports whether another payout already settled with this
// transaction.
func (d *DB) orpChainTxUsed(ctx context.Context, chainTx, id string) (bool, error) {
	var used bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orp_payouts
WHERE chain_tx=$1 AND id<>$2 AND (chain_status='verified' OR state='paid'))`, chainTx, id).Scan(&used)
	return used, err
}

func (d *DB) setORPPayoutChain(ctx context.Context, id, chainTx, status, reason string, now time.Time) (ORPPayout, bool, error) {
	p, err := scanORPPayout(d.Pool.QueryRow(ctx, `
UPDATE orp_payouts SET chain_status=$3, chain_reason=NULLIF($4,''), chain_checked_at=$5,
  state=CASE WHEN $3='verified' THEN 'paid' ELSE state END
WHERE id=$1 AND chain_tx=$2 AND state='sent'
RETURNING `+orpPayoutCols, id, chainTx, status, reason, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPPayout{}, false, nil
	}
	return p, err == nil, err
}

// ListORPChainOpen lists sent chain payouts still waiting on a read, oldest
// check first. A mismatch waits for a new ref and is not listed. A non-empty
// ownerID keeps the payouts that owner pays or receives.
func (d *DB) ListORPChainOpen(ctx context.Context, ownerID string, limit int) ([]ORPPayout, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT `+orpPayoutCols+` FROM orp_payouts
WHERE state='sent' AND chain_tx IS NOT NULL
  AND COALESCE(chain_status,'pending') IN ('pending','unavailable')
  AND ($1='' OR payer_owner::text=$1 OR payee_owner::text=$1)
ORDER BY chain_checked_at NULLS FIRST, created_at
LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ORPPayout
	for rows.Next() {
		p, err := scanORPPayout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetORPPayoutWallet saves the owner's receiving address on one network. An
// empty address removes it.
func (d *DB) SetORPPayoutWallet(ctx context.Context, ownerID, network, address string) error {
	if address == "" {
		_, err := d.Pool.Exec(ctx, `DELETE FROM orp_payout_wallets WHERE owner_id=$1::uuid AND network=$2`, ownerID, network)
		return err
	}
	_, err := d.Pool.Exec(ctx, `
INSERT INTO orp_payout_wallets(owner_id, network, address) VALUES($1::uuid,$2,$3)
ON CONFLICT (owner_id, network) DO UPDATE SET address=EXCLUDED.address, updated_at=now()`, ownerID, network, address)
	return err
}

// ORPPayoutWallets returns the owner's receiving addresses by network.
func (d *DB) ORPPayoutWallets(ctx context.Context, ownerID string) (map[string]string, error) {
	rows, err := d.Pool.Query(ctx, `SELECT network, address FROM orp_payout_wallets WHERE owner_id=$1::uuid`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var network, address string
		if err := rows.Scan(&network, &address); err != nil {
			return nil, err
		}
		out[network] = address
	}
	return out, rows.Err()
}
