package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	DeviceTTL         = 5 * time.Minute
	DeviceInterval    = 5
	ProjectKeyTTL     = 8 * time.Hour
	deviceSlowStep    = 5
	deviceSlowCeiling = 60
)

var (
	ErrDeviceNotFound = errors.New("device code not found")
	ErrDeviceExpired  = errors.New("expired")
	ErrDeviceUsed     = errors.New("used")
	ErrProjectScope   = errors.New("project is not on this account")
)

type DevicePoll struct {
	Error          string
	Interval       int
	ExpiresIn      int
	AccessToken    string
	Scope          string
	KeyHeader      string
	TokenExpiresIn int
}

type SetupSettings struct {
	OwnerID       string
	Role          string
	InstallPath   string
	Mode          string
	ModeShown     string
	Promote       string
	AvoidTopics   []string
	Commission    string
	SitesChannels []string
	NeverTopics   []string
	BoxPosition   string
	SavedBy       string
	SavedAt       time.Time
}

type ApprovalPolicy struct {
	Scope                  string
	SiteID                 string
	OfferID                string
	Mode                   string
	Version                int
	AllowMakers            []string
	DenyMakers             []string
	AllowCategories        []string
	DenyCategories         []string
	MinRelevance           int
	AllowedPositions       []string
	MaxPlacementsPer30Days int
	CopyEditsAllowed       bool
	MinSiteQuality         int
	AllowedNiches          []string
	KillSwitch             bool
	ModeBeforeKill         string
}

type ListingCardRow struct {
	ID              string
	MakerID         string
	OwnerID         string
	ProductID       string
	Card            []byte
	Signals         []byte
	Score           int
	PaymentsCheck   string
	OfferLinkHealth string
	CreatedAt       time.Time
}

type DigestItem struct {
	Kind       string
	ID         string
	Summary    string
	WentLiveAt time.Time
}

