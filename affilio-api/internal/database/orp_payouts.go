package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ORPPayout is one payout record for one pay obligation on a 0.3 deal.
type ORPPayout struct {
	ID              string
	DealID          string
	Obligation      string
	BasisObligation string
	PayerOwner      string
	PayerParty      string
	PayeeOwner      string
	PayeeParty      string
	State           string
	AmountMinor     int64
	Currency        string
	Rail            string
	Basis           string
	HoldUntil       *time.Time
	SentAt          *time.Time
	AnsweredAt      *time.Time
	Answer          string
	CreatedAt       time.Time
	// Ref is the rail's transfer reference the payer reported. ChainTx is
	// <caip2>:<hash> on a chain rail, and ChainStatus is the host's last
	// read of it (dev.allagio.settle-chain).
	Ref            string
	ChainTx        string
	ChainStatus    string
	ChainReason    string
	ChainCheckedAt *time.Time
}

func scanORPPayout(row interface {
	Scan(...any) error
}) (ORPPayout, error) {
	var p ORPPayout
	var basisObligation, answer, ref, chainTx, chainStatus, chainReason *string
	err := row.Scan(&p.ID, &p.DealID, &p.Obligation, &basisObligation,
		&p.PayerOwner, &p.PayerParty, &p.PayeeOwner, &p.PayeeParty,
		&p.State, &p.AmountMinor, &p.Currency, &p.Rail, &p.Basis,
		&p.HoldUntil, &p.SentAt, &p.AnsweredAt, &answer, &p.CreatedAt,
		&ref, &chainTx, &chainStatus, &chainReason, &p.ChainCheckedAt)
	for _, pair := range []struct {
		from *string
		to   *string
	}{{ref, &p.Ref}, {chainTx, &p.ChainTx}, {chainStatus, &p.ChainStatus}, {chainReason, &p.ChainReason}} {
		if pair.from != nil {
			*pair.to = *pair.from
		}
	}
	if basisObligation != nil {
		p.BasisObligation = *basisObligation
	}
	if answer != nil {
		p.Answer = *answer
	}
	return p, err
}

const orpPayoutCols = `id, deal_id, obligation, basis_obligation, payer_owner::text,
payer_party, payee_owner::text, payee_party, state, amount_minor, currency,
rail, basis, hold_until, sent_at, answered_at, answer, created_at,
ref, chain_tx, chain_status, chain_reason, chain_checked_at`

