package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrPlacementAuthority = errors.New("placement is not authorized, has expired, or terms changed")

type Placement struct {
	ID            string          `json:"id"`
	SiteID        string          `json:"site_id"`
	OfferID       string          `json:"offer_id"`
	SurfaceURL    string          `json:"surface_url"`
	Context       string          `json:"context"`
	Copy          string          `json:"copy"`
	Disclosure    string          `json:"disclosure"`
	Reason        string          `json:"reason"`
	Terms         json.RawMessage `json:"terms"`
	Status        string          `json:"status"`
	ExpiresAt     time.Time       `json:"expires_at"`
	Receipt       json.RawMessage `json:"receipt,omitempty"`
	RemovedAt     *time.Time      `json:"removed_at,omitempty"`
	ApprovedBy    string          `json:"approved_by,omitempty"`
	PolicyVersion int             `json:"policy_version,omitempty"`
	MatchedRule   string          `json:"matched_rule,omitempty"`
	AutoPaused    bool            `json:"auto_paused,omitempty"`
}

const placementColumns = `id,site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,status,expires_at,receipt,removed_at,COALESCE(approved_by_kind,''),COALESCE(policy_version,0),COALESCE(matched_rule,''),auto_paused`
const placementTermsSQL = `jsonb_build_object('product_id',o.product_id,'commission_type',o.commission_type,
 'commission_value_minor',(o.commission_value*100)::bigint,'currency',o.commission_currency,
 'cookie_duration_days',o.cookie_duration_days,'refund_window_days',o.refund_window_days,
 'landing_url',o.landing_url,'ref_param',o.ref_param,'terms',COALESCE(o.terms,''),
 'payment_mode',CASE WHEN pc.product_id IS NULL THEN 'none' WHEN pc.live_mode THEN 'live' ELSE 'test' END,'payout_method','direct_merchant','service_fee_percent_of_commission',10,'scope','first_paid_checkout')`

// A listed percentage or flat offer is placeable when its product has no payment
// provider, or when a Stripe product has an enabled connection in the offer currency.
const placementEligibleSQL = `FROM offers o JOIN products p ON p.id=o.product_id
 LEFT JOIN product_connections pc ON pc.product_id=o.product_id AND pc.enabled AND pc.currency=o.commission_currency
 WHERE o.status='listed' AND o.commission_type IN ('percentage','flat')
 AND (COALESCE(NULLIF(p.payment_provider, ''), 'none') = 'none' OR (p.payment_provider='stripe' AND pc.product_id IS NOT NULL))`

func scanPlacement(row interface{ Scan(...interface{}) error }) (*Placement, error) {
	var p Placement
	err := row.Scan(&p.ID, &p.SiteID, &p.OfferID, &p.SurfaceURL, &p.Context, &p.Copy, &p.Disclosure, &p.Reason, &p.Terms, &p.Status, &p.ExpiresAt, &p.Receipt, &p.RemovedAt, &p.ApprovedBy, &p.PolicyVersion, &p.MatchedRule, &p.AutoPaused)
	return &p, err
}

type MakerPlacement struct {
	ID         string     `json:"id"`
	SurfaceURL string     `json:"surface_url"`
	Copy       string     `json:"copy"`
	Status     string     `json:"status"`
	RemovedAt  *time.Time `json:"removed_at,omitempty"`
}

