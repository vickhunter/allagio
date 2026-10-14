package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	TrustProviderVerified        = "provider_verified"
	TrustSellerSignedWebhook     = "seller_signed_webhook"
	TrustSellerReported          = "seller_reported"
	ProviderSellerReport         = "seller_report"
	RefundApplied                = "applied"
	RefundPendingReview          = "pending_review"
	RefundRejected               = "rejected"
	RefundDuplicate              = "duplicate"
	ReasonRefundTrustBelowSale   = "refund_trust_below_sale"
	ReasonRefundExceedsRemaining = "refund_exceeds_remaining"
	ReasonInvalidRefundAmount    = "invalid_refund_amount"
)

type RefundOutcome struct {
	ID         string
	TrustLevel string
	Status     string
	RowStatus  string
	Reason     string
}

type ConversionReport struct {
	ID           string
	Kind         string
	TrustLevel   string
	RefundStatus *string
	Reason       string
	Amount       float64
	Currency     string
	ObjectID     string
}

type ClickSaleContext struct {
	OfferID   string
	ProductID string
	MakerID   string
	Currency  string
}

func trustRank(level string) int {
	switch level {
	case TrustProviderVerified:
		return 3
	case TrustSellerSignedWebhook:
		return 2
	case TrustSellerReported:
		return 1
	default:
		return 0
	}
}

func saleTrustRank(level string) int {
	if trustRank(level) == 0 {
		return trustRank(TrustProviderVerified)
	}
	return trustRank(level)
}

type refundInsert struct {
	offerID, campaignID, provider, eventID, objectID, currency, paymentMode, trustLevel, status, reason, makerID, saleID string
	clickID                                                                                                              *string
	amount, commission                                                                                                   float64
	grossMinor                                                                                                           int64
	occurredAt                                                                                                           time.Time
}

func refundByEvent(ctx context.Context, tx pgx.Tx, provider, eventID, makerID string) (*RefundOutcome, bool, error) {
	var id, trust, reason string
	var status *string
	query := `SELECT cv.id::text, COALESCE(cv.trust_level,''), cv.refund_status, COALESCE(cv.refund_reason,'')
FROM conversions cv WHERE cv.provider=$1 AND cv.provider_event_id=$2`
	args := []any{provider, eventID}
	if provider == ProviderSellerReport {
		query = `SELECT cv.id::text, COALESCE(cv.trust_level,''), cv.refund_status, COALESCE(cv.refund_reason,'')
FROM conversions cv
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
WHERE cv.provider=$1 AND cv.provider_event_id=$2 AND p.maker_id::text=$3`
		args = append(args, makerID)
	}
	err := tx.QueryRow(ctx, query, args...).Scan(&id, &trust, &status, &reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	row := ""
	if status != nil {
		row = *status
	}
	return &RefundOutcome{ID: id, TrustLevel: trust, Status: RefundDuplicate, RowStatus: row, Reason: reason}, true, nil
}

func appliedRefundTotals(ctx context.Context, tx pgx.Tx, saleID string) (gross, net, commission int64, err error) {
	err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(COALESCE(gross_minor,(amount*100)::bigint)),0)::bigint,
 COALESCE(SUM(amount*100),0)::bigint, COALESCE(SUM(commission_amount*100),0)::bigint
 FROM conversions WHERE refunded_conversion_id=$1 AND kind='refund' AND COALESCE(refund_status,'applied')='applied'`, saleID).Scan(&gross, &net, &commission)
	return gross, net, commission, err
}

func insertRefund(ctx context.Context, tx pgx.Tx, row refundInsert) (*RefundOutcome, error) {
	var id string
	conflict := `ON CONFLICT (provider, provider_event_id) WHERE provider <> 'seller_report' DO NOTHING`
	if row.provider == ProviderSellerReport {
		conflict = `ON CONFLICT (offer_id, provider_event_id) WHERE provider = 'seller_report' DO NOTHING`
	}
	err := tx.QueryRow(ctx, `INSERT INTO conversions(offer_id,campaign_id,click_id,provider,provider_event_id,provider_object_id,kind,amount,currency,commission_amount,occurred_at,gross_minor,payment_mode,trust_level,refund_status,refund_reason,refunded_conversion_id)
VALUES($1,$2,$3,$4,$5,$6,'refund',$7,$8,$9,$10,NULLIF($11,0),$12,$13,$14,NULLIF($15,''),NULLIF($16,'')::uuid)
`+conflict+` RETURNING id::text`,
		row.offerID, row.campaignID, row.clickID, row.provider, row.eventID, row.objectID, row.amount, row.currency, row.commission, row.occurredAt, row.grossMinor, row.paymentMode, row.trustLevel, row.status, row.reason, row.saleID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, ok, qerr := refundByEvent(ctx, tx, row.provider, row.eventID, row.makerID)
		if qerr != nil || ok {
			return existing, qerr
		}
		return &RefundOutcome{TrustLevel: row.trustLevel, Status: RefundDuplicate, RowStatus: row.status, Reason: row.reason}, nil
	}
	if err != nil {
		return nil, err
	}
	return &RefundOutcome{ID: id, TrustLevel: row.trustLevel, Status: row.status, RowStatus: row.status, Reason: row.reason}, nil
}

func distinctSaleMakers(ctx context.Context, tx pgx.Tx, provider, objectID string) ([]string, error) {
	return makerIDsForSales(ctx, tx, `SELECT DISTINCT p.maker_id::text
FROM conversions cv
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
WHERE cv.provider=$1 AND cv.provider_object_id=$2 AND $2 <> '' AND cv.kind IN ('sale','renewal')`, provider, objectID)
}

func makerIDsForSales(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func commitRefund(ctx context.Context, tx pgx.Tx, out *RefundOutcome, err error) (*RefundOutcome, error) {
	if err != nil || out == nil || out.Status == RefundDuplicate {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (d *DB) ConversionByProviderEvent(ctx context.Context, provider, eventID string) (id, trust, refundStatus, reason string, ok bool, err error) {
	var status *string
	err = d.Pool.QueryRow(ctx, `SELECT id::text, COALESCE(trust_level,''), refund_status, COALESCE(refund_reason,'')
FROM conversions WHERE provider=$1 AND provider_event_id=$2`, provider, eventID).Scan(&id, &trust, &status, &reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", "", false, err
	}
	if status != nil {
		refundStatus = *status
	}
	return id, trust, refundStatus, reason, true, nil
}

func (d *DB) MakerIDForProduct(ctx context.Context, productID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT maker_id::text FROM products WHERE id=$1`, productID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrConversionNotFound
	}
	return id, err
}

