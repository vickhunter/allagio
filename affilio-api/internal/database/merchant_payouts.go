package database

import (
	"context"
	"strings"
)

type MerchantPayout struct {
	ID           string  `json:"id"`
	Domain       string  `json:"domain"`
	Instructions string  `json:"instructions"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Mode         string  `json:"mode"`
	Status       string  `json:"status"`
	Reference    string  `json:"reference"`
	Received     bool    `json:"received"`
}

func (d *DB) MerchantPayouts(ctx context.Context, makerID string) ([]MerchantPayout, error) {
	rows, err := d.Pool.Query(ctx, `SELECT p.id,s.domain,s.payout_instructions,p.amount,p.currency,p.payment_mode,p.status,COALESCE(p.provider_reference,''),p.received_at IS NOT NULL FROM payouts p JOIN sites s ON s.id=p.site_id WHERE p.maker_id=$1 ORDER BY p.created_at DESC LIMIT 100`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MerchantPayout{}
	for rows.Next() {
		var p MerchantPayout
		if err = rows.Scan(&p.ID, &p.Domain, &p.Instructions, &p.Amount, &p.Currency, &p.Mode, &p.Status, &p.Reference, &p.Received); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) ReportMerchantPayment(ctx context.Context, makerID, payoutID, reference string) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err = lockMoney(ctx, tx); err != nil {
		return false, err
	}
	res, err := tx.Exec(ctx, `UPDATE payouts SET status='paid',paid_at=NOW(),provider_reference=$3 WHERE id=$1 AND maker_id=$2 AND status='pending' AND payment_mode='live'`, payoutID, makerID, strings.TrimSpace(reference))
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, tx.Commit(ctx)
}

func (d *DB) PayoutInstructions(ctx context.Context, siteID string) (string, error) {
	var instructions string
	err := d.Pool.QueryRow(ctx, `SELECT payout_instructions FROM sites WHERE id=$1`, siteID).Scan(&instructions)
	return instructions, err
}

func (d *DB) SavePayoutInstructions(ctx context.Context, siteID, instructions string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE sites SET payout_instructions=$2 WHERE id=$1`, siteID, strings.TrimSpace(instructions))
	return err
}
