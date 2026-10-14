package database

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
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
	CampaignID        string
	SiteID            string
	OfferID           string
	CampaignStatus    string
	OfferStatus       string
	LandingURL        string
	RefParam          string
	ProgramID         *string
	RequiresPlacement bool
	SiteDomain        string
}

// PlacementRedirectSQL is the placement go-link lookup. CountsAsLive is the
// shared placement_counts_as_live predicate. A hit is recorded only when it
// is true.
const PlacementRedirectSQL = `SELECT pp.site_id::text, s.domain, pp.post_url,
       (o.status = 'listed'), o.landing_url, COALESCE(NULLIF(o.ref_param,''),'via'),
       pp.campaign_id::text, o.program_id,
       EXISTS (SELECT 1 FROM placement_counts_as_live live WHERE live.id = pp.id)
FROM placement_proposals pp
JOIN sites s ON s.id = pp.site_id
JOIN offers o ON o.id = pp.offer_id
WHERE pp.id = $1`

// PlacementRedirect is everything GET /v1/go/<placement_id> needs to choose
// a 302 target. Found is false when the id is unknown.
type PlacementRedirect struct {
	Found        bool
	SiteID       string
	SiteDomain   string
	PostURL      string
	OfferListed  bool
	LandingURL   string
	RefParam     string
	CampaignID   string
	ProgramID    *string
	CountsAsLive bool
}

func (d *DB) LookupPlacementRedirect(ctx context.Context, placementID string) (PlacementRedirect, error) {
	var out PlacementRedirect
	var campaignID *string
	err := d.Pool.QueryRow(ctx, PlacementRedirectSQL, placementID).Scan(
		&out.SiteID, &out.SiteDomain, &out.PostURL, &out.OfferListed, &out.LandingURL, &out.RefParam,
		&campaignID, &out.ProgramID, &out.CountsAsLive)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlacementRedirect{}, nil
	}
	if err != nil {
		return PlacementRedirect{}, err
	}
	out.Found = true
	if campaignID != nil {
		out.CampaignID = *campaignID
	}
	return out, nil
}

// RecentPlacementClick returns a hit from the same reader hash and user agent
// on this site and placement inside the dedup window. An empty hash is not a
// reader, so it never matches. An empty id means there is no recent hit.
func (d *DB) RecentPlacementClick(ctx context.Context, siteID, placementID, ipHash, userAgent, clickClass string, withinSeconds int) (string, error) {
	if ipHash == "" {
		return "", nil
	}
	if clickClass != "verified_agent" && clickClass != "declared_agent" && clickClass != "browser" && clickClass != "unknown" {
		clickClass = "unknown"
	}
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT id::text FROM clicks
WHERE site_id=$1 AND placement_id=$2 AND ip_hash=$3 AND COALESCE(user_agent,'')=$4 AND click_class=$6
AND clicked_at >= NOW() - ($5 * INTERVAL '1 second')
ORDER BY clicked_at DESC LIMIT 1`, siteID, placementID, ipHash, userAgent, withinSeconds, clickClass).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// SiteForGoKey resolves a publisher key to the site it may speak for.
// ok is false when the key is missing or does not identify a site.
func (d *DB) SiteForGoKey(ctx context.Context, keyHash, keyPrefix string) (siteID, domain string, ok bool, err error) {
	site, err := d.GetSiteByAnyKeyHash(ctx, keyHash, keyPrefix)
	if err == nil {
		return site.ID, site.Domain, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, err
	}
	siteID, err = d.ProjectSiteKey(ctx, keyHash)
	if err == nil {
		site, err = d.GetSiteByID(ctx, siteID)
		if err != nil {
			return "", "", false, err
		}
		return site.ID, site.Domain, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, err
	}
	account, err := d.ActiveAccountKey(ctx, keyHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && account.SiteID == "") {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	site, err = d.GetSiteByID(ctx, account.SiteID)
	if err != nil {
		return "", "", false, err
	}
	return site.ID, site.Domain, true, nil
}

func (d *DB) GetCampaignByTrackingCode(ctx context.Context, code string) (*CampaignLink, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT c.id, c.site_id, c.offer_id, c.status, o.status, o.landing_url, o.ref_param, o.program_id, c.requires_placement, s.domain
FROM campaigns c
JOIN offers o ON o.id = c.offer_id
JOIN sites s ON s.id = c.site_id
WHERE c.tracking_code=$1`, code)
	var link CampaignLink
	if err := row.Scan(&link.CampaignID, &link.SiteID, &link.OfferID, &link.CampaignStatus, &link.OfferStatus, &link.LandingURL, &link.RefParam, &link.ProgramID, &link.RequiresPlacement, &link.SiteDomain); err != nil {
		return nil, err
	}
	return &link, nil
}

