package database

import (
	"context"
	"errors"
	"strings"
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
	OwnerID              string
	Role                 string
	InstallPath          string
	Mode                 string
	ModeShown            string
	Promote              string
	AvoidTopics          []string
	Commission           string
	ProjectName          string
	ListingURL           string
	CommissionType       string
	CommissionValue      *float64
	CommissionCurrency   string
	ListingPriceValue    *float64
	ListingPriceCurrency string
	SitesChannels        []string
	NeverTopics          []string
	BoxPosition          string
	SavedBy              string
	SavedAt              time.Time
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
	Kind               string
	ID                 string
	Summary            string
	WentLiveAt         time.Time
	CountsAsLive       bool
	PluginWait         string
	PluginWaitAt       *time.Time
	PluginContactAt    *time.Time
	PluginFailedReason string
	FailedAction       string
	MatchedRule        string
	PostTitle          string
	SurfaceURL         string
	Position           string
	LinkText           string
	Label              string
	Clicks             int
	ApprovedBy         string
	PolicyVersion      int
	UndoneBy           string
	Status             string
	ReviewReason       string
	ReviewMessage      string
}

// DigestPause is the account, site, and offer pause state for one digest.
type DigestPause struct {
	Account bool
	Site    bool
	Offer   bool
}

// DigestPageSize is the item page. Counts are not limited by it.
var DigestPageSize = 50

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
	_, err := d.Pool.Exec(ctx, `INSERT INTO owner_setup(owner_id,role,install_path,mode,mode_shown,promote_text,avoid_topics,commission,sites_channels,never_topics,box_position,saved_by,saved_at,project_name,listing_url,commission_type,commission_value,commission_currency,listing_price_value,listing_price_currency)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
ON CONFLICT(owner_id,role) DO UPDATE SET install_path=EXCLUDED.install_path, mode=owner_setup.mode, mode_shown=EXCLUDED.mode_shown,
promote_text=EXCLUDED.promote_text, avoid_topics=EXCLUDED.avoid_topics, commission=EXCLUDED.commission,
sites_channels=EXCLUDED.sites_channels, never_topics=EXCLUDED.never_topics, box_position=EXCLUDED.box_position,
saved_by=EXCLUDED.saved_by, saved_at=EXCLUDED.saved_at,
project_name=EXCLUDED.project_name, listing_url=EXCLUDED.listing_url,
commission_type=EXCLUDED.commission_type, commission_value=EXCLUDED.commission_value, commission_currency=EXCLUDED.commission_currency,
listing_price_value=EXCLUDED.listing_price_value, listing_price_currency=EXCLUDED.listing_price_currency`,
		s.OwnerID, s.Role, s.InstallPath, s.Mode, s.ModeShown, nullString(s.Promote), textList(s.AvoidTopics), nullString(s.Commission), textList(s.SitesChannels), textList(s.NeverTopics), nullString(s.BoxPosition), s.SavedBy, s.SavedAt,
		s.ProjectName, s.ListingURL, s.CommissionType, s.CommissionValue, s.CommissionCurrency, s.ListingPriceValue, s.ListingPriceCurrency)
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
	err := d.Pool.QueryRow(ctx, `SELECT owner_id::text, role, install_path, mode, mode_shown, promote_text, avoid_topics, commission, sites_channels, never_topics, box_position, saved_by::text, saved_at,
project_name, listing_url, commission_type, commission_value, commission_currency, listing_price_value, listing_price_currency
FROM owner_setup WHERE owner_id=$1 AND role=$2`, ownerID, role).Scan(&s.OwnerID, &s.Role, &s.InstallPath, &s.Mode, &s.ModeShown, &promote, &s.AvoidTopics, &commission, &s.SitesChannels, &s.NeverTopics, &box, &s.SavedBy, &s.SavedAt,
		&s.ProjectName, &s.ListingURL, &s.CommissionType, &s.CommissionValue, &s.CommissionCurrency, &s.ListingPriceValue, &s.ListingPriceCurrency)
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