func textList(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func (d *DB) CreateDeviceAuthorization(ctx context.Context, deviceHash, userHash string, expires time.Time) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO device_authorizations(device_code_hash,user_code_hash,interval_seconds,expires_at)
VALUES($1,$2,$3,$4)`, deviceHash, userHash, DeviceInterval, expires)
	return err
}

func (d *DB) DecideDevice(ctx context.Context, userHash, ownerID, decision, scope, projectID string, now time.Time) (time.Time, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var status, id string
	var expires time.Time
	err = tx.QueryRow(ctx, `SELECT id, status, expires_at FROM device_authorizations WHERE user_code_hash=$1 FOR UPDATE`, userHash).Scan(&id, &status, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrDeviceNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	if !now.Before(expires) {
		return time.Time{}, ErrDeviceExpired
	}
	if status != "pending" {
		return time.Time{}, ErrDeviceUsed
	}
	var loginAt time.Time
	if decision == "deny" {
		if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET status='denied', denied_at=$2, owner_id=$3 WHERE id=$1`, id, now, ownerID); err != nil {
			return time.Time{}, err
		}
		return time.Time{}, tx.Commit(ctx)
	}
	var siteID, offerID, makerID *string
	switch scope {
	case "site":
		var owned string
		err = tx.QueryRow(ctx, `SELECT COALESCE(site_id::text,'') FROM owners WHERE id=$1`, ownerID).Scan(&owned)
		if err != nil {
			return time.Time{}, err
		}
		if owned == "" || owned != projectID {
			return time.Time{}, ErrProjectScope
		}
		siteID = &projectID
	case "offer":
		var ownedMaker, offerMaker string
		err = tx.QueryRow(ctx, `SELECT COALESCE(maker_id::text,'') FROM owners WHERE id=$1`, ownerID).Scan(&ownedMaker)
		if err != nil {
			return time.Time{}, err
		}
		err = tx.QueryRow(ctx, `SELECT p.maker_id::text FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, projectID).Scan(&offerMaker)
		if errors.Is(err, pgx.ErrNoRows) || ownedMaker == "" || ownedMaker != offerMaker {
			return time.Time{}, ErrProjectScope
		}
		if err != nil {
			return time.Time{}, err
		}
		offerID = &projectID
		makerID = &offerMaker
	default:
		return time.Time{}, ErrProjectScope
	}
	_, err = tx.Exec(ctx, `UPDATE device_authorizations
SET status='approved', approved_at=$2, owner_id=$3, scope=$4, site_id=$5, offer_id=$6, maker_id=$7
WHERE id=$1`, id, now, ownerID, scope, siteID, offerID, makerID)
	if err != nil {
		return time.Time{}, err
	}
	err = tx.QueryRow(ctx, `UPDATE owners SET human_login=true, human_login_at=COALESCE(human_login_at, $2) WHERE id=$1 RETURNING human_login_at`, ownerID, now).Scan(&loginAt)
	if err != nil {
		return time.Time{}, err
	}
	return loginAt, tx.Commit(ctx)
}

func (d *DB) PollDevice(ctx context.Context, deviceHash, keyPlain, keyHash, keyPrefix string, now time.Time) (DevicePoll, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return DevicePoll{}, err
	}
	defer tx.Rollback(ctx)
	var id, status, scope, siteID, offerID, makerID, ownerID string
	var interval int
	var expires time.Time
	var last *time.Time
	err = tx.QueryRow(ctx, `SELECT id, status, interval_seconds, expires_at, last_polled_at,
COALESCE(scope,''), COALESCE(site_id::text,''), COALESCE(offer_id::text,''), COALESCE(maker_id::text,''), COALESCE(owner_id::text,'')
FROM device_authorizations WHERE device_code_hash=$1 FOR UPDATE`, deviceHash).Scan(&id, &status, &interval, &expires, &last, &scope, &siteID, &offerID, &makerID, &ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DevicePoll{Error: "expired"}, nil
	}
	if err != nil {
		return DevicePoll{}, err
	}
	if status == "consumed" || !now.Before(expires) {
		return DevicePoll{Error: "expired", Interval: interval}, nil
	}
	if last != nil && now.Sub(*last) < time.Duration(interval)*time.Second {
		next := interval + deviceSlowStep
		if next > deviceSlowCeiling {
			next = deviceSlowCeiling
		}
		if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET interval_seconds=$2, last_polled_at=$3 WHERE id=$1`, id, next, now); err != nil {
			return DevicePoll{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return DevicePoll{}, err
		}
		return DevicePoll{Error: "slow_down", Interval: next, ExpiresIn: expiresIn(expires, now)}, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET last_polled_at=$2 WHERE id=$1`, id, now); err != nil {
		return DevicePoll{}, err
	}
	out := DevicePoll{Interval: interval, ExpiresIn: expiresIn(expires, now)}
	switch status {
	case "pending":
		out.Error = "authorization_pending"
	case "denied":
		out.Error = "access_denied"
	case "approved":
		keyExpires := now.Add(ProjectKeyTTL)
		var siteArg, offerArg, makerArg any
		header := "X-API-Key"
		tokenScope := "site:" + siteID
		if scope == "offer" {
			header = "X-Maker-Key"
			tokenScope = "offer:" + offerID
			offerArg, makerArg = offerID, makerID
		} else {
			siteArg = siteID
		}
		_, err = tx.Exec(ctx, `INSERT INTO project_keys(key_hash,key_prefix,owner_id,scope,site_id,offer_id,maker_id,expires_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, keyHash, keyPrefix, ownerID, scope, siteArg, offerArg, makerArg, keyExpires)
		if err != nil {
			return DevicePoll{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET status='consumed', consumed_at=$2 WHERE id=$1`, id, now); err != nil {
			return DevicePoll{}, err
		}
		out.AccessToken = keyPlain
		out.Scope = tokenScope
		out.KeyHeader = header
		out.TokenExpiresIn = int(ProjectKeyTTL.Seconds())
	default:
		out.Error = "expired"
	}
	if err = tx.Commit(ctx); err != nil {
		return DevicePoll{}, err
	}
	return out, nil
}

func expiresIn(expires, now time.Time) int {
	secs := int(expires.Sub(now).Seconds())
	if secs < 0 {
		return 0
	}
	return secs
}

func (d *DB) ProjectSiteKey(ctx context.Context, keyHash string) (string, error) {
	var siteID string
	err := d.Pool.QueryRow(ctx, `SELECT site_id::text FROM project_keys
WHERE key_hash=$1 AND scope='site' AND revoked_at IS NULL AND expires_at>NOW()`, keyHash).Scan(&siteID)
	return siteID, err
}

func (d *DB) ProjectOfferKey(ctx context.Context, keyHash string) (string, string, error) {
	var makerID, offerID string
	err := d.Pool.QueryRow(ctx, `SELECT maker_id::text, offer_id::text FROM project_keys
WHERE key_hash=$1 AND scope='offer' AND revoked_at IS NULL AND expires_at>NOW()`, keyHash).Scan(&makerID, &offerID)
	return makerID, offerID, err
}

type AgentAccount struct {
	OwnerID string
	SiteID  string
	MakerID string
}

func (d *DB) CreateAgentOnlyAccount(ctx context.Context, subject, email, name, source, role, domain, siteHash, sitePrefix, makerKey string) (AgentAccount, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return AgentAccount{}, err
	}
	defer tx.Rollback(ctx)
	var out AgentAccount
	err = tx.QueryRow(ctx, `INSERT INTO owners(issuer,subject,email,name,source,agent_only,human_login)
VALUES('https://offer-rails.agent',$1,$2,$3,$4,true,false) RETURNING id`, subject, email, name, source).Scan(&out.OwnerID)
	if err != nil {
		return AgentAccount{}, err
	}
	switch role {
	case "publisher":
		err = tx.QueryRow(ctx, `INSERT INTO sites(api_key_hash,api_key_prefix,domain,name) VALUES($1,$2,$3,$4) RETURNING id`, siteHash, sitePrefix, domain, name).Scan(&out.SiteID)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1`, out.OwnerID, out.SiteID)
		}
	case "maker":
		err = tx.QueryRow(ctx, `INSERT INTO makers(api_key,email,name) VALUES($1,$2,$3) RETURNING id`, makerKey, email, name).Scan(&out.MakerID)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE owners SET maker_id=$2 WHERE id=$1`, out.OwnerID, out.MakerID)
		}
	default:
		return AgentAccount{}, errors.New("invalid account role")
	}
	if err != nil {
		return AgentAccount{}, err
	}
	return out, tx.Commit(ctx)
}

type AccountSignals struct {
	HumanIDs []string
	AgentIDs []string
}

func (d *DB) AccountSignals(ctx context.Context) (AccountSignals, error) {
	var out AccountSignals
	human, err := d.ownerIDs(ctx, `SELECT id::text FROM owners WHERE human_login ORDER BY human_login_at`)
	if err != nil {
		return out, err
	}
	agent, err := d.ownerIDs(ctx, `SELECT id::text FROM owners WHERE agent_only AND NOT human_login ORDER BY id`)
	if err != nil {
		return out, err
	}
	out.HumanIDs, out.AgentIDs = human, agent
	return out, nil
}

func (d *DB) ownerIDs(ctx context.Context, query string) ([]string, error) {
	rows, err := d.Pool.Query(ctx, query)
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

func (d *DB) SaveSetup(ctx context.Context, s SetupSettings) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO owner_setup(owner_id,role,install_path,mode,mode_shown,promote_text,avoid_topics,commission,sites_channels,never_topics,box_position,saved_by,saved_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT(owner_id,role) DO UPDATE SET install_path=EXCLUDED.install_path, mode=EXCLUDED.mode, mode_shown=EXCLUDED.mode_shown,
promote_text=EXCLUDED.promote_text, avoid_topics=EXCLUDED.avoid_topics, commission=EXCLUDED.commission,
sites_channels=EXCLUDED.sites_channels, never_topics=EXCLUDED.never_topics, box_position=EXCLUDED.box_position,
saved_by=EXCLUDED.saved_by, saved_at=EXCLUDED.saved_at`,
		s.OwnerID, s.Role, s.InstallPath, s.Mode, s.ModeShown, nullString(s.Promote), textList(s.AvoidTopics), nullString(s.Commission), textList(s.SitesChannels), textList(s.NeverTopics), nullString(s.BoxPosition), s.SavedBy, s.SavedAt)
	return err
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (d *DB) GetSetup(ctx context.Context, ownerID, role string) (SetupSettings, error) {
	var s SetupSettings
	var promote, commission, box *string
	err := d.Pool.QueryRow(ctx, `SELECT owner_id::text, role, install_path, mode, mode_shown, promote_text, avoid_topics, commission, sites_channels, never_topics, box_position, saved_by::text, saved_at
FROM owner_setup WHERE owner_id=$1 AND role=$2`, ownerID, role).Scan(&s.OwnerID, &s.Role, &s.InstallPath, &s.Mode, &s.ModeShown, &promote, &s.AvoidTopics, &commission, &s.SitesChannels, &s.NeverTopics, &box, &s.SavedBy, &s.SavedAt)
	if promote != nil {
		s.Promote = *promote
	}
	if commission != nil {
		s.Commission = *commission
	}
	if box != nil {
		s.BoxPosition = *box
	}
	return s, err
}

func (d *DB) OwnerIDBySite(ctx context.Context, siteID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT id::text FROM owners WHERE site_id=$1`, siteID).Scan(&id)
	return id, err
}

func (d *DB) OwnerIDByMaker(ctx context.Context, makerID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT id::text FROM owners WHERE maker_id=$1`, makerID).Scan(&id)
	return id, err
}