func (d *DB) InsertCampaignClick(ctx context.Context, campaignID, siteID string, programID *string, ipHash, userAgent, referrer string) (string, error) {
	return d.InsertPlacementClick(ctx, campaignID, siteID, programID, ipHash, userAgent, referrer, "", "")
}

// LivePlacementForCampaign returns the placement id only when exactly one
// placement on this campaign is in placement_counts_as_live. Zero or two or
// more return an empty id so the bare code redirects with no hit.
func (d *DB) LivePlacementForCampaign(ctx context.Context, campaignID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT COALESCE(CASE WHEN COUNT(*) = 1 THEN MIN(pp.id::text) ELSE '' END, '')
FROM placement_proposals pp
WHERE pp.campaign_id=$1
  AND EXISTS (SELECT 1 FROM placement_counts_as_live live WHERE live.id = pp.id)`, campaignID).Scan(&id)
	return id, err
}

// PurgeClickSignals clears reader signals older than the cutoff.
func (d *DB) PurgeClickSignals(ctx context.Context, cutoff time.Time) error {
	_, err := d.Pool.Exec(ctx, `UPDATE clicks
SET ip_hash=NULL, user_agent=NULL, referrer=NULL
WHERE clicked_at < $1
  AND (ip_hash IS NOT NULL OR user_agent IS NOT NULL OR referrer IS NOT NULL)`, cutoff)
	return err
}

func (d *DB) InsertPlacementClick(ctx context.Context, campaignID, siteID string, programID *string, ipHash, userAgent, referrer, placementID, clickClass string) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var snapshot []byte
	if placementID != "" {
		var status string
		err = tx.QueryRow(ctx, `SELECT status,terms_snapshot FROM placement_proposals WHERE id=$1 AND campaign_id=$2 FOR SHARE`, placementID, campaignID).Scan(&status, &snapshot)
		if err != nil {
			return "", err
		}
		if status != "committed" {
			return "", ErrPlacementAuthority
		}
	}
	var id string
	if clickClass != "verified_agent" && clickClass != "declared_agent" && clickClass != "browser" && clickClass != "unknown" {
		clickClass = "unknown"
	}
	err = tx.QueryRow(ctx, `INSERT INTO clicks(site_id,campaign_id,program_id,ip_hash,user_agent,referrer,terms_snapshot,placement_id,click_class)
 SELECT $1,$2,$3,$4,$5,$6,COALESCE($7::jsonb,jsonb_build_object(
 'product_id',o.product_id,'commission_type',o.commission_type,'commission_value_minor',(o.commission_value*100)::bigint,
 'currency',o.commission_currency,'cookie_duration_days',o.cookie_duration_days,'refund_window_days',o.refund_window_days)),NULLIF($8,'')::uuid,$9
 FROM campaigns cam JOIN offers o ON o.id=cam.offer_id WHERE cam.id=$2 RETURNING id`,
		siteID, campaignID, programID, ipHash, userAgent, referrer, snapshot, placementID, clickClass).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
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
	return d.recordSale(ctx, "", clickID, provider, eventID, objectID, kind, amount, 0, 0, currency, "legacy", occurredAt)
}

func (d *DB) RecordStripeSale(ctx context.Context, productID, clickID, eventID, objectID string, amountMinor, grossMinor int64, currency string, liveMode bool, occurredAt time.Time) (*SaleConversion, error) {
	mode := "test"
	if liveMode {
		mode = "live"
	}
	return d.recordSale(ctx, productID, clickID, "stripe", eventID, objectID, "sale", float64(amountMinor)/100, amountMinor, grossMinor, currency, mode, occurredAt)
}

func lockMoney(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4150110)`)
	return err
}

