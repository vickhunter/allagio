package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"affilio-api/internal/entry"

	"github.com/jackc/pgx/v5"
)

var ErrPlacementAuthority = errors.New("placement is not authorized, has expired, or terms changed")

// ErrOnePerPost is the second box on an article that already has one.
var ErrOnePerPost = errors.New("one box per post")

// ErrClaimsFromOffer is copy that states a number the offer does not contain.
var ErrClaimsFromOffer = errors.New("claims_from_offer")

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
	WPPostID      int64           `json:"-"`
	PostTitle     string          `json:"-"`
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
	err = d.Pool.QueryRow(ctx, `UPDATE placement_proposals SET removed_at=NOW(), plugin_wait_before=plugin_wait, plugin_wait='remove', plugin_wait_at=NOW()
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

// PlacementPostURL is the referring post stored for this placement on the
// campaign. An unknown placement returns an empty string.
func (d *DB) PlacementPostURL(ctx context.Context, placementID, campaignID string) (string, error) {
	var post string
	err := d.Pool.QueryRow(ctx, `SELECT COALESCE(post_url,'') FROM placement_proposals WHERE id=$1 AND campaign_id=$2`, placementID, campaignID).Scan(&post)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return post, err
}

func (d *DB) PlacementRevoked(ctx context.Context, placementID, campaignID string) (bool, error) {
	var revoked bool
	err := d.Pool.QueryRow(ctx, `SELECT status='revoked' FROM placement_proposals WHERE id=$1 AND campaign_id=$2`, placementID, campaignID).Scan(&revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return revoked, err
}

// PlacementRestoreUnconfirmed is true after a failed restore until a verified
// restored acknowledgement confirms the block is placed again.
func (d *DB) PlacementRestoreUnconfirmed(ctx context.Context, placementID, campaignID string) (bool, error) {
	var held bool
	err := d.Pool.QueryRow(ctx, `SELECT restore_unconfirmed FROM placement_proposals WHERE id=$1 AND campaign_id=$2`, placementID, campaignID).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return held, err
}

func (d *DB) CreatePlacement(ctx context.Context, p Placement) (*Placement, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// One box per post. The lock is per site so a second prepare waits, then
	// sees the row the first prepare inserted.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, p.SiteID); err != nil {
		return nil, err
	}
	boxes, err := countPlacementBoxes(ctx, tx, p.SiteID, p.SurfaceURL, "00000000-0000-0000-0000-000000000000", p.WPPostID)
	if err != nil {
		return nil, err
	}
	if boxes > 0 {
		return nil, ErrOnePerPost
	}
	created, err := scanPlacement(tx.QueryRow(ctx, `INSERT INTO placement_proposals(site_id,offer_id,surface_url,context,copy,disclosure,reason,terms_snapshot,expires_at,post_url,wp_post_id,post_title)
 SELECT $1,o.id,$3,$4,$5,$6,$7,`+placementTermsSQL+`,$8,$3,$9,COALESCE(NULLIF($10,''), (
  SELECT a.title FROM articles a
  WHERE a.site_id=$1::uuid AND COALESCE(a.title,'') <> ''
    AND regexp_replace(lower(split_part(split_part(COALESCE(a.url,''), '#', 1), '?', 1)), '/+$', '')
      = regexp_replace(lower(split_part(split_part($3, '#', 1), '?', 1)), '/+$', '')
  ORDER BY a.updated_at DESC LIMIT 1
), '') `+placementEligibleSQL+` AND o.id=$2
 RETURNING `+placementColumns, p.SiteID, p.OfferID, p.SurfaceURL, p.Context, p.Copy, p.Disclosure, p.Reason, p.ExpiresAt, p.WPPostID, p.PostTitle))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
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
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='revoked', revoked_by_side='publisher', plugin_wait_before=plugin_wait, plugin_wait='remove', plugin_wait_at=NOW() WHERE id=$1 AND site_id=$2 AND status<>'revoked'`, id, siteID)
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
plugin_wait_before=plugin_wait,
plugin_wait='remove',
plugin_wait_at=$4,
plugin_failed_reason=CASE WHEN $3='undo' THEN '' ELSE plugin_failed_reason END,
failed_action=CASE WHEN $3='undo' THEN '' ELSE failed_action END,
restore_unconfirmed=CASE WHEN $3='undo' THEN false ELSE restore_unconfirmed END,
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
	Relevance    int
}