func scanPolicy(row interface{ Scan(...any) error }) (ApprovalPolicy, error) {
	var p ApprovalPolicy
	var site, offer, before *string
	err := row.Scan(&p.Scope, &site, &offer, &p.Mode, &p.Version, &p.AllowMakers, &p.DenyMakers, &p.AllowCategories, &p.DenyCategories, &p.MinRelevance, &p.AllowedPositions, &p.MaxPlacementsPer30Days, &p.CopyEditsAllowed, &p.MinSiteQuality, &p.AllowedNiches, &p.KillSwitch, &before)
	if site != nil {
		p.SiteID = *site
	}
	if offer != nil {
		p.OfferID = *offer
	}
	if before != nil {
		p.ModeBeforeKill = *before
	}
	return p, err
}

const policySelect = `scope, site_id::text, offer_id::text, mode, version, allow_makers, deny_makers, allow_categories, deny_categories, min_relevance, allowed_positions, max_placements_per_30d, copy_edits_allowed, min_site_quality, allowed_niches, kill_switch, mode_before_kill`

func (d *DB) GetSitePolicy(ctx context.Context, siteID string) (ApprovalPolicy, error) {
	return scanPolicy(d.Pool.QueryRow(ctx, `SELECT `+policySelect+` FROM approval_policies WHERE site_id=$1`, siteID))
}

