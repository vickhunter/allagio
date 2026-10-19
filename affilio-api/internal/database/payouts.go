package database

import (
	"context"
	"errors"
	"strings"
	"time"
)

// clearMaturedSQL moves pending commissions whose refund window has passed to
// cleared — except entries whose conversion carries a fraud flag: those stay
// pending until an admin clears the flag. clears_at is timestamptz and is
// compared with NOW().
const clearMaturedSQL = `
UPDATE ledger_entries le SET status='cleared'
WHERE le.status='pending' AND le.clears_at <= NOW()
  AND NOT EXISTS (SELECT 1 FROM conversions cv JOIN conversions source ON source.provider=cv.provider AND source.provider_object_id=cv.provider_object_id AND source.offer_id=cv.offer_id WHERE cv.id=le.conversion_id AND source.flagged)`

// ClearMaturedLedgerEntries runs cheaply before balance reads and payout batches.
func (d *DB) ClearMaturedLedgerEntries(ctx context.Context) (int64, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMoney(ctx, tx); err != nil {
		return 0, err
	}
	res, err := tx.Exec(ctx, clearMaturedSQL)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), tx.Commit(ctx)
}

type SiteBalance struct {
	Currency        string
	Mode            string
	Pending         float64
	ClearedUnpaid   float64
	Paid            float64
	Reserved        float64
	PaymentReported float64
}