func (d *DB) recordSale(ctx context.Context, productID, clickID, provider, eventID, objectID, kind string, amount float64, amountMinor, grossMinor int64, currency, paymentMode string, occurredAt time.Time) (*SaleConversion, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockMoney(ctx, tx); err != nil {
		return nil, err
	}
	if provider == "stripe" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversions WHERE provider='stripe' AND provider_object_id=$1 AND kind='sale')`, objectID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			return nil, nil
		}
	}
	var out SaleConversion
	var commissionType string
	var commissionValueMinor int64
	var refundWindowDays int
	var clickedAt time.Time
	err = tx.QueryRow(ctx, `
SELECT cam.id, cam.site_id, cam.offer_id,
 COALESCE(cl.terms_snapshot->>'commission_type',o.commission_type),
 COALESCE((cl.terms_snapshot->>'commission_value_minor')::bigint,(o.commission_value*100)::bigint),
 COALESCE((cl.terms_snapshot->>'refund_window_days')::int,o.refund_window_days), cl.clicked_at
FROM clicks cl
JOIN campaigns cam ON cam.id = cl.campaign_id
JOIN offers o ON o.id = cam.offer_id
WHERE cl.id=$1 AND ($2='' OR o.product_id::text=$2)`, clickID, productID).Scan(&out.CampaignID, &out.SiteID, &out.OfferID, &commissionType, &commissionValueMinor, &refundWindowDays, &clickedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClickNotFound
		}
		return nil, err
	}
	out.Commission = commissionFor(commissionType, float64(commissionValueMinor)/100, amount)
	if productID != "" {
		commissionMinor := commissionValueMinor
		if commissionType != "flat" {
			commissionMinor = (amountMinor*commissionValueMinor + 5000) / 10000
		}
		if commissionMinor < 0 || commissionMinor > amountMinor {
			return nil, fmt.Errorf("commission exceeds eligible revenue")
		}
		out.Commission = float64(commissionMinor) / 100
	}

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
INSERT INTO conversions (offer_id, campaign_id, click_id, provider, provider_event_id, provider_object_id, kind, amount, currency, commission_amount, occurred_at, flagged, flag_reason, gross_minor,payment_mode)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),NULLIF($14,0),$15)
ON CONFLICT (provider, provider_event_id) DO NOTHING
RETURNING id`, out.OfferID, out.CampaignID, clickID, provider, eventID, objectID, kind, amount, currency, out.Commission, occurredAt, out.Flagged, out.FlagReason, grossMinor, paymentMode).Scan(&out.ConversionID)
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

func (d *DB) RecordRefund(ctx context.Context, provider, eventID, objectID string, occurredAt time.Time) (string, error) {
	return d.RecordPartialRefund(ctx, provider, eventID, objectID, -1, occurredAt)
}

