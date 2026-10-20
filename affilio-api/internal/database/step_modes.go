package database

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

const (
	StepSiteSignup       = "site_signup"
	StepOfferMatch       = "offer_match"
	StepPlacementPublish = "placement_publish"
	StepLinkChange       = "link_change"
	StepPayout           = "payout"
	ModeAuto             = "auto"
	ModeAsk              = "ask"
	ModeOff              = "off"
)

// StepModes is the owner's choice for each agent step.
// An empty field on a missing row reads as auto, which is what sites created
// before D-088 still do. A new site stores placement publish as ask.
type StepModes struct {
	SiteSignup       string
	OfferMatch       string
	PlacementPublish string
	LinkChange       string
	Payout           string
}

func (m StepModes) Get(step string) string {
	var mode string
	switch step {
	case StepSiteSignup:
		mode = m.SiteSignup
	case StepOfferMatch:
		mode = m.OfferMatch
	case StepPlacementPublish:
		mode = m.PlacementPublish
	case StepLinkChange:
		mode = m.LinkChange
	case StepPayout:
		mode = m.Payout
	}
	if mode == "" {
		return ModeAuto
	}
	return mode
}

func (m *StepModes) Set(step, mode string) bool {
	switch step {
	case StepSiteSignup:
		m.SiteSignup = mode
	case StepOfferMatch:
		m.OfferMatch = mode
	case StepPlacementPublish:
		m.PlacementPublish = mode
	case StepLinkChange:
		m.LinkChange = mode
	case StepPayout:
		m.Payout = mode
	default:
		return false
	}
	return true
}

func (m StepModes) filled() StepModes {
	if m.SiteSignup == "" {
		m.SiteSignup = ModeAuto
	}
	if m.OfferMatch == "" {
		m.OfferMatch = ModeAuto
	}
	if m.PlacementPublish == "" {
		m.PlacementPublish = ModeAuto
	}
	if m.LinkChange == "" {
		m.LinkChange = ModeAuto
	}
	if m.Payout == "" {
		m.Payout = ModeAuto
	}
	return m
}

func ValidStepMode(mode string) bool {
	return mode == ModeAuto || mode == ModeAsk || mode == ModeOff
}

func stepScopeKey(ownerID, siteID string) (string, bool) {
	switch {
	case ownerID != "" && siteID != "":
		return ownerID + ":" + siteID, true
	case ownerID != "":
		return "owner:" + ownerID, true
	case siteID != "":
		return "site:" + siteID, true
	default:
		return "", false
	}
}

// LoadStepModes returns the saved modes for this owner and site.
// A site row wins over an owner-wide row. No row means every step is auto.
// New site creation writes placement publish as ask, so a missing row is only
// a site that never received that write.
func (d *DB) LoadStepModes(ctx context.Context, ownerID, siteID string) (StepModes, error) {
	if ownerID == "" && siteID == "" {
		return StepModes{}.filled(), nil
	}
	var m StepModes
	err := d.Pool.QueryRow(ctx, `
SELECT site_signup, offer_match, placement_publish, link_change, payout
FROM agent_step_modes
WHERE (NULLIF($2,'') IS NOT NULL AND site_id = NULLIF($2,'')::uuid)
   OR (NULLIF($1,'') IS NOT NULL AND owner_id = NULLIF($1,'')::uuid AND site_id IS NULL)
ORDER BY CASE
  WHEN site_id IS NOT NULL AND owner_id IS NOT NULL THEN 0
  WHEN site_id IS NOT NULL THEN 1
  ELSE 2 END
LIMIT 1`, ownerID, siteID).Scan(&m.SiteSignup, &m.OfferMatch, &m.PlacementPublish, &m.LinkChange, &m.Payout)
	if errors.Is(err, pgx.ErrNoRows) {
		return StepModes{}.filled(), nil
	}
	if err != nil {
		return StepModes{}, err
	}
	return m.filled(), nil
}