func (d *DB) SiteBalances(ctx context.Context, siteID string) ([]SiteBalance, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT le.currency,cv.payment_mode,
 COALESCE(SUM(le.amount) FILTER (WHERE le.status='pending'),0),
 COALESCE(SUM(le.amount) FILTER (WHERE le.status='cleared' AND le.payout_id IS NULL),0),
 COALESCE(SUM(le.amount) FILTER (WHERE le.status='cleared' AND p.status='paid' AND p.received_at IS NOT NULL),0),
 COALESCE(SUM(le.amount) FILTER (WHERE le.status='cleared' AND p.status='pending'),0),
 COALESCE(SUM(le.amount) FILTER (WHERE le.status='cleared' AND p.status='paid' AND p.received_at IS NULL),0)
 FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id LEFT JOIN payouts p ON p.id=le.payout_id
 WHERE le.site_id=$1 GROUP BY le.currency,cv.payment_mode ORDER BY le.currency,cv.payment_mode`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteBalance{}
	for rows.Next() {
		var b SiteBalance
		if err := rows.Scan(&b.Currency, &b.Mode, &b.Pending, &b.ClearedUnpaid, &b.Paid, &b.Reserved, &b.PaymentReported); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type LedgerHistoryEntry struct {
	Mode         string
	ID           string
	Kind         string
	Amount       float64
	Currency     string
	Status       string
	ProductName  string
	ClearsAt     *time.Time
	CreatedAt    time.Time
	TrustLevel   string
	RefundStatus *string
	RefundReason string
}

func (d *DB) ListLedgerHistory(ctx context.Context, siteID string, limit int) ([]LedgerHistoryEntry, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT le.id, le.kind, le.amount, le.currency, le.status,
       COALESCE(p.name, ''), le.clears_at, (le.created_at `+sessionWallAsInstant+`),COALESCE(cv.payment_mode,'legacy'),
       COALESCE(cv.trust_level,''), cv.refund_status, COALESCE(cv.refund_reason,'')
FROM ledger_entries le
LEFT JOIN conversions cv ON cv.id = le.conversion_id
LEFT JOIN offers o ON o.id = cv.offer_id
LEFT JOIN products p ON p.id = o.product_id
WHERE le.site_id=$1
ORDER BY le.created_at DESC
LIMIT $2`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerHistoryEntry{}
	for rows.Next() {
		var e LedgerHistoryEntry
		if err := rows.Scan(&e.ID, &e.Kind, &e.Amount, &e.Currency, &e.Status, &e.ProductName, &e.ClearsAt, &e.CreatedAt, &e.Mode, &e.TrustLevel, &e.RefundStatus, &e.RefundReason); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type Payout struct {
	ID                string
	SiteID            string
	Amount            float64
	Currency          string
	Method            string
	ProviderReference string
	Status            string
	CreatedAt         time.Time
	PaidAt            *time.Time
	MakerID           *string
	Mode              string
	ReceivedAt        *time.Time
}

func (d *DB) ListPayouts(ctx context.Context, siteID string) ([]Payout, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, site_id, amount, currency, COALESCE(method,''), COALESCE(provider_reference,''), status, (created_at `+sessionWallAsInstant+`), (paid_at `+sessionWallAsInstant+`),maker_id,payment_mode,received_at
FROM payouts
WHERE site_id=$1
ORDER BY created_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payout{}
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.SiteID, &p.Amount, &p.Currency, &p.Method, &p.ProviderReference, &p.Status, &p.CreatedAt, &p.PaidAt, &p.MakerID, &p.Mode, &p.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RunPayoutBatch groups each site's cleared-unpaid balance by currency and,
// where it meets the minimum, creates a pending manual payout and stamps the
// entries it covers so they can never pay out twice.
func (d *DB) RunPayoutBatch(ctx context.Context, minPayout float64, includeTest bool) ([]Payout, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockMoney(ctx, tx); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, clearMaturedSQL); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT le.site_id, le.currency, p.maker_id,cv.payment_mode, SUM(le.amount)
FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id
JOIN offers o ON o.id=cv.offer_id JOIN products p ON p.id=o.product_id
WHERE le.status='cleared' AND le.payout_id IS NULL AND le.site_id IS NOT NULL AND ($2 OR cv.payment_mode<>'test')
AND NOT EXISTS (SELECT 1 FROM sites demo_site WHERE demo_site.id = le.site_id AND demo_site.demo)
AND NOT EXISTS (SELECT 1 FROM clicks cl WHERE cl.id = cv.click_id AND cl.click_class = 'test')
GROUP BY le.site_id, le.currency,p.maker_id,cv.payment_mode
HAVING SUM(le.amount) >= $1`, minPayout, includeTest)
	if err != nil {
		return nil, err
	}
	type due struct {
		siteID, currency, makerID, mode string
		amount                          float64
	}
	dues := []due{}
	for rows.Next() {
		var item due
		if err := rows.Scan(&item.siteID, &item.currency, &item.makerID, &item.mode, &item.amount); err != nil {
			rows.Close()
			return nil, err
		}
		dues = append(dues, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	payouts := []Payout{}
	for _, item := range dues {
		mode, err := PayoutMode(ctx, tx, item.siteID)
		if err != nil {
			return nil, err
		}
		if mode == ModeOff {
			continue
		}
		if mode == ModeAsk {
			if err = QueuePayoutBatch(ctx, tx, item.siteID, item.currency, item.makerID, item.mode, item.amount); err != nil {
				return nil, err
			}
			continue
		}
		var payout Payout
		err = tx.QueryRow(ctx, `
INSERT INTO payouts (site_id, amount, currency, method, status,maker_id,payment_mode)
VALUES ($1,$2,$3,'manual','pending',$4,$5)
RETURNING id, site_id, amount, currency, COALESCE(method,''), COALESCE(provider_reference,''), status, (created_at `+sessionWallAsInstant+`), (paid_at `+sessionWallAsInstant+`),maker_id,payment_mode,received_at`,
			item.siteID, item.amount, item.currency, item.makerID, item.mode).Scan(&payout.ID, &payout.SiteID, &payout.Amount, &payout.Currency, &payout.Method, &payout.ProviderReference, &payout.Status, &payout.CreatedAt, &payout.PaidAt, &payout.MakerID, &payout.Mode, &payout.ReceivedAt)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE ledger_entries SET payout_id=$1
WHERE site_id=$2 AND currency=$3 AND status='cleared' AND payout_id IS NULL
AND NOT EXISTS (SELECT 1 FROM sites demo_site WHERE demo_site.id = ledger_entries.site_id AND demo_site.demo)
AND conversion_id IN (
  SELECT cv.id FROM conversions cv
  JOIN offers o ON o.id=cv.offer_id
  JOIN products p ON p.id=o.product_id
  WHERE p.maker_id=$4 AND cv.payment_mode=$5
  AND NOT EXISTS (SELECT 1 FROM clicks cl WHERE cl.id = cv.click_id AND cl.click_class = 'test')
)`, payout.ID, item.siteID, item.currency, item.makerID, item.mode); err != nil {
			return nil, err
		}
		payouts = append(payouts, payout)
	}
	return payouts, tx.Commit(ctx)
}