func (d *DB) PlacementUndo(ctx context.Context, id string) (PlacementUndo, error) {
	var row PlacementUndo
	err := d.Pool.QueryRow(ctx, `SELECT status, COALESCE(status_before_undo,''), undone_at, COALESCE(undone_by_user::text,''), COALESCE(undone_by_key_hash,''), COALESCE(approved_by_kind,''), disclosure, copy, site_id::text, surface_url, offer_id::text, COALESCE(relevance,-1)
FROM placement_proposals WHERE id=$1`, id).Scan(&row.Status, &row.StatusBefore, &row.UndoneAt, &row.UserID, &row.KeyHash, &row.ApprovedBy, &row.Disclosure, &row.Copy, &row.SiteID, &row.SurfaceURL, &row.OfferID, &row.Relevance)
	return row, err
}

// RestoreUndonePlacement puts the placement back only when the same identity
// is still inside the undo window. Pause, the placement cap, and the offer
// claim check run in this transaction, after the row is locked.
// approved_by_kind is left as it was. block is paused, cap, claims_from_offer,
// or window_expired when the update does not run.
func (d *DB) RestoreUndonePlacement(ctx context.Context, id, userID, keyHash, actor string, now time.Time, cap int, claims entry.ClaimChecker) (status, approvedBy, block string, ok bool, err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", "", false, err
	}
	defer tx.Rollback(ctx)
	var siteID, offerID string
	var undoneAt time.Time
	err = tx.QueryRow(ctx, `SELECT site_id::text, offer_id::text, undone_at
FROM placement_proposals
WHERE id=$1 AND status='revoked' AND status_before_undo<>'' AND undone_at IS NOT NULL
AND undone_by_user=$2::uuid AND ($5 = 'owner_ui' OR COALESCE(undone_by_key_hash,'')=$3)
AND undone_at >= $4::timestamptz - INTERVAL '6 seconds'
FOR UPDATE`, id, userID, keyHash, now, actor).Scan(&siteID, &offerID, &undoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "window_expired", false, nil
	}
	if err != nil {
		return "", "", "", false, err
	}
	var paused bool
	err = tx.QueryRow(ctx, `SELECT
EXISTS (
    SELECT 1 FROM pause_activations pa
    WHERE pa.created_at >= $3
      AND (
        pa.site_id = $1::uuid
        OR pa.offer_id = $2::uuid
        OR pa.owner_id IN (
            SELECT id FROM owners WHERE site_id = $1::uuid
            UNION
            SELECT o.id FROM owners o
            JOIN products p ON p.maker_id = o.maker_id
            JOIN offers f ON f.product_id = p.id
            WHERE f.id = $2::uuid
        )
      )
)
OR EXISTS (SELECT 1 FROM approval_policies WHERE site_id=$1::uuid AND kill_switch)
OR EXISTS (SELECT 1 FROM approval_policies WHERE offer_id=$2::uuid AND kill_switch)`, siteID, offerID, undoneAt).Scan(&paused)
	if err != nil {
		return "", "", "", false, err
	}
	if paused {
		return "", "", "paused", false, nil
	}
	var used int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND created_at>$2 AND approved_by_kind IN ('policy:auto','policy:yolo')
AND status<>'revoked' AND id<>$3`, siteID, now.Add(-30*24*time.Hour), id).Scan(&used)
	if err != nil {
		return "", "", "", false, err
	}
	if used >= cap {
		return "", "", "cap", false, nil
	}
	var copyText, product, category, website, terms, commission string
	err = tx.QueryRow(ctx, `SELECT pp.copy, COALESCE(p.name,''), COALESCE(p.primary_category,''), COALESCE(p.website_url,''), COALESCE(o.terms,''), COALESCE(o.commission_value::text,'')
FROM placement_proposals pp
JOIN offers o ON o.id = pp.offer_id
JOIN products p ON p.id = o.product_id
WHERE pp.id=$1`, id).Scan(&copyText, &product, &category, &website, &terms, &commission)
	if err != nil {
		return "", "", "", false, err
	}
	if entry.ClaimsFail(claims, copyText, strings.TrimSpace(strings.Join([]string{product, category, website, terms, commission}, "\n"))) {
		return "", "", "claims_from_offer", false, nil
	}
	err = tx.QueryRow(ctx, `UPDATE placement_proposals SET status=status_before_undo, restored_by=$2, undone_at=NULL,