func (d *DB) ClickSaleContext(ctx context.Context, clickID string) (ClickSaleContext, error) {
	var out ClickSaleContext
	err := d.Pool.QueryRow(ctx, `SELECT cam.offer_id::text, o.product_id::text, p.maker_id::text,
COALESCE(NULLIF(cl.terms_snapshot->>'currency',''), o.commission_currency)
FROM clicks cl
JOIN campaigns cam ON cam.id = cl.campaign_id
JOIN offers o ON o.id = cam.offer_id
JOIN products p ON p.id = o.product_id
WHERE cl.id=$1`, clickID).Scan(&out.OfferID, &out.ProductID, &out.MakerID, &out.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClickSaleContext{}, ErrClickNotFound
	}
	return out, err
}

func (d *DB) SaleForMaker(ctx context.Context, makerID, objectID, offerID, currency string) (saleCurrency string, ok bool, err error) {
	if objectID == "" {
		return "", false, nil
	}
	err = d.Pool.QueryRow(ctx, `SELECT cv.currency FROM conversions cv
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
WHERE cv.provider_object_id=$1 AND cv.kind IN ('sale','renewal') AND p.maker_id::text=$2
AND ($3='' OR o.id::text=$3)
AND ($4='' OR upper(cv.currency)=upper($4))
ORDER BY (cv.provider=$5) DESC, cv.created_at DESC LIMIT 1`, objectID, makerID, offerID, currency, ProviderSellerReport).Scan(&saleCurrency)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return saleCurrency, true, nil
}

func (d *DB) ListConversionReports(ctx context.Context, makerID, siteID, offerID string, limit int) ([]ConversionReport, error) {
	if limit <= 0 {
		limit = 50
	}
	if makerID == "" && siteID == "" && offerID == "" {
		return []ConversionReport{}, nil
	}
	rows, err := d.Pool.Query(ctx, `SELECT cv.id::text, cv.kind, COALESCE(cv.trust_level,''), cv.refund_status, COALESCE(cv.refund_reason,''),
cv.amount, cv.currency, COALESCE(cv.provider_object_id,'')
FROM conversions cv
JOIN campaigns cam ON cam.id = cv.campaign_id
JOIN offers o ON o.id = cv.offer_id
JOIN products p ON p.id = o.product_id
WHERE ($1='' OR p.maker_id::text=$1)
  AND ($2='' OR cam.site_id::text=$2)
  AND ($3='' OR o.id::text=$3)
ORDER BY cv.created_at DESC
LIMIT $4`, makerID, siteID, offerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConversionReport{}
	for rows.Next() {
		var row ConversionReport
		if err = rows.Scan(&row.ID, &row.Kind, &row.TrustLevel, &row.RefundStatus, &row.Reason, &row.Amount, &row.Currency, &row.ObjectID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