func (d *DB) ListMakerPlacements(ctx context.Context, makerID string) ([]MakerPlacement, error) {
	rows, err := d.Pool.Query(ctx, `SELECT pp.id, pp.surface_url, pp.copy, pp.status, pp.removed_at
FROM placement_proposals pp
JOIN offers o ON o.id=pp.offer_id
JOIN products p ON p.id=o.product_id
WHERE p.maker_id=$1
ORDER BY pp.created_at DESC LIMIT 50`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MakerPlacement{}
	for rows.Next() {
		var item MakerPlacement
		if err = rows.Scan(&item.ID, &item.SurfaceURL, &item.Copy, &item.Status, &item.RemovedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) MarkPlacementRemoved(ctx context.Context, siteID, id string) (already bool, at time.Time, found bool, err error) {
	err = d.Pool.QueryRow(ctx, `UPDATE placement_proposals SET removed_at=NOW()
WHERE id=$1 AND site_id=$2 AND removed_at IS NULL RETURNING removed_at`, id, siteID).Scan(&at)
	if err == nil {
		return false, at, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, time.Time{}, false, err
	}
	err = d.Pool.QueryRow(ctx, `SELECT removed_at FROM placement_proposals WHERE id=$1 AND site_id=$2 AND removed_at IS NOT NULL`, id, siteID).Scan(&at)
	if err == nil {
		return true, at, true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, time.Time{}, false, nil
	}
	return false, time.Time{}, false, err
}

func (d *DB) PlacementRemoved(ctx context.Context, placementID, campaignID string) (bool, error) {
	var removed bool
	err := d.Pool.QueryRow(ctx, `SELECT removed_at IS NOT NULL FROM placement_proposals WHERE id=$1 AND campaign_id=$2`, placementID, campaignID).Scan(&removed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return removed, err
}

func (d *DB) CreatePlacement(ctx context.Context, p Placement) (*Placement, error) {
	return scanPlacement(d.Pool.QueryRow(ctx, `INSERT INTO placement_proposals(site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,expires_at)
 SELECT $1,o.id,$3,$4,$5,$6,$7,`+placementTermsSQL+`,$8 `+placementEligibleSQL+` AND o.id=$2
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
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals pp SET status='approved',approved_at=NOW(),approval_evidence=$4,
approved_by_kind='owner', matched_rule='owner_evidence',
policy_version=COALESCE((SELECT version FROM approval_policies WHERE site_id=pp.site_id),0)
 FROM offers o JOIN products p ON p.id=o.product_id JOIN makers m ON m.id=p.maker_id
 WHERE pp.id=$1 AND pp.site_id=$2 AND pp.offer_id=o.id AND pp.status='pending' AND pp.expires_at>NOW()
 AND lower(m.email)<>lower($3)`, id, siteID, ownerEmail, evidence)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) RevokePlacement(ctx context.Context, siteID, id string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='revoked', revoked_by_side='publisher' WHERE id=$1 AND site_id=$2 AND status<>'revoked'`, id, siteID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) GetPlacementByID(ctx context.Context, id string) (*Placement, error) {
	return scanPlacement(d.Pool.QueryRow(ctx, `SELECT `+placementColumns+` FROM placement_proposals WHERE id=$1`, id))
}

// RevokePlacementForSide revokes a placement the caller is allowed to undo.
// reason "undo" records the previous status, the server time, and the key or
// session that undid it, and writes a placement_undo event in the same commit.
func (d *DB) RevokePlacementForSide(ctx context.Context, placementID, side, userID, keyHash, actor, reason string, now time.Time) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals SET
status_before_undo=CASE WHEN $3='undo' THEN status ELSE status_before_undo END,
undone_at=CASE WHEN $3='undo' THEN $4 ELSE undone_at END,
undone_by_user=CASE WHEN $3='undo' THEN NULLIF($5,'')::uuid ELSE undone_by_user END,
undone_by_key_hash=CASE WHEN $3='undo' THEN NULLIF($6,'') ELSE undone_by_key_hash END,
status='revoked', revoked_by_side=$2
WHERE id=$1 AND status<>'revoked'`, placementID, side, reason, now, userID, keyHash)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() != 1 {
		return false, tx.Commit(ctx)
	}
	if reason == "undo" {
		if _, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,created_at,actor) VALUES('placement_undo',$1,$2,$3,$4)`, userID, placementID, now, actor); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

// PlacementUndo is the server record of the latest undo for one placement.
type PlacementUndo struct {
	Status       string
	StatusBefore string
	UndoneAt     *time.Time
	UserID       string
	KeyHash      string
	ApprovedBy   string
	Disclosure   string
	Copy         string
	SiteID       string
	SurfaceURL   string
	OfferID      string
}

func (d *DB) PlacementUndo(ctx context.Context, id string) (PlacementUndo, error) {
	var row PlacementUndo
	err := d.Pool.QueryRow(ctx, `SELECT status, COALESCE(status_before_undo,''), undone_at, COALESCE(undone_by_user::text,''), COALESCE(undone_by_key_hash,''), COALESCE(approved_by_kind,''), disclosure, copy, site_id::text, surface_url, offer_id::text
FROM placement_proposals WHERE id=$1`, id).Scan(&row.Status, &row.StatusBefore, &row.UndoneAt, &row.UserID, &row.KeyHash, &row.ApprovedBy, &row.Disclosure, &row.Copy, &row.SiteID, &row.SurfaceURL, &row.OfferID)
	return row, err
}

// RestoreUndonePlacement puts the placement back only when the same identity
// is still inside the undo window. approved_by_kind is left as it was.
func (d *DB) RestoreUndonePlacement(ctx context.Context, id, userID, keyHash, actor string, now time.Time) (status, approvedBy string, ok bool, err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", false, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `UPDATE placement_proposals SET status=status_before_undo, restored_by=$2, undone_at=NULL
WHERE id=$1 AND status='revoked' AND status_before_undo<>'' AND undone_at IS NOT NULL
AND undone_by_user=$2::uuid AND COALESCE(undone_by_key_hash,'')=$3
AND undone_at >= $4::timestamptz - INTERVAL '6 seconds'
RETURNING status, COALESCE(approved_by_kind,''), COALESCE(restored_by,'')`, id, userID, keyHash, now).Scan(&status, &approvedBy, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,created_at,actor) VALUES('placement_restore',$1,$2,$3,$4)`, userID, id, now, actor); err != nil {
		return "", "", false, err
	}
	return status, approvedBy, true, tx.Commit(ctx)
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
	if p.Status != "approved" || p.AutoPaused {
		return nil, ErrPlacementAuthority
	}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 `+placementEligibleSQL+` AND o.id=$1 AND `+placementTermsSQL+`=$2::jsonb)`, p.OfferID, p.Terms).Scan(&eligible)
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