plugin_wait=CASE WHEN plugin_wait='remove' AND plugin_wait_before='placed' THEN plugin_wait_before ELSE 'restore' END,
plugin_wait_at=CASE WHEN plugin_wait='remove' AND plugin_wait_before='placed' THEN plugin_wait_at ELSE $3 END,
plugin_failed_reason='', failed_action='', restore_unconfirmed=false
WHERE id=$1
RETURNING status, COALESCE(approved_by_kind,'')`, id, userID, now).Scan(&status, &approvedBy)
	if err != nil {
		return "", "", "", false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,created_at,actor) VALUES('placement_restore',$1,$2,$3,$4)`, userID, id, now, actor); err != nil {
		return "", "", "", false, err
	}
	return status, approvedBy, "", true, tx.Commit(ctx)
}

func (d *DB) CommitPlacement(ctx context.Context, siteID, id, code string, claims entry.ClaimChecker) (*Placement, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	p, err := scanPlacement(tx.QueryRow(ctx, `SELECT `+placementColumns+` FROM placement_proposals WHERE id=$1 AND site_id=$2 FOR UPDATE`, id, siteID))
	if err != nil {
		return nil, err
	}
	if p.Status == "committed" {
		return p, nil
	}
	if !time.Now().Before(p.ExpiresAt) {
		return nil, ErrPlacementAuthority
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
	var product, category, website, terms, commission string
	err = tx.QueryRow(ctx, `SELECT COALESCE(pr.name,''), COALESCE(pr.primary_category,''), COALESCE(pr.website_url,''), COALESCE(o.terms,''), COALESCE(o.commission_value::text,'')
FROM offers o JOIN products pr ON pr.id=o.product_id WHERE o.id=$1`, p.OfferID).Scan(&product, &category, &website, &terms, &commission)
	if err != nil {
		return nil, err
	}
	if entry.ClaimsFail(claims, p.Copy, strings.TrimSpace(strings.Join([]string{product, category, website, terms, commission}, "\n"))) {
		return nil, ErrClaimsFromOffer
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, siteID); err != nil {
		return nil, err
	}
	var postID int64
	if err = tx.QueryRow(ctx, `SELECT wp_post_id FROM placement_proposals WHERE id=$1`, p.ID).Scan(&postID); err != nil {
		return nil, err
	}
	boxes, err := countPlacementBoxes(ctx, tx, siteID, p.SurfaceURL, p.ID, postID)
	if err != nil {
		return nil, err
	}
	if boxes > 0 {
		return nil, ErrOnePerPost
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
	var postTitle string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(post_title,''), wp_post_id FROM placement_proposals WHERE id=$1`, p.ID).Scan(&postTitle, &postID); err != nil {
		return nil, err
	}
	receipt, err := json.Marshal(map[string]interface{}{"placement_id": p.ID, "campaign_id": campaignID,
		"tracked_link": "/v1/go/" + p.ID, "publication_status": "awaiting_evidence",
		"approved_text": p.Copy + "\n\n" + p.Disclosure, "terms": p.Terms,
		"post_title": postTitle, "post_id": postID, "surface_url": p.SurfaceURL})
	if err != nil {
		return nil, err
	}
	p, err = scanPlacement(tx.QueryRow(ctx, `UPDATE placement_proposals SET status='committed',campaign_id=$2,receipt=$3,plugin_wait_at=COALESCE(plugin_wait_at, NOW()) WHERE id=$1 RETURNING `+placementColumns, p.ID, campaignID, receipt))
	if err != nil {
		return nil, err
	}
	return p, tx.Commit(ctx)
}

func (d *DB) PlacementAllowsClick(ctx context.Context, placementID, campaignID string) (bool, error) {
	var valid bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM placement_proposals WHERE id=$1 AND campaign_id=$2 AND status='committed')`, placementID, campaignID).Scan(&valid)
	return valid, err
}

// ReopenRefusedReceipt returns a committed placement to pending when the
// publisher's receipt does not match. The tracked link stays on the receipt.
// A later matching receipt can commit it again. A receipt that already
// reported publication is left committed.
func (d *DB) ReopenRefusedReceipt(ctx context.Context, siteID, id string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE placement_proposals
SET status='pending',
    receipt = COALESCE(receipt, '{}'::jsonb) || jsonb_build_object('publication_status', 'refused')
WHERE id=$1 AND site_id=$2 AND status='committed'
  AND COALESCE(receipt->>'publication_status', '') <> 'publisher_reported'`, id, siteID)
	return err
}

func (d *DB) RecordPlacementReceipt(ctx context.Context, siteID, id, surfaceURL, contentHash, evidence, postTitle string, postID int64, at time.Time) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='committed',
 post_title = COALESCE(NULLIF($7, ''), NULLIF(post_title, ''), ''),
 wp_post_id = CASE WHEN $8::bigint > 0 THEN $8::bigint ELSE wp_post_id END,
 receipt=receipt || jsonb_build_object(
 'publication_status','publisher_reported','surface_url',$3::text,'content_sha256',$4::text,'evidence',$5::text,
 'post_title', COALESCE(NULLIF($7, ''), NULLIF(post_title, ''), receipt->>'post_title', ''),
 'post_id', CASE WHEN $8::bigint > 0 THEN $8::bigint ELSE wp_post_id END),
 plugin_contact_at=$6
 WHERE id=$1 AND site_id=$2 AND surface_url=$3
 AND (
   (status='committed' AND receipt->>'publication_status'='awaiting_evidence')
   OR (status='pending' AND receipt->>'publication_status'='refused')
 )`, id, siteID, surfaceURL, contentHash, evidence, at, postTitle, postID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

// RefusePlacement returns a placement to pending when the plugin will not
// insert it, before a receipt. publication_status refused is the same open
// state as a refused receipt, so the row no longer occupies the post.
// A receipt that already reported publication is left committed.
// Calling it again on an already refused row is a no-op success.
func (d *DB) RefusePlacement(ctx context.Context, siteID, id, reason string) (string, error) {
	var status, publication string
	err := d.Pool.QueryRow(ctx, `SELECT status, COALESCE(receipt->>'publication_status','')
FROM placement_proposals WHERE id=$1 AND site_id=$2`, id, siteID).Scan(&status, &publication)
	if errors.Is(err, pgx.ErrNoRows) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if publication == "publisher_reported" {
		return "recorded", nil
	}
	if status != "pending" && status != "approved" && status != "committed" {
		return "unavailable", nil
	}
	_, err = d.Pool.Exec(ctx, `UPDATE placement_proposals
SET status='pending',
    receipt = COALESCE(receipt, '{}'::jsonb) || jsonb_build_object('publication_status', 'refused', 'refuse_reason', $3::text)
WHERE id=$1 AND site_id=$2
  AND status IN ('pending','approved','committed')
  AND COALESCE(receipt->>'publication_status', '') <> 'publisher_reported'`, id, siteID, reason)
	if err != nil {
		return "", err
	}
	return "refused", nil
}

func (d *DB) TouchPlacementContact(ctx context.Context, siteID string, at time.Time) error {
	_, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET plugin_contact_at=$2 WHERE site_id=$1`, siteID, at)
	return err
}

func (d *DB) AckPluginPlacement(ctx context.Context, id, siteID, action, reason string, at time.Time) (bool, error) {
	switch action {
	case "placed", "restored", "removed":
		// placed and removed are stored only here, from the plugin key's ack.
		wait, ok := entry.VerifiedPluginWait(action)
		if !ok {
			return false, nil
		}
		pending := "plugin_wait='remove'"
		if action == "placed" || action == "restored" {
			pending = "plugin_wait IN ('', 'restore')"
		}
		tag, err := d.Pool.Exec(ctx, `UPDATE placement_proposals
SET plugin_wait=$4, plugin_wait_at=$3, plugin_contact_at=$3, plugin_failed_reason='', failed_action='', restore_unconfirmed=false
WHERE id=$1 AND site_id=$2 AND `+pending, id, siteID, at, wait)
		return err == nil && tag.RowsAffected() == 1, err
	case "failed":
		tag, err := d.Pool.Exec(ctx, `UPDATE placement_proposals
SET plugin_failed_reason=$4, plugin_contact_at=$3,
failed_action=CASE WHEN plugin_wait='remove' THEN 'removed' ELSE 'restored' END,
restore_unconfirmed=CASE WHEN plugin_wait='remove' THEN false ELSE true END
WHERE id=$1 AND site_id=$2 AND plugin_wait IN ('', 'restore', 'remove')`, id, siteID, at, reason)
		return err == nil && tag.RowsAffected() == 1, err
	default:
		return false, nil
	}
}

// RetryPluginPlacement re-queues only the action stored when the failure
// arrived. removed stays a removal. restored stays a re-insert and re-checks
// the offer's claims in this same transaction, before the row is updated.
// Status and restore_unconfirmed are left alone, so a revoked or failed-restore
// go link stays stopped. block is claims_from_offer when that check refuses.
func (d *DB) RetryPluginPlacement(ctx context.Context, id, siteID, action string, at time.Time, claims entry.ClaimChecker) (ok bool, block string, err error) {
	wait := "restore"
	if action == "removed" {
		wait = "remove"
	} else if action != "restored" {
		return false, "", nil
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback(ctx)
	var copyText, product, category, website, terms, commission string
	err = tx.QueryRow(ctx, `SELECT pp.copy, COALESCE(p.name,''), COALESCE(p.primary_category,''), COALESCE(p.website_url,''), COALESCE(o.terms,''), COALESCE(o.commission_value::text,'')
FROM placement_proposals pp
JOIN offers o ON o.id = pp.offer_id
JOIN products p ON p.id = o.product_id
WHERE pp.id=$1 AND pp.site_id=$2 AND pp.plugin_failed_reason <> '' AND pp.failed_action=$3
AND pp.plugin_wait IN ('', 'restore', 'remove')
FOR UPDATE OF pp`, id, siteID, action).Scan(&copyText, &product, &category, &website, &terms, &commission)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if action == "restored" && entry.ClaimsFail(claims, copyText, strings.TrimSpace(strings.Join([]string{product, category, website, terms, commission}, "\n"))) {
		return false, "claims_from_offer", nil
	}
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals
SET plugin_failed_reason='', failed_action='', plugin_wait=$4, plugin_wait_at=$3, plugin_contact_at=$3
WHERE id=$1 AND site_id=$2 AND plugin_failed_reason <> '' AND failed_action=$5
AND plugin_wait IN ('', 'restore', 'remove')`, id, siteID, at, wait, action)
	if err != nil {
		return false, "", err
	}
	if tag.RowsAffected() != 1 {
		return false, "", nil
	}
	return true, "", tx.Commit(ctx)
}

// PlacementStatusSQL is the placement poll. It reads one row by primary key
// and does not join another table.
const PlacementStatusSQL = `SELECT status, plugin_wait, plugin_wait_at, plugin_failed_reason, failed_action, COALESCE(matched_rule,''), surface_url FROM placement_proposals WHERE id=$1 AND site_id=$2`

func (d *DB) ReadPlacementStatus(ctx context.Context, siteID, id string) (status, wait, failedReason, failedAction, matchedRule, surfaceURL string, waitAt *time.Time, err error) {
	err = d.Pool.QueryRow(ctx, PlacementStatusSQL, id, siteID).Scan(&status, &wait, &waitAt, &failedReason, &failedAction, &matchedRule, &surfaceURL)
	return status, wait, failedReason, failedAction, matchedRule, surfaceURL, waitAt, err
}

// PlacementFallbackURL is the offer's plain landing URL for one placement.
func (d *DB) PlacementFallbackURL(ctx context.Context, siteID, id string) (string, error) {
	var landing string
	err := d.Pool.QueryRow(ctx, `SELECT o.landing_url
FROM placement_proposals pp
JOIN offers o ON o.id = pp.offer_id
WHERE pp.id=$1 AND pp.site_id=$2`, id, siteID).Scan(&landing)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return landing, err
}

// PlacementFallbackURLs maps each placement on a site to the offer's plain URL.
func (d *DB) PlacementFallbackURLs(ctx context.Context, siteID string) (map[string]string, error) {
	rows, err := d.Pool.Query(ctx, `SELECT pp.id::text, o.landing_url
FROM placement_proposals pp
JOIN offers o ON o.id = pp.offer_id
WHERE pp.site_id=$1`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, landing string
		if err = rows.Scan(&id, &landing); err != nil {
			return nil, err
		}
		out[id] = landing
	}
	return out, rows.Err()
}

func (d *DB) PlacementPlugin(ctx context.Context, id string) (wait string, waitAt, contact *time.Time, failedReason, failedAction string, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT plugin_wait, plugin_wait_at, plugin_contact_at, plugin_failed_reason, failed_action FROM placement_proposals WHERE id=$1`, id).Scan(&wait, &waitAt, &contact, &failedReason, &failedAction)
	return wait, waitAt, contact, failedReason, failedAction, err
}

func (d *DB) CreateAgentAccess(ctx context.Context, siteID, keyHash string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO publisher_agent_keys(site_id,key_hash,expires_at) VALUES($1,$2,NOW()+INTERVAL '7 days') RETURNING id`, siteID, keyHash).Scan(&id)
	return id, err
}

func (d *DB) PluginSite(ctx context.Context, keyHash string) (string, error) {
	var siteID string
	err := d.Pool.QueryRow(ctx, `SELECT site_id FROM plugin_keys WHERE key_hash=$1 AND revoked_at IS NULL`, keyHash).Scan(&siteID)
	return siteID, err
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