func (d *DB) GetOfferPolicy(ctx context.Context, offerID string) (ApprovalPolicy, error) {
	return scanPolicy(d.Pool.QueryRow(ctx, `SELECT `+policySelect+` FROM approval_policies WHERE offer_id=$1`, offerID))
}

func (d *DB) UpsertSitePolicy(ctx context.Context, siteID string, p ApprovalPolicy, actor, userID, fromMode string) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var version int
	err = tx.QueryRow(ctx, `INSERT INTO approval_policies(scope,site_id,mode,version,allow_makers,deny_makers,allow_categories,deny_categories,min_relevance,allowed_positions,max_placements_per_30d,copy_edits_allowed,min_site_quality,allowed_niches)
VALUES('site',$1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT(site_id) DO UPDATE SET mode=EXCLUDED.mode, version=approval_policies.version+1,
allow_makers=EXCLUDED.allow_makers, deny_makers=EXCLUDED.deny_makers, allow_categories=EXCLUDED.allow_categories, deny_categories=EXCLUDED.deny_categories,
min_relevance=EXCLUDED.min_relevance, allowed_positions=EXCLUDED.allowed_positions, max_placements_per_30d=EXCLUDED.max_placements_per_30d,
copy_edits_allowed=EXCLUDED.copy_edits_allowed, min_site_quality=EXCLUDED.min_site_quality, allowed_niches=EXCLUDED.allowed_niches,
kill_switch=CASE WHEN $13='owner_ui' THEN false ELSE approval_policies.kill_switch END,
mode_before_kill=CASE WHEN $13='owner_ui' THEN NULL ELSE approval_policies.mode_before_kill END,
updated_at=NOW()
RETURNING version`, siteID, p.Mode, textList(p.AllowMakers), textList(p.DenyMakers), textList(p.AllowCategories), textList(p.DenyCategories), p.MinRelevance, textList(p.AllowedPositions), p.MaxPlacementsPer30Days, p.CopyEditsAllowed, p.MinSiteQuality, textList(p.AllowedNiches), actor).Scan(&version)
	if err != nil {
		return 0, err
	}
	if fromMode != p.Mode {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(site_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),$5)`, siteID, actor, userID, fromMode, p.Mode); err != nil {
			return 0, err
		}
	}
	return version, tx.Commit(ctx)
}

func (d *DB) UpsertOfferPolicy(ctx context.Context, offerID string, p ApprovalPolicy, actor, userID, fromMode string) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var version int
	err = tx.QueryRow(ctx, `INSERT INTO approval_policies(scope,offer_id,mode,version,allow_makers,deny_makers,allow_categories,deny_categories,min_relevance,allowed_positions,max_placements_per_30d,copy_edits_allowed,min_site_quality,allowed_niches)