// AdvanceORPHolds moves held payouts whose hold has passed to payable and
// returns them so the caller can apply the payer's settle step and notify.
func (d *DB) AdvanceORPHolds(ctx context.Context, now time.Time) ([]ORPPayout, error) {
	rows, err := d.Pool.Query(ctx, `
UPDATE orp_payouts SET state='payable'
WHERE state='held' AND hold_until <= $1
RETURNING `+orpPayoutCols, now)
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

// ORPActiveDealTerms is the payout planner's read of one active deal.
type ORPActiveDealTerms struct {
	ID            string
	OffererOwner  string
	AcceptorOwner string
	OffererParty  string
	AcceptorParty string
	Obligations   []map[string]any
	States        []map[string]any
}

// ListORPActiveDealTerms loads the terms and obligation states of every
// active 0.3 deal. The test pool is small; a bounded scan is honest here.
func (d *DB) ListORPActiveDealTerms(ctx context.Context, limit int) ([]ORPActiveDealTerms, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id, COALESCE(offerer_owner::text,''), COALESCE(acceptor_owner::text,''),
       offerer_party, acceptor_party, terms, obligations
FROM orp_deals
WHERE state='active' AND parties IS NULL
ORDER BY created_at
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ORPActiveDealTerms
	for rows.Next() {
		var deal ORPActiveDealTerms
		var termsRaw, statesRaw []byte
		var offererOwner, acceptorOwner *string
		if err = rows.Scan(&deal.ID, &offererOwner, &acceptorOwner,
			&deal.OffererParty, &deal.AcceptorParty, &termsRaw, &statesRaw); err != nil {
			return nil, err
		}
		if offererOwner != nil {
			deal.OffererOwner = *offererOwner
		}
		if acceptorOwner != nil {
			deal.AcceptorOwner = *acceptorOwner
		}
		var terms struct {
			Obligations []map[string]any `json:"obligations"`
		}
		if err = json.Unmarshal(termsRaw, &terms); err != nil {
			return nil, err
		}
		deal.Obligations = terms.Obligations
		if err = json.Unmarshal(statesRaw, &deal.States); err != nil {
			deal.States = nil
		}
		out = append(out, deal)
	}
	return out, rows.Err()
}

// MintORPPayout stores one planned payout. The dedup index makes a repeat
// plan a no-op, so created is false on replays.
func (d *DB) MintORPPayout(ctx context.Context, p ORPPayout) (created bool, err error) {
	var basisObligation *string
	if p.BasisObligation != "" {
		basisObligation = &p.BasisObligation
	}
	tag, err := d.Pool.Exec(ctx, `
INSERT INTO orp_payouts(id, deal_id, obligation, basis_obligation, payer_owner,
  payer_party, payee_owner, payee_party, state, amount_minor, currency, rail,
  basis, hold_until)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT DO NOTHING`, p.ID, p.DealID, p.Obligation, basisObligation,
		p.PayerOwner, p.PayerParty, p.PayeeOwner, p.PayeeParty, p.State,
		p.AmountMinor, p.Currency, p.Rail, p.Basis, p.HoldUntil)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (d *DB) GetORPPayout(ctx context.Context, id string) (ORPPayout, bool, error) {
	p, err := scanORPPayout(d.Pool.QueryRow(ctx, `SELECT `+orpPayoutCols+` FROM orp_payouts WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPPayout{}, false, nil
	}
	return p, err == nil, err
}

// SettleORPPayout moves a payable payout to sent. The rail is test-mode
// manual: sent records that the host released it, not that money moved.
func (d *DB) SettleORPPayout(ctx context.Context, id string, now time.Time) (ORPPayout, bool, error) {
	p, err := scanORPPayout(d.Pool.QueryRow(ctx, `
UPDATE orp_payouts SET state='sent', sent_at=$2
WHERE id=$1 AND state='payable'
RETURNING `+orpPayoutCols, id, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPPayout{}, false, nil
	}
	return p, err == nil, err
}

// AnswerORPPayout records the payee's answer. confirm moves sent to paid;
// not_received leaves it sent so the payee can still dispute by due.
func (d *DB) AnswerORPPayout(ctx context.Context, id, answer string, now time.Time) (ORPPayout, bool, error) {
	next := "paid"
	if answer == "not_received" {
		next = "sent"
	}
	p, err := scanORPPayout(d.Pool.QueryRow(ctx, `
UPDATE orp_payouts SET state=$2, answered_at=$3, answer=$4
WHERE id=$1 AND state='sent'
RETURNING `+orpPayoutCols, id, next, now, answer))
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPPayout{}, false, nil
	}
	return p, err == nil, err
}

// ORPPayoutsForOwner lists payouts where the owner pays or receives.
func (d *DB) ORPPayoutsForOwner(ctx context.Context, ownerID string) ([]ORPPayout, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT `+orpPayoutCols+` FROM orp_payouts
WHERE payer_owner=$1::uuid OR payee_owner=$1::uuid
ORDER BY created_at DESC
LIMIT 100`, ownerID)
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

// InsertORPInbox writes one host message into a party's inbox outside a
// message transaction: payout records mint by the sweep, not by a reply.
func (d *DB) InsertORPInbox(ctx context.Context, ownerID string, message []byte) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO orp_inbox(owner_id, message) VALUES($1::uuid,$2)`, ownerID, message)
	return err
}
