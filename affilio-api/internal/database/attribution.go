package database

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrClickNotFound      = errors.New("click not found")
	ErrConversionNotFound = errors.New("conversion not found")
)

type MarketplaceOffer struct {
	OfferID            string
	ProductName        string
	ProductTagline     string
	WebsiteURL         string
	PrimaryCategory    string
	Keywords           []string
	Tags               []string
	CommissionType     string
	CommissionValue    float64
	CommissionCurrency string
	CookieDurationDays int
	RefundWindowDays   int
	Terms              string
	ProgramID          *string
	TrackingCode       *string
	CampaignStatus     *string
}

func (d *DB) ListMarketplaceOffers(ctx context.Context, siteID string) ([]MarketplaceOffer, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT o.id, p.name, COALESCE(p.tagline,''), p.website_url, COALESCE(p.primary_category,''),
       COALESCE(p.keywords,'{}'), COALESCE(p.tags,'{}'),
       o.commission_type, o.commission_value, o.commission_currency,
       o.cookie_duration_days, o.refund_window_days, COALESCE(o.terms,''),
       o.program_id, c.tracking_code, c.status
FROM offers o
JOIN products p ON p.id = o.product_id
LEFT JOIN campaigns c ON c.offer_id = o.id AND c.site_id = $1
WHERE o.status='listed'
ORDER BY o.created_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MarketplaceOffer{}
	for rows.Next() {
		var m MarketplaceOffer
		if err := rows.Scan(&m.OfferID, &m.ProductName, &m.ProductTagline, &m.WebsiteURL, &m.PrimaryCategory,
			&m.Keywords, &m.Tags,
			&m.CommissionType, &m.CommissionValue, &m.CommissionCurrency,
			&m.CookieDurationDays, &m.RefundWindowDays, &m.Terms,
			&m.ProgramID, &m.TrackingCode, &m.CampaignStatus); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// JoinOffer creates the campaign for an offer x site pair, or re-activates the
// existing one; the tracking code is stable for the life of the campaign.
func (d *DB) JoinOffer(ctx context.Context, offerID, siteID, trackingCode string) (campaignID, code string, err error) {
	err = d.Pool.QueryRow(ctx, `
INSERT INTO campaigns (offer_id, site_id, tracking_code)
VALUES ($1,$2,$3)
ON CONFLICT (offer_id, site_id) DO UPDATE SET status='active'
RETURNING id, tracking_code`, offerID, siteID, trackingCode).Scan(&campaignID, &code)
	return campaignID, code, err
}

type CampaignLink struct {
	CampaignID     string
	SiteID         string
	OfferID        string
	CampaignStatus string
	OfferStatus    string
	LandingURL     string
	RefParam       string
	ProgramID      *string
}

func (d *DB) GetCampaignByTrackingCode(ctx context.Context, code string) (*CampaignLink, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT c.id, c.site_id, c.offer_id, c.status, o.status, o.landing_url, o.ref_param, o.program_id
FROM campaigns c
JOIN offers o ON o.id = c.offer_id
WHERE c.tracking_code=$1`, code)
	var link CampaignLink
	if err := row.Scan(&link.CampaignID, &link.SiteID, &link.OfferID, &link.CampaignStatus, &link.OfferStatus, &link.LandingURL, &link.RefParam, &link.ProgramID); err != nil {
		return nil, err
	}
	return &link, nil
}

func (d *DB) InsertCampaignClick(ctx context.Context, campaignID, siteID string, programID *string, ipHash, userAgent, referrer string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO clicks (site_id, campaign_id, program_id, ip_hash, user_agent, referrer)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id`, siteID, campaignID, programID, ipHash, userAgent, referrer).Scan(&id)
	return id, err
}

// InsertWebhookEvent stores the raw event for replay and acts as the
// idempotency barrier: a duplicate delivery reports inserted=false.
func (d *DB) InsertWebhookEvent(ctx context.Context, provider, eventID string, payload []byte, signatureValid bool) (bool, error) {
	res, err := d.Pool.Exec(ctx, `
INSERT INTO webhook_events (provider, provider_event_id, payload, signature_valid)
VALUES ($1,$2,$3,$4)
ON CONFLICT (provider, provider_event_id) DO NOTHING`, provider, eventID, payload, signatureValid)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func (d *DB) MarkWebhookProcessed(ctx context.Context, provider, eventID, errText string) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE webhook_events SET processed_at=NOW(), error=NULLIF($3,'')
WHERE provider=$1 AND provider_event_id=$2`, provider, eventID, errText)
	return err
}

type SaleConversion struct {
	ConversionID string
	CampaignID   string
	SiteID       string
	OfferID      string
	Commission   float64
	Flagged      bool
	FlagReason   string
}

const (
	// A purchase this soon after the click is a self-referral tell.
	fraudFastPurchaseWindow = 30 * time.Second
	// More conversions than this per campaign per hour pauses the campaign.
	fraudVelocityCapPerHour = 20
)

// RecordSaleConversion joins a provider sale/renewal back to the click it came
// from, computes the commission from the offer terms, and books a pending
// ledger entry that clears after the offer's refund window. Fraud tells flag
// the conversion — flagged commissions never clear until an admin clears the
// flag — and a velocity spike pauses the campaign.
func (d *DB) RecordSaleConversion(ctx context.Context, clickID, provider, eventID, objectID, kind string, amount float64, currency string, occurredAt time.Time) (*SaleConversion, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out SaleConversion
	var commissionType string
	var commissionValue float64
	var refundWindowDays int
	var clickedAt time.Time
	err = tx.QueryRow(ctx, `
SELECT cam.id, cam.site_id, cam.offer_id, o.commission_type, o.commission_value, o.refund_window_days, cl.clicked_at
FROM clicks cl
JOIN campaigns cam ON cam.id = cl.campaign_id
JOIN offers o ON o.id = cam.offer_id
WHERE cl.id=$1`, clickID).Scan(&out.CampaignID, &out.SiteID, &out.OfferID, &commissionType, &commissionValue, &refundWindowDays, &clickedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClickNotFound
		}
		return nil, err
	}
	out.Commission = commissionFor(commissionType, commissionValue, amount)

	// A purchase before the click is as suspicious as one seconds after it
	// (and provider timestamps truncate to seconds, so tiny negatives happen).
	if interval := occurredAt.Sub(clickedAt); interval < fraudFastPurchaseWindow {
		out.Flagged = true
		out.FlagReason = "implausibly fast click-to-purchase"
	}
	var recentConversions int
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*) FROM conversions
WHERE campaign_id=$1 AND kind IN ('sale','renewal') AND created_at >= NOW() - INTERVAL '1 hour'`, out.CampaignID).Scan(&recentConversions); err != nil {
		return nil, err
	}
	if recentConversions >= fraudVelocityCapPerHour {
		out.Flagged = true
		out.FlagReason = "campaign conversion velocity exceeded; campaign paused"
		if _, err := tx.Exec(ctx, `UPDATE campaigns SET status='paused' WHERE id=$1`, out.CampaignID); err != nil {
			return nil, err
		}
	}

	err = tx.QueryRow(ctx, `
INSERT INTO conversions (offer_id, campaign_id, click_id, provider, provider_event_id, provider_object_id, kind, amount, currency, commission_amount, occurred_at, flagged, flag_reason)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''))
ON CONFLICT (provider, provider_event_id) DO NOTHING
RETURNING id`, out.OfferID, out.CampaignID, clickID, provider, eventID, objectID, kind, amount, currency, out.Commission, occurredAt, out.Flagged, out.FlagReason).Scan(&out.ConversionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate delivery raced past the webhook_events barrier; nothing to book.
			return nil, nil
		}
		return nil, err
	}

	clearsAt := occurredAt.Add(time.Duration(refundWindowDays) * 24 * time.Hour)
	if _, err := tx.Exec(ctx, `
INSERT INTO ledger_entries (site_id, conversion_id, kind, amount, currency, status, clears_at)
VALUES ($1,$2,'commission',$3,$4,'pending',$5)`, out.SiteID, out.ConversionID, out.Commission, currency, clearsAt); err != nil {
		return nil, err
	}
	return &out, tx.Commit(ctx)
}