VALUES('offer',$1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT(offer_id) DO UPDATE SET mode=EXCLUDED.mode, version=approval_policies.version+1,
allow_makers=EXCLUDED.allow_makers, deny_makers=EXCLUDED.deny_makers, allow_categories=EXCLUDED.allow_categories, deny_categories=EXCLUDED.deny_categories,
min_relevance=EXCLUDED.min_relevance, allowed_positions=EXCLUDED.allowed_positions, max_placements_per_30d=EXCLUDED.max_placements_per_30d,
copy_edits_allowed=EXCLUDED.copy_edits_allowed, min_site_quality=EXCLUDED.min_site_quality, allowed_niches=EXCLUDED.allowed_niches,
kill_switch=CASE WHEN $13='owner_ui' THEN false ELSE approval_policies.kill_switch END,
mode_before_kill=CASE WHEN $13='owner_ui' THEN NULL ELSE approval_policies.mode_before_kill END,
updated_at=NOW()
RETURNING version`, offerID, p.Mode, textList(p.AllowMakers), textList(p.DenyMakers), textList(p.AllowCategories), textList(p.DenyCategories), p.MinRelevance, textList(p.AllowedPositions), p.MaxPlacementsPer30Days, p.CopyEditsAllowed, p.MinSiteQuality, textList(p.AllowedNiches), actor).Scan(&version)
	if err != nil {
		return 0, err
	}
	if fromMode != p.Mode {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(offer_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),$5)`, offerID, actor, userID, fromMode, p.Mode); err != nil {
			return 0, err
		}
	}
	return version, tx.Commit(ctx)
}

func (d *DB) KillSiteSwitch(ctx context.Context, siteID, actor, userID, fromMode string) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals SET auto_paused=true
WHERE site_id=$1 AND status='approved' AND approved_by_kind IN ('policy:auto','policy:yolo') AND auto_paused=false`, siteID)
	if err != nil {
		return 0, err
	}
	var version int
	err = tx.QueryRow(ctx, `INSERT INTO approval_policies(scope,site_id,mode,kill_switch,mode_before_kill)
VALUES('site',$1,'manual',true,NULLIF($2,''))
ON CONFLICT(site_id) DO UPDATE SET
mode_before_kill=CASE WHEN approval_policies.kill_switch THEN approval_policies.mode_before_kill ELSE NULLIF($2,'') END,
kill_switch=true, mode='manual', version=approval_policies.version+1, updated_at=NOW()
RETURNING version`, siteID, fromMode).Scan(&version)
	if err != nil || version < 1 {
		if err == nil {
			err = errors.New("kill switch was not saved")
		}
		return 0, err
	}
	if fromMode != "manual" {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(site_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),'manual')`, siteID, actor, userID, fromMode); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ReleaseSiteKillSwitch clears the kill switch for a signed-in human, restores
// the mode it replaced, and unpauses the approvals it held.
func (d *DB) ReleaseSiteKillSwitch(ctx context.Context, siteID, actor, userID string) (string, int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals SET auto_paused=false
WHERE site_id=$1 AND status='approved' AND approved_by_kind IN ('policy:auto','policy:yolo') AND auto_paused=true`, siteID)
	if err != nil {
		return "", 0, err
	}
	var mode, before string
	var on bool
	err = tx.QueryRow(ctx, `SELECT mode, kill_switch, COALESCE(mode_before_kill,'') FROM approval_policies WHERE site_id=$1 FOR UPDATE`, siteID).Scan(&mode, &on, &before)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return "", 0, err
		}
		return "manual", int(tag.RowsAffected()), nil
	}
	if err != nil {
		return "", 0, err
	}
	restored := mode
	if on && before != "" {
		restored = before
	}
	if _, err = tx.Exec(ctx, `UPDATE approval_policies SET mode=$2, kill_switch=false, mode_before_kill=NULL, version=version+1, updated_at=NOW() WHERE site_id=$1`, siteID, restored); err != nil {
		return "", 0, err
	}
	if restored != mode {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(site_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5)`, siteID, actor, userID, mode, restored); err != nil {
			return "", 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return restored, int(tag.RowsAffected()), nil
}

