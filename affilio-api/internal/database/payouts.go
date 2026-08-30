package database

import (
	"context"
	"time"
)

// ClearMaturedLedgerEntries moves pending commissions whose refund window has
// passed to cleared. Runs cheaply before balance reads and payout batches.
func (d *DB) ClearMaturedLedgerEntries(ctx context.Context) (int64, error) {
	res, err := d.Pool.Exec(ctx, `
UPDATE ledger_entries SET status='cleared'
WHERE status='pending' AND clears_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

type SiteBalance struct {
	Currency      string
	Pending       float64
	ClearedUnpaid float64
	Paid          float64
}

func (d *DB) SiteBalances(ctx context.Context, siteID string) ([]SiteBalance, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT currency,
       COALESCE(SUM(amount) FILTER (WHERE status='pending'), 0),
       COALESCE(SUM(amount) FILTER (WHERE status='cleared' AND payout_id IS NULL), 0),
       COALESCE(SUM(amount) FILTER (WHERE status='cleared' AND payout_id IS NOT NULL), 0)
FROM ledger_entries
WHERE site_id=$1
GROUP BY currency
ORDER BY currency`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteBalance{}
	for rows.Next() {
		var b SiteBalance
		if err := rows.Scan(&b.Currency, &b.Pending, &b.ClearedUnpaid, &b.Paid); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type LedgerHistoryEntry struct {
	ID          string
	Kind        string
	Amount      float64
	Currency    string
	Status      string
	ProductName string
	ClearsAt    *time.Time
	CreatedAt   time.Time
}

func (d *DB) ListLedgerHistory(ctx context.Context, siteID string, limit int) ([]LedgerHistoryEntry, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT le.id, le.kind, le.amount, le.currency, le.status,
       COALESCE(p.name, ''), le.clears_at, le.created_at
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
		if err := rows.Scan(&e.ID, &e.Kind, &e.Amount, &e.Currency, &e.Status, &e.ProductName, &e.ClearsAt, &e.CreatedAt); err != nil {
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
}

func (d *DB) ListPayouts(ctx context.Context, siteID string) ([]Payout, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, site_id, amount, currency, COALESCE(method,''), COALESCE(provider_reference,''), status, created_at, paid_at
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
		if err := rows.Scan(&p.ID, &p.SiteID, &p.Amount, &p.Currency, &p.Method, &p.ProviderReference, &p.Status, &p.CreatedAt, &p.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RunPayoutBatch groups each site's cleared-unpaid balance by currency and,
// where it meets the minimum, creates a pending manual payout and stamps the
// entries it covers so they can never pay out twice.
func (d *DB) RunPayoutBatch(ctx context.Context, minPayout float64) ([]Payout, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
UPDATE ledger_entries SET status='cleared'
WHERE status='pending' AND clears_at <= NOW()`); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT site_id, currency, SUM(amount)
FROM ledger_entries
WHERE status='cleared' AND payout_id IS NULL AND site_id IS NOT NULL
GROUP BY site_id, currency
HAVING SUM(amount) >= $1`, minPayout)
	if err != nil {
		return nil, err
	}
	type due struct {
		siteID, currency string
		amount           float64
	}
	dues := []due{}
	for rows.Next() {
		var item due
		if err := rows.Scan(&item.siteID, &item.currency, &item.amount); err != nil {
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
		var payout Payout
		err := tx.QueryRow(ctx, `
INSERT INTO payouts (site_id, amount, currency, method, status)
VALUES ($1,$2,$3,'manual','pending')
RETURNING id, site_id, amount, currency, COALESCE(method,''), COALESCE(provider_reference,''), status, created_at, paid_at`,
			item.siteID, item.amount, item.currency).Scan(&payout.ID, &payout.SiteID, &payout.Amount, &payout.Currency, &payout.Method, &payout.ProviderReference, &payout.Status, &payout.CreatedAt, &payout.PaidAt)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE ledger_entries SET payout_id=$1
WHERE site_id=$2 AND currency=$3 AND status='cleared' AND payout_id IS NULL`, payout.ID, item.siteID, item.currency); err != nil {
			return nil, err
		}
		payouts = append(payouts, payout)
	}
	return payouts, tx.Commit(ctx)
}

func (d *DB) MarkPayoutPaid(ctx context.Context, payoutID, providerReference string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `
UPDATE payouts SET status='paid', paid_at=NOW(), provider_reference=NULLIF($2,'')
WHERE id=$1 AND status='pending'`, payoutID, providerReference)
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
}

// MakerEarnings aggregates the demand side's activity per offer so the maker
// sees clicks, conversions, gross revenue, and the commission they owe.
func (d *DB) MakerEarnings(ctx context.Context, makerID string) ([]OfferEarnings, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT o.id, p.name, o.status, o.commission_currency,
       COALESCE(cl.clicks, 0),
       COALESCE(cv.sales, 0), COALESCE(cv.refunds, 0),
       COALESCE(cv.revenue, 0), COALESCE(cv.commission, 0)
FROM offers o
JOIN products p ON p.id = o.product_id
LEFT JOIN (
    SELECT cam.offer_id, COUNT(*) AS clicks
    FROM clicks c JOIN campaigns cam ON cam.id = c.campaign_id
    GROUP BY cam.offer_id
) cl ON cl.offer_id = o.id
LEFT JOIN (
    SELECT offer_id,
           COUNT(*) FILTER (WHERE kind IN ('sale','renewal')) AS sales,
           COUNT(*) FILTER (WHERE kind='refund') AS refunds,
           COALESCE(SUM(amount) FILTER (WHERE kind IN ('sale','renewal')), 0)
             - COALESCE(SUM(amount) FILTER (WHERE kind='refund'), 0) AS revenue,
           COALESCE(SUM(commission_amount) FILTER (WHERE kind IN ('sale','renewal')), 0)
             - COALESCE(SUM(commission_amount) FILTER (WHERE kind='refund'), 0) AS commission
    FROM conversions
    GROUP BY offer_id
) cv ON cv.offer_id = o.id
WHERE p.maker_id=$1
ORDER BY o.created_at DESC`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OfferEarnings{}
	for rows.Next() {
		var e OfferEarnings
		if err := rows.Scan(&e.OfferID, &e.ProductName, &e.Status, &e.Currency, &e.Clicks, &e.Sales, &e.Refunds, &e.Revenue, &e.Commission); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