// RecordRefund finds the sale a refund belongs to via the provider object id,
// records the refund conversion, and unwinds the commission: a still-pending
// ledger entry is reversed in place, a cleared one gets a clawback entry.
func (d *DB) RecordRefund(ctx context.Context, provider, eventID, objectID string, occurredAt time.Time) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var saleID, campaignID, siteID, offerID, currency string
	var clickID *string
	var amount, commission float64
	err = tx.QueryRow(ctx, `
SELECT cv.id, cv.campaign_id, cam.site_id, cv.offer_id, cv.click_id, cv.amount, COALESCE(cv.commission_amount,0), cv.currency
FROM conversions cv
JOIN campaigns cam ON cam.id = cv.campaign_id
WHERE cv.provider=$1 AND cv.provider_object_id=$2 AND cv.kind IN ('sale','renewal')
ORDER BY cv.created_at DESC
LIMIT 1`, provider, objectID).Scan(&saleID, &campaignID, &siteID, &offerID, &clickID, &amount, &commission, &currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrConversionNotFound
		}
		return "", err
	}

	var refundID string
	err = tx.QueryRow(ctx, `
INSERT INTO conversions (offer_id, campaign_id, click_id, provider, provider_event_id, provider_object_id, kind, amount, currency, commission_amount, occurred_at)
VALUES ($1,$2,$3,$4,$5,$6,'refund',$7,$8,$9,$10)
ON CONFLICT (provider, provider_event_id) DO NOTHING
RETURNING id`, offerID, campaignID, clickID, provider, eventID, objectID, amount, currency, commission, occurredAt).Scan(&refundID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	var entryStatus string
	err = tx.QueryRow(ctx, `
SELECT status FROM ledger_entries
WHERE conversion_id=$1 AND kind='commission'
ORDER BY created_at DESC LIMIT 1`, saleID).Scan(&entryStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	switch entryStatus {
	case "pending":
		if _, err := tx.Exec(ctx, `
UPDATE ledger_entries SET status='reversed'
WHERE conversion_id=$1 AND kind='commission' AND status='pending'`, saleID); err != nil {
			return "", err
		}
	case "cleared":
		if _, err := tx.Exec(ctx, `
INSERT INTO ledger_entries (site_id, conversion_id, kind, amount, currency, status, clears_at)
VALUES ($1,$2,'clawback',$3,$4,'cleared',NOW())`, siteID, refundID, -commission, currency); err != nil {
			return "", err
		}
	}
	return refundID, tx.Commit(ctx)
}

type FlaggedConversion struct {
	ID          string
	CampaignID  string
	SiteID      string
	OfferID     string
	Provider    string
	Kind        string
	Amount      float64
	Currency    string
	Commission  float64
	FlagReason  string
	OccurredAt  time.Time
	SiteDomain  string
	ProductName string
}

func (d *DB) ListFlaggedConversions(ctx context.Context, limit int) ([]FlaggedConversion, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT cv.id, cv.campaign_id, cam.site_id, cv.offer_id, cv.provider, cv.kind,
       cv.amount, cv.currency, COALESCE(cv.commission_amount,0), COALESCE(cv.flag_reason,''), cv.occurred_at,
       s.domain, p.name
FROM conversions cv
JOIN campaigns cam ON cam.id = cv.campaign_id
JOIN sites s ON s.id = cam.site_id
JOIN offers o ON o.id = cv.offer_id
JOIN products p ON p.id = o.product_id
WHERE cv.flagged
ORDER BY cv.created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FlaggedConversion{}
	for rows.Next() {
		var f FlaggedConversion
		if err := rows.Scan(&f.ID, &f.CampaignID, &f.SiteID, &f.OfferID, &f.Provider, &f.Kind,
			&f.Amount, &f.Currency, &f.Commission, &f.FlagReason, &f.OccurredAt, &f.SiteDomain, &f.ProductName); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ClearConversionFlag releases a flagged conversion: the commission resumes the
// normal hold-and-clear lifecycle.
func (d *DB) ClearConversionFlag(ctx context.Context, conversionID string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `
UPDATE conversions SET flagged=false, flag_reason=NULL WHERE id=$1 AND flagged`, conversionID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func commissionFor(commissionType string, commissionValue, amount float64) float64 {
	var commission float64
	switch commissionType {
	case "flat":
		commission = commissionValue
	default: // percentage, recurring
		commission = amount * commissionValue / 100
	}
	return math.Round(commission*100) / 100
}
