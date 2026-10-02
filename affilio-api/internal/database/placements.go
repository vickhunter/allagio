package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrPlacementAuthority = errors.New("placement is not authorized, has expired, or terms changed")

type Placement struct {
	ID         string          `json:"id"`
	SiteID     string          `json:"site_id"`
	OfferID    string          `json:"offer_id"`
	SurfaceURL string          `json:"surface_url"`
	Context    string          `json:"context"`
	Copy       string          `json:"copy"`
	Disclosure string          `json:"disclosure"`
	Reason     string          `json:"reason"`
	Terms      json.RawMessage `json:"terms"`
	Status     string          `json:"status"`
	ExpiresAt  time.Time       `json:"expires_at"`
	Receipt    json.RawMessage `json:"receipt,omitempty"`
}

const placementColumns = `id,site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,status,expires_at,receipt`
const placementTermsSQL = `jsonb_build_object('product_id',o.product_id,'commission_type',o.commission_type,
 'commission_value_minor',(o.commission_value*100)::bigint,'currency',o.commission_currency,
 'cookie_duration_days',o.cookie_duration_days,'refund_window_days',o.refund_window_days,
 'landing_url',o.landing_url,'ref_param',o.ref_param,'terms',COALESCE(o.terms,''),
 'payment_mode',CASE WHEN pc.live_mode THEN 'live' ELSE 'test' END,'payout_method','direct_merchant','service_fee_percent_of_commission',10,'scope','first_paid_checkout')`

func scanPlacement(row interface{ Scan(...interface{}) error }) (*Placement, error) {
	var p Placement
	err := row.Scan(&p.ID, &p.SiteID, &p.OfferID, &p.SurfaceURL, &p.Context, &p.Copy, &p.Disclosure, &p.Reason, &p.Terms, &p.Status, &p.ExpiresAt, &p.Receipt)
	return &p, err
}

func (d *DB) CreatePlacement(ctx context.Context, p Placement) (*Placement, error) {
	return scanPlacement(d.Pool.QueryRow(ctx, `INSERT INTO placement_proposals(site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,expires_at)
 SELECT $1,o.id,$3,$4,$5,$6,$7,`+placementTermsSQL+`,$8 FROM offers o JOIN product_connections pc ON pc.product_id=o.product_id
 WHERE o.id=$2 AND o.status='listed' AND pc.enabled AND pc.currency=o.commission_currency
 RETURNING `+placementColumns, p.SiteID, p.OfferID, p.SurfaceURL, p.Context, p.Copy, p.Disclosure, p.Reason, p.ExpiresAt))
}

func (d *DB) GetPlacement(ctx context.Context, siteID, id string) (*Placement, error) {
	return scanPlacement(d.Pool.QueryRow(ctx, `SELECT `+placementColumns+` FROM placement_proposals WHERE id=$1 AND site_id=$2`, id, siteID))
}

func (d *DB) ListPlacements(ctx context.Context, siteID string) ([]Placement, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+placementColumns+` FROM placement_proposals WHERE site_id=$1 ORDER BY created_at DESC LIMIT 50`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Placement{}
	for rows.Next() {
		p, err := scanPlacement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (d *DB) ApprovePlacement(ctx context.Context, id, siteID, ownerEmail, evidence string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals pp SET status='approved',approved_at=NOW(),approval_evidence=$4
 FROM offers o JOIN products p ON p.id=o.product_id JOIN makers m ON m.id=p.maker_id
 WHERE pp.id=$1 AND pp.site_id=$2 AND pp.offer_id=o.id AND pp.status='pending' AND pp.expires_at>NOW()
 AND lower(m.email)<>lower($3)`, id, siteID, ownerEmail, evidence)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) RevokePlacement(ctx context.Context, siteID, id string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='revoked' WHERE id=$1 AND site_id=$2 AND status<>'revoked'`, id, siteID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) CommitPlacement(ctx context.Context, siteID, id, code string) (*Placement, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	p, err := scanPlacement(tx.QueryRow(ctx, `SELECT `+placementColumns+` FROM placement_proposals WHERE id=$1 AND site_id=$2 FOR UPDATE`, id, siteID))
	if err != nil {
		return nil, err
	}
	if !time.Now().Before(p.ExpiresAt) {
		return nil, ErrPlacementAuthority
	}
	if p.Status == "committed" {
		return p, nil
	}
	if p.Status != "approved" {
		return nil, ErrPlacementAuthority
	}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM offers o JOIN product_connections pc ON pc.product_id=o.product_id
 WHERE o.id=$1 AND o.status='listed' AND pc.enabled AND pc.currency=o.commission_currency AND `+placementTermsSQL+`=$2::jsonb)`, p.OfferID, p.Terms).Scan(&eligible)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, ErrPlacementAuthority
	}
	var campaignID, trackingCode, status string
	err = tx.QueryRow(ctx, `INSERT INTO campaigns(offer_id,site_id,tracking_code,requires_placement) VALUES($1,$2,$3,true)
 ON CONFLICT(offer_id,site_id) DO UPDATE SET requires_placement=true
 RETURNING id,tracking_code,status`, p.OfferID, siteID, code).Scan(&campaignID, &trackingCode, &status)
	if err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, ErrPlacementAuthority
	}
	receipt, err := json.Marshal(map[string]interface{}{"placement_id": p.ID, "campaign_id": campaignID,
		"tracked_link": "/v1/go/" + trackingCode + "?placement=" + p.ID, "publication_status": "awaiting_evidence",
		"approved_text": p.Copy + "\n\n" + p.Disclosure, "terms": p.Terms})
	if err != nil {
		return nil, err
	}
	p, err = scanPlacement(tx.QueryRow(ctx, `UPDATE placement_proposals SET status='committed',campaign_id=$2,receipt=$3 WHERE id=$1 RETURNING `+placementColumns, p.ID, campaignID, receipt))
	if err != nil {
		return nil, err
	}
	return p, tx.Commit(ctx)
}

func (d *DB) PlacementAllowsClick(ctx context.Context, placementID, campaignID string) (bool, error) {
	var valid bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM placement_proposals WHERE id=$1 AND campaign_id=$2 AND status='committed' AND expires_at>NOW())`, placementID, campaignID).Scan(&valid)
	return valid, err
}

func (d *DB) RecordPlacementReceipt(ctx context.Context, siteID, id, surfaceURL, contentHash, evidence string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET receipt=receipt || jsonb_build_object(
 'publication_status','publisher_reported','surface_url',$3::text,'content_sha256',$4::text,'evidence',$5::text)
 WHERE id=$1 AND site_id=$2 AND surface_url=$3 AND status='committed' AND expires_at>NOW()
 AND receipt->>'publication_status'='awaiting_evidence'`, id, siteID, surfaceURL, contentHash, evidence)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) CreateAgentAccess(ctx context.Context, siteID, keyHash string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO publisher_agent_keys(site_id,key_hash,expires_at) VALUES($1,$2,NOW()+INTERVAL '7 days') RETURNING id`, siteID, keyHash).Scan(&id)
	return id, err
}

func (d *DB) AgentSite(ctx context.Context, keyHash string) (string, error) {
	var siteID string
	err := d.Pool.QueryRow(ctx, `SELECT site_id FROM publisher_agent_keys WHERE key_hash=$1 AND revoked_at IS NULL AND expires_at>NOW()`, keyHash).Scan(&siteID)
	return siteID, err
}

func (d *DB) RevokeAgentAccess(ctx context.Context, id string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE publisher_agent_keys SET revoked_at=NOW() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}