func (d *DB) OwnerHumanLogin(ctx context.Context, ownerID string) (bool, error) {
	var human bool
	err := d.Pool.QueryRow(ctx, `SELECT human_login FROM owners WHERE id=$1`, ownerID).Scan(&human)
	return human, err
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

func (d *DB) KillSiteSwitch(ctx context.Context, siteID, actor, userID, fromMode string, at time.Time) (int, error) {
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
	if _, err = tx.Exec(ctx, `INSERT INTO pause_activations(site_id, created_at) VALUES($1,$2)`, siteID, at); err != nil {
		return 0, err
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

func (d *DB) KillOfferSwitch(ctx context.Context, offerID, actor, userID, fromMode string, at time.Time) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals SET auto_paused=true
WHERE offer_id=$1 AND status='approved' AND approved_by_kind IN ('policy:auto','policy:yolo') AND auto_paused=false`, offerID)
	if err != nil {
		return 0, err
	}
	var version int
	err = tx.QueryRow(ctx, `INSERT INTO approval_policies(scope,offer_id,mode,kill_switch,mode_before_kill)
VALUES('offer',$1,'manual',true,NULLIF($2,''))
ON CONFLICT(offer_id) DO UPDATE SET
mode_before_kill=CASE WHEN approval_policies.kill_switch THEN approval_policies.mode_before_kill ELSE NULLIF($2,'') END,
kill_switch=true, mode='manual', version=approval_policies.version+1, updated_at=NOW()
RETURNING version`, offerID, fromMode).Scan(&version)
	if err != nil || version < 1 {
		if err == nil {
			err = errors.New("kill switch was not saved")
		}
		return 0, err
	}
	if fromMode != "manual" {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(offer_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),'manual')`, offerID, actor, userID, fromMode); err != nil {
			return 0, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO pause_activations(offer_id, created_at) VALUES($1,$2)`, offerID, at); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (d *DB) ReleaseOfferKillSwitch(ctx context.Context, offerID, actor, userID string) (string, int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE placement_proposals SET auto_paused=false
WHERE offer_id=$1 AND status='approved' AND approved_by_kind IN ('policy:auto','policy:yolo') AND auto_paused=true`, offerID)
	if err != nil {
		return "", 0, err
	}
	var mode, before string
	var on bool
	err = tx.QueryRow(ctx, `SELECT mode, kill_switch, COALESCE(mode_before_kill,'') FROM approval_policies WHERE offer_id=$1 FOR UPDATE`, offerID).Scan(&mode, &on, &before)
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
	if _, err = tx.Exec(ctx, `UPDATE approval_policies SET mode=$2, kill_switch=false, mode_before_kill=NULL, version=version+1, updated_at=NOW() WHERE offer_id=$1`, offerID, restored); err != nil {
		return "", 0, err
	}
	if restored != mode {
		if _, err = tx.Exec(ctx, `INSERT INTO mode_changes(offer_id,actor,user_id,from_mode,to_mode) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5)`, offerID, actor, userID, mode, restored); err != nil {
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

// OfferCopy is the stored product text used to score a placement.
type OfferCopy struct {
	Live        bool
	MakerID     string
	Category    string
	ProductName string
	Website     string
	Terms       string
	Commission  string
}

func (d *DB) OwnerSource(ctx context.Context, ownerID string) (string, error) {
	var source string
	err := d.Pool.QueryRow(ctx, `SELECT COALESCE(source,'') FROM owners WHERE id=$1`, ownerID).Scan(&source)
	if err != nil {
		return "", err
	}
	return NormalizeAttributionSource(source), nil
}

func (d *DB) OfferCopy(ctx context.Context, offerID string) (OfferCopy, error) {
	var row OfferCopy
	var status string
	err := d.Pool.QueryRow(ctx, `SELECT o.status, p.maker_id::text, COALESCE(p.primary_category,''), COALESCE(p.name,''), COALESCE(p.website_url,''), COALESCE(o.terms,''), COALESCE(o.commission_value::text,'')
FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(&status, &row.MakerID, &row.Category, &row.ProductName, &row.Website, &row.Terms, &row.Commission)
	row.Live = status == "listed"
	return row, err
}

func (d *DB) OfferFacts(ctx context.Context, offerID string) (live bool, makerID, category string, err error) {
	var status string
	err = d.Pool.QueryRow(ctx, `SELECT o.status, p.maker_id::text, COALESCE(p.primary_category,'')
FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(&status, &makerID, &category)
	return status == "listed", makerID, category, err
}

// normalizedSurfaceSQL folds host case, drops a query and a fragment, and
// drops trailing slashes so the same post is one box.
func normalizedSurfaceSQL(expr string) string {
	return `regexp_replace(lower(split_part(split_part(` + expr + `, '#', 1), '?', 1)), '/+$', '')`
}

// An approved placement that expired without being committed is not the box.
// A pending row and a committed row still are, including a committed row
// whose prepare window has passed.
var placementBoxCountSQL = `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND id<>$3::uuid AND status<>'revoked' AND removed_at IS NULL
AND NOT (status='pending' AND COALESCE(receipt->>'publication_status','')='refused')
AND NOT (status='approved' AND expires_at <= NOW())
AND (
  ($4::bigint > 0 AND wp_post_id = $4::bigint)
  OR (
    wp_post_id > 0 AND EXISTS (
      SELECT 1 FROM placement_proposals self
      WHERE self.id = $3::uuid AND self.wp_post_id > 0 AND self.wp_post_id = placement_proposals.wp_post_id
    )
  )
  OR ` + normalizedSurfaceSQL("surface_url") + ` = ` + normalizedSurfaceSQL("$2") + `
)`

type rowQuery interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func countPlacementBoxes(ctx context.Context, q rowQuery, siteID, surfaceURL, exceptID string, postID int64) (int, error) {
	var n int
	err := q.QueryRow(ctx, placementBoxCountSQL, siteID, surfaceURL, exceptID, postID).Scan(&n)
	return n, err
}

func (d *DB) CountPlacementBoxes(ctx context.Context, siteID, surfaceURL, exceptID string, postID int64) (int, error) {
	return countPlacementBoxes(ctx, d.Pool, siteID, surfaceURL, exceptID, postID)
}

// SiteHasApprovedPlacement is true after the first approved or committed box.
func (d *DB) SiteHasApprovedPlacement(ctx context.Context, siteID string) (bool, error) {
	if siteID == "" {
		return false, nil
	}
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND status IN ('approved','committed')`, siteID).Scan(&n)
	return n > 0, err
}

// OfferClaimText is the offer text a placement copy is checked against.
func (d *DB) OfferClaimText(ctx context.Context, offerID string) (string, error) {
	var product, category, website, terms, commission string
	err := d.Pool.QueryRow(ctx, `SELECT COALESCE(p.name,''), COALESCE(p.primary_category,''), COALESCE(p.website_url,''), COALESCE(o.terms,''), COALESCE(o.commission_value::text,'')
FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(&product, &category, &website, &terms, &commission)
	return strings.TrimSpace(strings.Join([]string{product, category, website, terms, commission}, "\n")), err
}

func (d *DB) CountPolicyPlacements(ctx context.Context, siteID string, since time.Time, includeID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND created_at>$2 AND approved_by_kind IN ('policy:auto','policy:yolo')
AND status<>'revoked' AND ($3 = '' OR id <> NULLIF($3,'')::uuid)`, siteID, since, includeID).Scan(&n)
	return n, err
}

// PauseBlocksRestore is true when a pause or kill switch for the site, the
// offer, or either account was turned on at or after the undo, or is still on.
func (d *DB) PauseBlocksRestore(ctx context.Context, siteID, offerID string, undoneAt time.Time) (bool, error) {
	var blocked bool
	err := d.Pool.QueryRow(ctx, `SELECT
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
OR EXISTS (SELECT 1 FROM approval_policies WHERE offer_id=$2::uuid AND kill_switch)`, siteID, offerID, undoneAt).Scan(&blocked)
	return blocked, err
}

func (d *DB) RecordOwnerPause(ctx context.Context, ownerID string, at time.Time) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO pause_activations(owner_id, created_at) VALUES($1,$2)`, ownerID, at)
	return err
}

func (d *DB) RecordPolicyApproval(ctx context.Context, placementID, siteID, ownerID, kind, rule string, version, relevance int, position string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE placement_proposals SET status='approved', approved_at=NOW(),
approved_by=NULLIF($3,'')::uuid, approved_by_kind=$4, policy_version=$5, matched_rule=$6, relevance=$7, position=$8
WHERE id=$1 AND site_id=$2 AND status='pending' AND expires_at>NOW() AND auto_paused=false`, placementID, siteID, ownerID, kind, version, rule, relevance, position)
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

// LatestListingCardJSON is the newest card stored for a product.
// A product with no card returns nil, nil.
func (d *DB) LatestListingCardJSON(ctx context.Context, makerID, productID string) ([]byte, error) {
	var raw []byte
	err := d.Pool.QueryRow(ctx, `SELECT card FROM listing_cards
WHERE maker_id=$1::uuid AND product_id=$2::uuid
ORDER BY created_at DESC, id DESC
LIMIT 1`, makerID, productID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}

// OfferClaimSource is the offer text the page checker compares.
type OfferClaimSource struct {
	MakerID         string
	ProductID       string
	LandingURL      string
	Terms           string
	CommissionType  string
	CommissionValue float64
}

func (d *DB) OfferClaimSource(ctx context.Context, offerID string) (OfferClaimSource, error) {
	var src OfferClaimSource
	err := d.Pool.QueryRow(ctx, `SELECT p.maker_id::text, p.id::text, COALESCE(o.landing_url,''), COALESCE(o.terms,''), COALESCE(o.commission_type,''), o.commission_value
FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(
		&src.MakerID, &src.ProductID, &src.LandingURL, &src.Terms, &src.CommissionType, &src.CommissionValue)
	return src, err
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

// PlacementCountsAsLiveSQL is the only placement live-count predicate.
// Live now, site totals, digest went-live, and account totals all read
// placement_counts_as_live. Listed offers are not placements.
const PlacementCountsAsLiveSQL = `EXISTS (SELECT 1 FROM placement_counts_as_live WHERE placement_counts_as_live.id = placement_proposals.id)`

func (d *DB) CountLivePlacements(ctx context.Context, siteID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals WHERE site_id=$1 AND `+PlacementCountsAsLiveSQL, siteID).Scan(&n)
	return n, err
}

// CountWentLiveSince counts placements on the site that count as live and
// whose live timestamp is after the previous digest open.
func (d *DB) CountWentLiveSince(ctx context.Context, siteID string, since time.Time) (int, error) {
	if siteID == "" {
		return 0, nil
	}
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals
WHERE site_id=$1 AND `+PlacementCountsAsLiveSQL+`
AND COALESCE(plugin_wait_at, approved_at, created_at) > $2`, siteID, since).Scan(&n)
	return n, err
}

// DigestPause reports whether the account, the site, or an offer is paused.
// Account is on only when an account pause was recorded and every site and
// offer on the account is still on the kill switch.
func (d *DB) DigestPause(ctx context.Context, ownerID, siteID, makerID string) (DigestPause, error) {
	var pause DigestPause
	err := d.Pool.QueryRow(ctx, `SELECT
EXISTS (SELECT 1 FROM pause_activations WHERE owner_id = NULLIF($1,'')::uuid)
AND ($2 = '' OR EXISTS (SELECT 1 FROM approval_policies WHERE site_id = NULLIF($2,'')::uuid AND kill_switch))
AND ($3 = '' OR NOT EXISTS (
    SELECT 1 FROM offers o
    JOIN products pr ON pr.id = o.product_id
    LEFT JOIN approval_policies ap ON ap.offer_id = o.id
    WHERE pr.maker_id = NULLIF($3,'')::uuid AND COALESCE(ap.kill_switch, false) = false
)),
$2 <> '' AND EXISTS (SELECT 1 FROM approval_policies WHERE site_id = NULLIF($2,'')::uuid AND kill_switch),
$3 <> '' AND EXISTS (
    SELECT 1 FROM approval_policies ap
    JOIN offers o ON o.id = ap.offer_id
    JOIN products pr ON pr.id = o.product_id
    WHERE pr.maker_id = NULLIF($3,'')::uuid AND ap.kill_switch
)`, ownerID, siteID, makerID).Scan(&pause.Account, &pause.Site, &pause.Offer)
	return pause, err
}

const digestPlacementWhere = `site_id=$1 AND (
    (status IN ('approved','committed') AND COALESCE(approved_at, created_at) > $2)
    OR plugin_wait <> ''
    OR plugin_failed_reason <> ''
)`

func (d *DB) CountDigestPlacements(ctx context.Context, siteID string, since time.Time) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM placement_proposals WHERE `+digestPlacementWhere, siteID, since).Scan(&n)
	return n, err
}

// digestPostTitleSQL is the WordPress article title. The plugin sends it on
// prepare or receipt. The box copy is not a title. A prepare that omits the
// title and matches no article returns an empty string. The UI uses the URL path.
const digestPostTitleSQL = `COALESCE(NULLIF(receipt->>'post_title', ''), NULLIF(post_title, ''), (
  SELECT a.title FROM articles a
  WHERE a.site_id = placement_proposals.site_id
    AND COALESCE(a.title, '') <> ''
    AND regexp_replace(lower(split_part(split_part(COALESCE(a.url, ''), '#', 1), '?', 1)), '/+$', '')
      = regexp_replace(lower(split_part(split_part(placement_proposals.surface_url, '#', 1), '?', 1)), '/+$', '')
  ORDER BY a.updated_at DESC
  LIMIT 1
), '')`

// digestPositionSQL uses the stored position, then the publisher's saved box
// position, then end, so a digest item never has an empty position.
const digestPositionSQL = `COALESCE(NULLIF(position, ''), (
  SELECT os.box_position FROM owner_setup os
  JOIN owners o ON o.id = os.owner_id
  WHERE o.site_id = placement_proposals.site_id
    AND os.role = 'publisher'
    AND COALESCE(os.box_position, '') <> ''
  LIMIT 1
), 'end')`

// digestSurfaceSQL prefers the plugin receipt, then the stored placement URL.
const digestSurfaceSQL = `COALESCE(NULLIF(receipt->>'surface_url', ''), surface_url)`

func (d *DB) DigestPlacements(ctx context.Context, siteID string, since time.Time) ([]DigestItem, error) {
	limit := DigestPageSize
	if limit < 1 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `SELECT id::text, copy, COALESCE(approved_at, created_at),
plugin_wait, plugin_wait_at, plugin_contact_at, plugin_failed_reason, failed_action, `+PlacementCountsAsLiveSQL+`, COALESCE(matched_rule,''),
`+digestPostTitleSQL+`, `+digestSurfaceSQL+`, `+digestPositionSQL+`, COALESCE(NULLIF(link_text,''), copy), COALESCE(NULLIF(label,''), reason),
(SELECT COUNT(*) FROM clicks WHERE clicks.placement_id = placement_proposals.id),
COALESCE(approved_by_kind,''), COALESCE(policy_version,0), COALESCE(undone_by_user::text,'')
FROM placement_proposals
WHERE `+digestPlacementWhere+`
ORDER BY COALESCE(approved_at, created_at) DESC LIMIT $3`, siteID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DigestItem{}
	for rows.Next() {
		var item DigestItem
		item.Kind = "placement"
		if err = rows.Scan(&item.ID, &item.Summary, &item.WentLiveAt, &item.PluginWait, &item.PluginWaitAt, &item.PluginContactAt, &item.PluginFailedReason, &item.FailedAction, &item.CountsAsLive, &item.MatchedRule, &item.PostTitle, &item.SurfaceURL, &item.Position, &item.LinkText, &item.Label, &item.Clicks, &item.ApprovedBy, &item.PolicyVersion, &item.UndoneBy); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// digestOfferColumns is the shared offer row for both digest endpoints.
// Title and URL come from the offer's placement receipt, then that placement.
// A placement prepared without a title returns an empty string so the UI can
// use the URL path. A held offer with no placement uses the product name.
// The placement copy is not the title.
const digestOfferColumns = `o.id::text, p.name,
(o.updated_at AT TIME ZONE current_setting('TimeZone')),
(SELECT COUNT(*) FROM clicks c JOIN campaigns cam ON cam.id=c.campaign_id WHERE cam.offer_id=o.id),
o.status,
CASE WHEN place.id IS NOT NULL
  THEN COALESCE(NULLIF(place.receipt->>'post_title', ''), NULLIF(place.post_title, ''), '')
  ELSE COALESCE(NULLIF(p.name, ''), '')
END,
COALESCE(NULLIF(place.receipt->>'surface_url', ''), NULLIF(place.surface_url, ''), NULLIF(o.landing_url, ''), NULLIF(p.website_url, '')),
COALESCE(NULLIF(place.position, ''), 'end')`

const digestOfferFrom = `FROM offers o JOIN products p ON p.id=o.product_id
LEFT JOIN LATERAL (
  SELECT id, receipt, post_title, surface_url, position
  FROM placement_proposals
  WHERE offer_id = o.id
  ORDER BY COALESCE(approved_at, created_at) DESC
  LIMIT 1
) place ON true`

func (d *DB) DigestOffers(ctx context.Context, makerID string, since time.Time) ([]DigestItem, error) {
	limit := DigestPageSize
	if limit < 1 {
		limit = 50
	}
	// offers.updated_at is timestamp without time zone: the session wall clock
	// at write. Bind the floor as timestamptz, then convert that instant into
	// the same wall clock, so a process location such as Europe/Rome does not
	// move the cutoff. Return the column as that same instant.
	// Submitted offers stay in the digest until they leave that state. Listed
	// offers stay inside the window. The floor is still an absolute instant.
	rows, err := d.Pool.Query(ctx, `SELECT `+digestOfferColumns+`
`+digestOfferFrom+`
WHERE p.maker_id=$1 AND o.status IN ('listed','submitted')
AND (
  o.status = 'submitted'
  OR o.updated_at > ($2::timestamptz AT TIME ZONE current_setting('TimeZone'))
)
ORDER BY o.updated_at DESC LIMIT $3`, makerID, since.UTC(), limit)
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
		if err := rows.Scan(&item.ID, &item.Summary, &item.WentLiveAt, &item.Clicks, &item.Status, &item.PostTitle, &item.SurfaceURL, &item.Position); err != nil {
			return nil, err
		}
		item.Label = item.Summary
		item.CountsAsLive = item.Status == "listed"
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) PeekUndo(ctx context.Context, hash, ownerID string, now time.Time) (kind, placementID, offerID string, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT kind, COALESCE(placement_id::text,''), COALESCE(offer_id::text,'')
FROM undo_tokens WHERE token_hash=$1 AND owner_id=$2 AND used_at IS NULL AND expires_at>$3`, hash, ownerID, now).Scan(&kind, &placementID, &offerID)
	return kind, placementID, offerID, err
}

// MakerDigestOffers lists this maker's held, in-review, and live offers.
// Submitted stays until it leaves that state. Listed stays with no digest window.
// offerID limits the read to one offer when the key is scoped to it.
func (d *DB) MakerDigestOffers(ctx context.Context, makerID, offerID string) ([]DigestItem, error) {
	limit := DigestPageSize
	if limit < 1 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `SELECT `+digestOfferColumns+`
`+digestOfferFrom+`
WHERE p.maker_id=$1 AND o.status IN ('listed','submitted')
AND ($2 = '' OR o.id::text = $2)
ORDER BY o.updated_at DESC LIMIT $3`, makerID, offerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigest(rows, "offer")
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
		if _, err = tx.Exec(ctx, `UPDATE placement_proposals SET
status_before_undo=status,
undone_at=$2,
undone_by_user=$3::uuid,
undone_by_key_hash='',
plugin_wait_before=plugin_wait,
plugin_wait='remove',
plugin_wait_at=$2,
plugin_failed_reason='',
failed_action='',
restore_unconfirmed=false,
status='revoked',
revoked_by_side=CASE
  WHEN EXISTS (SELECT 1 FROM owners o WHERE o.id=$3::uuid AND o.site_id = placement_proposals.site_id) THEN 'publisher'
  ELSE 'maker'
END
WHERE id=$1 AND status<>'revoked'`, placementID, now, ownerID); err != nil {
			return "", "", "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,placement_id,created_at,actor) VALUES('placement_undo',$1,$2,$3,$4)`, ownerID, placementID, now, ownerID)
	case "offer":
		if _, err = tx.Exec(ctx, `UPDATE offers SET status='paused', updated_at=$2 WHERE id=$1 AND status='listed'`, offerID, now); err != nil {
			return "", "", "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO owner_events(kind,user_id,offer_id,created_at,actor) VALUES('offer_undo',$1,$2,$3,$4)`, ownerID, offerID, now, ownerID)
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