func (d *DB) RecordPartialRefund(ctx context.Context, provider, eventID, objectID string, cumulativeMinor int64, occurredAt time.Time) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMoney(ctx, tx); err != nil {
		return "", err
	}
	var saleID, campaignID, siteID, offerID, currency, paymentMode string
	var clickID *string
	var netMinor, commissionMinor, grossMinor int64
	err = tx.QueryRow(ctx, `SELECT cv.id,cv.campaign_id,cam.site_id,cv.offer_id,cv.click_id,
 (cv.amount*100)::bigint,(COALESCE(cv.commission_amount,0)*100)::bigint,cv.currency,COALESCE(cv.gross_minor,(cv.amount*100)::bigint),cv.payment_mode
 FROM conversions cv JOIN campaigns cam ON cam.id=cv.campaign_id
 WHERE cv.provider=$1 AND cv.provider_object_id=$2 AND cv.kind IN ('sale','renewal') ORDER BY cv.created_at DESC LIMIT 1`, provider, objectID).
		Scan(&saleID, &campaignID, &siteID, &offerID, &clickID, &netMinor, &commissionMinor, &currency, &grossMinor, &paymentMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrConversionNotFound
	}
	if err != nil {
		return "", err
	}
	if cumulativeMinor == -1 {
		cumulativeMinor = grossMinor
	}
	if cumulativeMinor <= 0 || cumulativeMinor > grossMinor {
		return "", errors.New("refund exceeds payment")
	}
	var priorGross, priorNet, priorCommission int64
	err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(COALESCE(gross_minor,(amount*100)::bigint)),0)::bigint,
 COALESCE(SUM(amount*100),0)::bigint,COALESCE(SUM(commission_amount*100),0)::bigint
 FROM conversions WHERE provider=$1 AND provider_object_id=$2 AND kind='refund'`, provider, objectID).Scan(&priorGross, &priorNet, &priorCommission)
	if err != nil {
		return "", err
	}
	if cumulativeMinor <= priorGross {
		return "", nil
	}
	netDelta := proratedMinor(netMinor, cumulativeMinor, grossMinor) - priorNet
	commissionDelta := proratedMinor(commissionMinor, cumulativeMinor, grossMinor) - priorCommission
	var refundID string
	err = tx.QueryRow(ctx, `INSERT INTO conversions(offer_id,campaign_id,click_id,provider,provider_event_id,provider_object_id,kind,amount,currency,commission_amount,occurred_at,gross_minor,payment_mode)
 VALUES($1,$2,$3,$4,$5,$6,'refund',$7,$8,$9,$10,$11,$12) ON CONFLICT(provider,provider_event_id) DO NOTHING RETURNING id`,
		offerID, campaignID, clickID, provider, eventID, objectID, float64(netDelta)/100, currency, float64(commissionDelta)/100, occurredAt, cumulativeMinor-priorGross, paymentMode).Scan(&refundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var status string
	var clearsAt *time.Time
	err = tx.QueryRow(ctx, `SELECT status,clears_at FROM ledger_entries WHERE conversion_id=$1 AND kind='commission' LIMIT 1`, saleID).Scan(&status, &clearsAt)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `WITH cancelled AS (UPDATE payouts SET status='cancelled' WHERE status='pending' AND id IN
 (SELECT payout_id FROM ledger_entries WHERE conversion_id=$1 AND payout_id IS NOT NULL) RETURNING id)
 UPDATE ledger_entries SET payout_id=NULL WHERE payout_id IN (SELECT id FROM cancelled)`, saleID); err != nil {
		return "", err
	}
	if status == "pending" && cumulativeMinor == grossMinor {
		_, err = tx.Exec(ctx, `UPDATE ledger_entries SET status='reversed' WHERE status='pending' AND conversion_id IN
  (SELECT id FROM conversions WHERE provider=$1 AND provider_object_id=$2)`, provider, objectID)
	} else if commissionDelta > 0 {
		_, err = tx.Exec(ctx, `INSERT INTO ledger_entries(site_id,conversion_id,kind,amount,currency,status,clears_at)
  VALUES($1,$2,'clawback',$3,$4,$5,$6)`, siteID, refundID, -float64(commissionDelta)/100, currency, status, clearsAt)
	}
	if err != nil {
		return "", err
	}
	return refundID, tx.Commit(ctx)
}

func proratedMinor(amount, refunded, total int64) int64 {
	numerator := new(big.Int).Mul(big.NewInt(amount), big.NewInt(refunded))
	numerator.Add(numerator, big.NewInt(total/2))
	return numerator.Quo(numerator, big.NewInt(total)).Int64()
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