func (d *DB) MarkPayoutPaid(ctx context.Context, payoutID, providerReference string) (bool, error) {
	if strings.TrimSpace(providerReference) == "" {
		return false, errors.New("payment reference required")
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMoney(ctx, tx); err != nil {
		return false, err
	}
	res, err := tx.Exec(ctx, `UPDATE payouts SET status='paid',paid_at=NOW(),provider_reference=$2 WHERE id=$1 AND status='pending'`, payoutID, providerReference)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, tx.Commit(ctx)
}

func (d *DB) ConfirmPayoutReceived(ctx context.Context, siteID, payoutID string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE payouts SET received_at=NOW() WHERE id=$1 AND site_id=$2 AND status='paid' AND received_at IS NULL`, payoutID, siteID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

type OfferEarnings struct {
	OfferID     string
	ProductName string
	Status      string
	Clicks      int64
	Sales       int64
	Refunds     int64
	Revenue     float64
	Commission  float64
	Currency    string
	Mode        string
}

// MakerEarnings aggregates the demand side's activity per offer so the maker
// sees clicks, conversions, gross revenue, and the commission they owe.
func (d *DB) MakerEarnings(ctx context.Context, makerID, offerID string) ([]OfferEarnings, error) {
	query := `
SELECT o.id, p.name, o.status, o.commission_currency,COALESCE(cv.payment_mode,'none'),
       COALESCE(cl.clicks, 0),
       COALESCE(cv.sales, 0), COALESCE(cv.refunds, 0),
       COALESCE(cv.revenue, 0), COALESCE(cv.commission, 0)
FROM offers o
JOIN products p ON p.id = o.product_id
LEFT JOIN (
    SELECT cam.offer_id, COUNT(*) AS clicks
    FROM clicks c JOIN campaigns cam ON cam.id = c.campaign_id
    WHERE c.click_class <> 'test'
      AND NOT EXISTS (SELECT 1 FROM sites demo_site WHERE demo_site.id = c.site_id AND demo_site.demo)
    GROUP BY cam.offer_id
) cl ON cl.offer_id = o.id
LEFT JOIN (
    SELECT offer_id,payment_mode,
           COUNT(*) FILTER (WHERE kind IN ('sale','renewal')) AS sales,
           COUNT(*) FILTER (WHERE kind='refund' AND COALESCE(refund_status,'applied')='applied') AS refunds,
           COALESCE(SUM(amount) FILTER (WHERE kind IN ('sale','renewal')), 0)
             - COALESCE(SUM(amount) FILTER (WHERE kind='refund' AND COALESCE(refund_status,'applied')='applied'), 0) AS revenue,
           COALESCE(SUM(commission_amount) FILTER (WHERE kind IN ('sale','renewal')), 0)
             - COALESCE(SUM(commission_amount) FILTER (WHERE kind='refund' AND COALESCE(refund_status,'applied')='applied'), 0) AS commission
    FROM conversions
    GROUP BY offer_id,payment_mode
) cv ON cv.offer_id = o.id
WHERE p.maker_id=$1`
	args := []any{makerID}
	if offerID != "" {
		query += ` AND o.id=$2`
		args = append(args, offerID)
	}
	query += ` ORDER BY o.created_at DESC`
	rows, err := d.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OfferEarnings{}
	for rows.Next() {
		var e OfferEarnings
		if err := rows.Scan(&e.OfferID, &e.ProductName, &e.Status, &e.Currency, &e.Mode, &e.Clicks, &e.Sales, &e.Refunds, &e.Revenue, &e.Commission); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