// seedNewSitePublishAsk stores placement publish as ask for a site that was
// just created. The other steps stay auto. An existing row is left alone, and
// an owner-wide row is left alone, so a site that already chose its modes
// keeps them. A missing row still reads as auto.
func seedNewSitePublishAsk(ctx context.Context, tx pgx.Tx, ownerID, siteID string) error {
	if siteID == "" {
		return errors.New("site required")
	}
	key, ok := stepScopeKey(ownerID, siteID)
	if !ok {
		return errors.New("site required")
	}
	_, err := tx.Exec(ctx, `
INSERT INTO agent_step_modes(scope_key, owner_id, site_id, site_signup, offer_match, placement_publish, link_change, payout)
SELECT $1, NULLIF($2,'')::uuid, $3::uuid, 'auto', 'auto', 'ask', 'auto', 'auto'
WHERE NOT EXISTS (
  SELECT 1 FROM agent_step_modes
  WHERE scope_key = $1
     OR (NULLIF($2,'') IS NOT NULL AND owner_id = NULLIF($2,'')::uuid AND site_id IS NULL)
)
ON CONFLICT (scope_key) DO NOTHING`, key, ownerID, siteID)
	return err
}

// SaveStepModes upserts one owner/site row. It does not rewrite other rows.
func (d *DB) SaveStepModes(ctx context.Context, ownerID, siteID string, modes StepModes) error {
	key, ok := stepScopeKey(ownerID, siteID)
	if !ok {
		return errors.New("owner or site required")
	}
	modes = modes.filled()
	_, err := d.Pool.Exec(ctx, `
INSERT INTO agent_step_modes(scope_key, owner_id, site_id, site_signup, offer_match, placement_publish, link_change, payout)
VALUES($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8)
ON CONFLICT (scope_key) DO UPDATE SET
  site_signup = EXCLUDED.site_signup,
  offer_match = EXCLUDED.offer_match,
  placement_publish = EXCLUDED.placement_publish,
  link_change = EXCLUDED.link_change,
  payout = EXCLUDED.payout,
  updated_at = NOW()`, key, ownerID, siteID, modes.SiteSignup, modes.OfferMatch, modes.PlacementPublish, modes.LinkChange, modes.Payout)
	return err
}

// SiteVerified reports whether D-063b has set verified_at.
func (d *DB) SiteVerified(ctx context.Context, siteID string) (bool, error) {
	if siteID == "" {
		return false, nil
	}
	var verified bool
	err := d.Pool.QueryRow(ctx, `SELECT verified_at IS NOT NULL FROM sites WHERE id=$1`, siteID).Scan(&verified)
	return verified, err
}

// OwnerRef is an owner and the site linked on that row, if any.
type OwnerRef struct {
	ID     string
	SiteID string
}

// OwnersByEmail returns every owner for an email. Email is not unique.
func (d *DB) OwnersByEmail(ctx context.Context, email string) ([]OwnerRef, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, COALESCE(site_id::text,'') FROM owners WHERE lower(email)=lower($1) ORDER BY id`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OwnerRef{}
	for rows.Next() {
		var owner OwnerRef
		if err = rows.Scan(&owner.ID, &owner.SiteID); err != nil {
			return nil, err
		}
		out = append(out, owner)
	}
	return out, rows.Err()
}

// StepApproval is one ask that is waiting on the operator queue.
type StepApproval struct {
	ID      string
	Step    string
	OwnerID string
	SiteID  string
	Status  string
	Payload json.RawMessage
	Result  json.RawMessage
}

func (d *DB) InsertStepApproval(ctx context.Context, step, ownerID, siteID string, payload []byte) (string, error) {
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO agent_step_approvals(step, owner_id, site_id, payload)
VALUES($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4::jsonb)
RETURNING id::text`, step, ownerID, siteID, payload).Scan(&id)
	return id, err
}