func (d *DB) MakerOfferIDs(ctx context.Context, makerID string) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `SELECT o.id::text FROM offers o JOIN products p ON p.id=o.product_id WHERE p.maker_id=$1`, makerID)
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

func (d *DB) OfferFacts(ctx context.Context, offerID string) (live bool, makerID, category string, err error) {
	var status string
	err = d.Pool.QueryRow(ctx, `SELECT o.status, p.maker_id::text, COALESCE(p.primary_category,'')
FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(&status, &makerID, &category)
	return status == "listed", makerID, category, err
}

func (d *DB) CountPlacementBoxes(ctx context.Context, siteID, surfaceURL, exceptID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND surface_url=$2 AND id<>$3 AND status<>'revoked' AND removed_at IS NULL`, siteID, surfaceURL, exceptID).Scan(&n)
	return n, err
}

func (d *DB) CountPolicyPlacements(ctx context.Context, siteID string, since time.Time) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND created_at>$2 AND approved_by_kind IN ('policy:auto','policy:yolo') AND status<>'revoked'`, siteID, since).Scan(&n)
	return n, err
}

func (d *DB) RecordPolicyApproval(ctx context.Context, placementID, siteID, ownerID, kind, rule string, version int) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='approved', approved_at=NOW(),
approved_by=NULLIF($3,'')::uuid, approved_by_kind=$4, policy_version=$5, matched_rule=$6
WHERE id=$1 AND site_id=$2 AND status='pending' AND expires_at>NOW() AND auto_paused=false`, placementID, siteID, ownerID, kind, version, rule)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (d *DB) RecordPublisherAcceptance(ctx context.Context, offerID, siteID, kind, rule string, version int) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO publisher_acceptances(offer_id,site_id,approved_by_kind,policy_version,matched_rule)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT(offer_id,site_id) DO UPDATE SET approved_by_kind=EXCLUDED.approved_by_kind, policy_version=EXCLUDED.policy_version, matched_rule=EXCLUDED.matched_rule, created_at=NOW()`, offerID, siteID, kind, version, rule)
	return err
}

func (d *DB) InsertListingCard(ctx context.Context, row ListingCardRow) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO listing_cards(maker_id,owner_id,product_id,card,signals,score,payments_check,offer_link_health)
VALUES(NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4::jsonb, $5::jsonb, $6, $7, $8) RETURNING id`,
		row.MakerID, row.OwnerID, row.ProductID, row.Card, row.Signals, row.Score, row.PaymentsCheck, row.OfferLinkHealth).Scan(&id)
	return id, err
}

func (d *DB) GetListingCard(ctx context.Context, id, makerID string) (ListingCardRow, error) {
	var row ListingCardRow
	var product, owner *string
	err := d.Pool.QueryRow(ctx, `SELECT id::text, COALESCE(maker_id::text,''), owner_id::text, product_id::text, card, signals, score, payments_check, offer_link_health, created_at
FROM listing_cards WHERE id=$1 AND maker_id=$2`, id, makerID).Scan(&row.ID, &row.MakerID, &owner, &product, &row.Card, &row.Signals, &row.Score, &row.PaymentsCheck, &row.OfferLinkHealth, &row.CreatedAt)
	if owner != nil {
		row.OwnerID = *owner
	}
	if product != nil {
		row.ProductID = *product
	}
	return row, err
}

func (d *DB) MakerOwnsProduct(ctx context.Context, makerID, productID string) (bool, error) {
	var ok bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND maker_id=$2)`, productID, makerID).Scan(&ok)
	return ok, err
}

func (d *DB) InsertOwnerEvent(ctx context.Context, kind, userID, placementID, offerID string, at time.Time) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,offer_id,created_at)
VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5)`, kind, userID, placementID, offerID, at)
	return err
}

func (d *DB) LastDigestOpen(ctx context.Context, userID string) (*time.Time, error) {
	var at *time.Time
	err := d.Pool.QueryRow(ctx, `SELECT MAX(created_at) FROM owner_events WHERE user_id=$1 AND kind='digest_opened'`, userID).Scan(&at)
	return at, err
}

func (d *DB) DigestPlacements(ctx context.Context, siteID string, since time.Time) ([]DigestItem, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, copy, COALESCE(approved_at, created_at)
FROM placement_proposals
WHERE site_id=$1 AND status IN ('approved','committed') AND COALESCE(approved_at, created_at) > $2
ORDER BY COALESCE(approved_at, created_at) DESC LIMIT 50`, siteID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigest(rows, "placement")
}

func (d *DB) DigestOffers(ctx context.Context, makerID string, since time.Time) ([]DigestItem, error) {
	rows, err := d.Pool.Query(ctx, `SELECT o.id::text, p.name, o.updated_at
FROM offers o JOIN products p ON p.id=o.product_id
WHERE p.maker_id=$1 AND o.status='listed' AND o.updated_at > $2
ORDER BY o.updated_at DESC LIMIT 50`, makerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigest(rows, "offer")
}

func scanDigest(rows pgx.Rows, kind string) ([]DigestItem, error) {
	out := []DigestItem{}
	for rows.Next() {
		var item DigestItem
		item.Kind = kind
		if err := rows.Scan(&item.ID, &item.Summary, &item.WentLiveAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) StoreUndoToken(ctx context.Context, hash, ownerID, kind, placementID, offerID string, expires time.Time) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO undo_tokens(token_hash,owner_id,kind,placement_id,offer_id,expires_at)
VALUES($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,$6)`, hash, ownerID, kind, placementID, offerID, expires)
	return err
}

func (d *DB) ConsumeUndo(ctx context.Context, hash, ownerID string, now time.Time) (kind, placementID, offerID string, err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `UPDATE undo_tokens SET used_at=$3
WHERE token_hash=$1 AND owner_id=$2 AND used_at IS NULL AND expires_at>$3
RETURNING kind, COALESCE(placement_id::text,''), COALESCE(offer_id::text,'')`, hash, ownerID, now).Scan(&kind, &placementID, &offerID)
	if err != nil {
		return "", "", "", err
	}
	switch kind {
	case "placement":
		if _, err = tx.Exec(ctx, `UPDATE placement_proposals SET status='revoked' WHERE id=$1 AND status<>'revoked'`, placementID); err != nil {
			return "", "", "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,created_at) VALUES('placement_undo',$1,$2,$3)`, ownerID, placementID, now)
	case "offer":
		if _, err = tx.Exec(ctx, `UPDATE offers SET status='paused', updated_at=$2 WHERE id=$1 AND status='listed'`, offerID, now); err != nil {
			return "", "", "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,offer_id,created_at) VALUES('offer_undo',$1,$2,$3)`, ownerID, offerID, now)
	default:
		return "", "", "", errors.New("unknown undo kind")
	}
	if err != nil {
		return "", "", "", err
	}
	return kind, placementID, offerID, tx.Commit(ctx)
}

func (d *DB) CountSourceHit(ctx context.Context, source string) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO source_hits(source,hits) VALUES($1,1)
ON CONFLICT(source) DO UPDATE SET hits=source_hits.hits+1, updated_at=NOW()`, source)
	return err
}

func (d *DB) SourceHits(ctx context.Context, source string) (int64, error) {
	var n int64
	err := d.Pool.QueryRow(ctx, `SELECT hits FROM source_hits WHERE source=$1`, source).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func (d *DB) LatestModeChange(ctx context.Context, siteID, offerID string) (actor, userID, toMode string, at time.Time, err error) {
	if siteID != "" {
		err = d.Pool.QueryRow(ctx, `SELECT actor, COALESCE(user_id::text,''), to_mode, created_at FROM mode_changes WHERE site_id=$1 ORDER BY created_at DESC LIMIT 1`, siteID).Scan(&actor, &userID, &toMode, &at)
		return
	}
	err = d.Pool.QueryRow(ctx, `SELECT actor, COALESCE(user_id::text,''), to_mode, created_at FROM mode_changes WHERE offer_id=$1 ORDER BY created_at DESC LIMIT 1`, offerID).Scan(&actor, &userID, &toMode, &at)
	return
}