func (d *DB) ListPendingStepApprovals(ctx context.Context, limit int) ([]StepApproval, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id::text, step, COALESCE(owner_id::text,''), COALESCE(site_id::text,''), status, payload
FROM agent_step_approvals
WHERE status='pending'
ORDER BY created_at
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StepApproval{}
	for rows.Next() {
		var row StepApproval
		if err = rows.Scan(&row.ID, &row.Step, &row.OwnerID, &row.SiteID, &row.Status, &row.Payload); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (d *DB) GetStepApproval(ctx context.Context, id string) (StepApproval, error) {
	var row StepApproval
	err := d.Pool.QueryRow(ctx, `
SELECT id::text, step, COALESCE(owner_id::text,''), COALESCE(site_id::text,''), status, payload, result
FROM agent_step_approvals WHERE id=$1`, id).Scan(&row.ID, &row.Step, &row.OwnerID, &row.SiteID, &row.Status, &row.Payload, &row.Result)
	return row, err
}

// PayoutMode is the payout step for a site, read on the caller's connection.
// No row means auto.
func PayoutMode(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, siteID string) (string, error) {
	var mode string
	err := q.QueryRow(ctx, `
SELECT payout FROM agent_step_modes
WHERE site_id = $1::uuid
   OR (owner_id IN (SELECT id FROM owners WHERE site_id = $1::uuid) AND site_id IS NULL)
ORDER BY CASE WHEN site_id IS NOT NULL THEN 0 ELSE 1 END
LIMIT 1`, siteID).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) || mode == "" {
		return ModeAuto, nil
	}
	return mode, err
}

// QueuePayoutBatch records one ask for a batch payout and does not pay it.
// A pending row for the same site, currency, and payment mode is left as it is.
func QueuePayoutBatch(ctx context.Context, tx pgx.Tx, siteID, currency, makerID, paymentMode string, amount float64) error {
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM agent_step_approvals
  WHERE step='payout' AND site_id=$1 AND status='pending'
    AND payload->>'action'='batch' AND payload->>'currency'=$2 AND payload->>'payment_mode'=$3
)`, siteID, currency, paymentMode).Scan(&exists)
	if err != nil || exists {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"action": "batch", "site_id": siteID, "currency": currency,
		"maker_id": makerID, "payment_mode": paymentMode, "amount": amount,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO agent_step_approvals(step, owner_id, site_id, payload)
VALUES('payout', (SELECT id FROM owners WHERE site_id=$1 LIMIT 1), $1, $2::jsonb)`, siteID, payload)
	return err
}

// FulfillBatchPayout pays the cleared ledger for one site once an ask is approved.
// A ledger that was already stamped is left alone.
func (d *DB) FulfillBatchPayout(ctx context.Context, siteID, currency, makerID, paymentMode string) (string, bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockMoney(ctx, tx); err != nil {
		return "", false, err
	}
	var demo bool
	err = tx.QueryRow(ctx, `SELECT demo FROM sites WHERE id=$1`, siteID).Scan(&demo)
	if errors.Is(err, pgx.ErrNoRows) || demo {
		return "", false, tx.Commit(ctx)
	}
	if err != nil {
		return "", false, err
	}
	var amount float64
	err = tx.QueryRow(ctx, `
SELECT COALESCE(SUM(le.amount),0)
FROM ledger_entries le
JOIN conversions cv ON cv.id=le.conversion_id
JOIN offers o ON o.id=cv.offer_id
JOIN products p ON p.id=o.product_id
WHERE le.status='cleared' AND le.payout_id IS NULL AND le.site_id=$1
  AND le.currency=$2 AND p.maker_id=$3 AND cv.payment_mode=$4
  AND NOT EXISTS (SELECT 1 FROM clicks cl WHERE cl.id = cv.click_id AND cl.click_class = 'test')`, siteID, currency, makerID, paymentMode).Scan(&amount)
	if err != nil {
		return "", false, err
	}
	if amount <= 0 {
		return "", false, tx.Commit(ctx)
	}
	var payoutID string
	err = tx.QueryRow(ctx, `
INSERT INTO payouts (site_id, amount, currency, method, status, maker_id, payment_mode)
VALUES ($1,$2,$3,'manual','pending',$4,$5)
RETURNING id::text`, siteID, amount, currency, makerID, paymentMode).Scan(&payoutID)
	if err != nil {
		return "", false, err
	}
	tag, err := tx.Exec(ctx, `
UPDATE ledger_entries SET payout_id=$1
WHERE site_id=$2 AND currency=$3 AND status='cleared' AND payout_id IS NULL
AND conversion_id IN (
  SELECT cv.id FROM conversions cv
  JOIN offers o ON o.id=cv.offer_id
  JOIN products p ON p.id=o.product_id
  WHERE p.maker_id=$4 AND cv.payment_mode=$5
  AND NOT EXISTS (SELECT 1 FROM clicks cl WHERE cl.id = cv.click_id AND cl.click_class = 'test')
)`, payoutID, siteID, currency, makerID, paymentMode)
	if err != nil {
		return "", false, err
	}
	if tag.RowsAffected() == 0 {
		return "", false, tx.Rollback(ctx)
	}
	return payoutID, true, tx.Commit(ctx)
}

func (d *DB) FinishStepApproval(ctx context.Context, id string, result []byte) (bool, error) {
	if len(result) == 0 {
		result = []byte(`{}`)
	}
	tag, err := d.Pool.Exec(ctx, `
UPDATE agent_step_approvals
SET status='approved', result=$2::jsonb, decided_at=NOW()
WHERE id=$1 AND status='pending'`, id, result)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
