package database

import (
	"context"
	"sort"
	"time"

	"affilio-api/internal/entry"

	"github.com/jackc/pgx/v5"
)

// AccountKey is an active account-wide credential. SiteID and MakerID come
// from the owner and may be empty when that side does not exist yet.
type AccountKey struct {
	OwnerID string
	SiteID  string
	MakerID string
}

type AccountKeyRow struct {
	ID        string
	Prefix    string
	CreatedAt time.Time
	RevokedAt *time.Time
}

type MoneyAmount struct {
	Currency string
	Amount   float64
}

type SourceCount struct {
	Source string
	Clicks int
}

type AccountProject struct {
	Kind           string
	ID             string
	Name           string
	Mode           string
	WentLive       int
	Clicks         int
	ClicksBySource []SourceCount
	Earnings       []MoneyAmount
	Items          []DigestItem
}

type AccountDigest struct {
	WentLive          int
	Clicks            int
	ClicksBySource    []SourceCount
	PublisherEarnings []MoneyAmount
	MakerEarnings     []MoneyAmount
	Projects          []AccountProject
}

func (d *DB) ActiveAccountKey(ctx context.Context, keyHash string) (AccountKey, error) {
	var key AccountKey
	err := d.Pool.QueryRow(ctx, `SELECT o.id::text, COALESCE(o.site_id::text,''), COALESCE(o.maker_id::text,'')
FROM account_keys k JOIN owners o ON o.id=k.owner_id
WHERE k.key_hash=$1 AND k.revoked_at IS NULL`, keyHash).Scan(&key.OwnerID, &key.SiteID, &key.MakerID)
	return key, err
}

func (d *DB) InsertAccountKey(ctx context.Context, ownerID, keyHash, keyPrefix string, at time.Time) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO account_keys(key_hash,key_prefix,owner_id,created_at)
VALUES($1,$2,$3,$4) RETURNING id::text`, keyHash, keyPrefix, ownerID, at).Scan(&id)
	return id, err
}

func (d *DB) ListAccountKeys(ctx context.Context, ownerID string) ([]AccountKeyRow, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, key_prefix, created_at, revoked_at
FROM account_keys WHERE owner_id=$1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountKeyRow{}
	for rows.Next() {
		var row AccountKeyRow
		if err = rows.Scan(&row.ID, &row.Prefix, &row.CreatedAt, &row.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (d *DB) RevokeAccountKey(ctx context.Context, ownerID, id string, at time.Time) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE account_keys SET revoked_at=$3
WHERE id=$1 AND owner_id=$2 AND revoked_at IS NULL`, id, ownerID, at)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var exists bool
	err = d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_keys WHERE id=$1 AND owner_id=$2)`, id, ownerID).Scan(&exists)
	return exists, err
}

func (d *DB) LoadOwnerScope(ctx context.Context, ownerID string) (siteID, makerID string, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT COALESCE(site_id::text,''), COALESCE(maker_id::text,'') FROM owners WHERE id=$1`, ownerID).Scan(&siteID, &makerID)
	return siteID, makerID, err
}

func (d *DB) AccountProjects(ctx context.Context, ownerID string) ([]AccountProject, error) {
	rows, err := d.Pool.Query(ctx, `SELECT 'site', s.id::text, COALESCE(s.name, s.domain)
FROM owners o JOIN sites s ON s.id=o.site_id WHERE o.id=$1
UNION ALL
SELECT 'offer', ofr.id::text, p.name
FROM owners o
JOIN products p ON p.maker_id=o.maker_id
JOIN offers ofr ON ofr.product_id=p.id
WHERE o.id=$1
ORDER BY 1 DESC, 3`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountProject{}
	for rows.Next() {
		var project AccountProject
		if err = rows.Scan(&project.Kind, &project.ID, &project.Name); err != nil {
			return nil, err
		}
		out = append(out, project)
	}
	return out, rows.Err()
}

func (d *DB) AccountDigest(ctx context.Context, ownerID string) (AccountDigest, error) {
	projects, err := d.AccountProjects(ctx, ownerID)
	if err != nil {
		return AccountDigest{}, err
	}
	var digest AccountDigest
	seenClicks := map[string]string{}
	var siteIDs, offerIDs []string
	for i := range projects {
		project := &projects[i]
		switch project.Kind {
		case "site":
			siteIDs = append(siteIDs, project.ID)
			project.Items, err = d.DigestPlacements(ctx, project.ID, time.Unix(0, 0).UTC())
			if err != nil {
				return AccountDigest{}, err
			}
			project.WentLive, err = d.CountLivePlacements(ctx, project.ID)
			if err != nil {
				return AccountDigest{}, err
			}
			project.ClicksBySource, err = d.clickSources(ctx, `SELECT c.id::text, COALESCE(c.referrer,'') FROM clicks c WHERE c.site_id=$1`, project.ID)
			if err != nil {
				return AccountDigest{}, err
			}
			project.Earnings, err = d.testPublisherEarnings(ctx, project.ID)
		case "offer":
			offerIDs = append(offerIDs, project.ID)
			project.Items, err = d.digestOffer(ctx, project.ID)
			if err != nil {
				return AccountDigest{}, err
			}
			project.ClicksBySource, err = d.clickSources(ctx, `SELECT c.id::text, COALESCE(c.referrer,'')
FROM clicks c JOIN campaigns cam ON cam.id=c.campaign_id WHERE cam.offer_id=$1`, project.ID)
			if err != nil {
				return AccountDigest{}, err
			}
			project.Earnings, err = d.testMakerEarnings(ctx, project.ID)
		}
		if err != nil {
			return AccountDigest{}, err
		}
		project.Clicks = sumSources(project.ClicksBySource)
		if project.Kind == "offer" {
			for _, item := range project.Items {
				if item.CountsAsLive {
					project.WentLive++
				}
			}
		}
		digest.WentLive += project.WentLive
		mode, modeErr := d.projectMode(ctx, project.Kind, project.ID)
		if modeErr != nil {
			return AccountDigest{}, modeErr
		}
		project.Mode = mode
	}
	if len(siteIDs) > 0 || len(offerIDs) > 0 {
		clickRows, err := d.Pool.Query(ctx, `SELECT c.id::text, COALESCE(c.referrer,'')
FROM clicks c
WHERE c.site_id = ANY($1::uuid[])
   OR EXISTS (SELECT 1 FROM campaigns cam WHERE cam.id=c.campaign_id AND cam.offer_id = ANY($2::uuid[]))`, uuidArray(siteIDs), uuidArray(offerIDs))
		if err != nil {
			return AccountDigest{}, err
		}
		defer clickRows.Close()
		for clickRows.Next() {
			var id, referrer string
			if err = clickRows.Scan(&id, &referrer); err != nil {
				return AccountDigest{}, err
			}
			seenClicks[id] = referrer
		}
		if err = clickRows.Err(); err != nil {
			return AccountDigest{}, err
		}
	}
	digest.ClicksBySource = sourcesFromMap(seenClicks)
	digest.Clicks = sumSources(digest.ClicksBySource)
	digest.PublisherEarnings, err = d.testAccountPublisherEarnings(ctx, ownerID)
	if err != nil {
		return AccountDigest{}, err
	}
	digest.MakerEarnings, err = d.testAccountMakerEarnings(ctx, ownerID)
	if err != nil {
		return AccountDigest{}, err
	}
	digest.Projects = projects
	return digest, nil
}

func (d *DB) digestOffer(ctx context.Context, offerID string) ([]DigestItem, error) {
	rows, err := d.Pool.Query(ctx, `SELECT o.id::text, p.name, o.updated_at,
(SELECT COUNT(*) FROM clicks c JOIN campaigns cam ON cam.id=c.campaign_id WHERE cam.offer_id=o.id)
FROM offers o JOIN products p ON p.id=o.product_id
WHERE o.id=$1 AND o.status='listed'`, offerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigest(rows, "offer")
}

func (d *DB) projectMode(ctx context.Context, kind, id string) (string, error) {
	var mode string
	var err error
	if kind == "site" {
		err = d.Pool.QueryRow(ctx, `SELECT mode FROM approval_policies WHERE site_id=$1`, id).Scan(&mode)
	} else {
		err = d.Pool.QueryRow(ctx, `SELECT mode FROM approval_policies WHERE offer_id=$1`, id).Scan(&mode)
	}
	if err == pgx.ErrNoRows {
		return entry.ModeManual, nil
	}
	return mode, err
}

func (d *DB) clickSources(ctx context.Context, query, id string) ([]SourceCount, error) {
	rows, err := d.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]string{}
	for rows.Next() {
		var clickID, referrer string
		if err = rows.Scan(&clickID, &referrer); err != nil {
			return nil, err
		}
		counts[clickID] = referrer
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return sourcesFromMap(counts), nil
}

// unconfirmedRestoreEarnings drops test commission whose click belongs to a
// placement whose restore failed and has not been confirmed placed again.
const unconfirmedRestoreEarnings = ` AND NOT EXISTS (
  SELECT 1 FROM clicks held_click
  JOIN placement_proposals held_placement ON held_placement.id = held_click.placement_id
  WHERE held_click.id = cv.click_id AND held_placement.restore_unconfirmed)`

func (d *DB) testPublisherEarnings(ctx context.Context, siteID string) ([]MoneyAmount, error) {
	return d.moneyQuery(ctx, `SELECT le.currency, COALESCE(SUM(le.amount),0)
FROM ledger_entries le JOIN conversions cv ON cv.id=le.conversion_id
WHERE le.site_id=$1 AND cv.payment_mode='test'`+unconfirmedRestoreEarnings+`
GROUP BY le.currency ORDER BY le.currency`, siteID)
}

func (d *DB) testMakerEarnings(ctx context.Context, offerID string) ([]MoneyAmount, error) {
	return d.moneyQuery(ctx, `SELECT cv.currency, COALESCE(SUM(cv.commission_amount),0)
FROM conversions cv
WHERE cv.offer_id=$1 AND cv.payment_mode='test' AND cv.commission_amount IS NOT NULL`+unconfirmedRestoreEarnings+`
GROUP BY cv.currency ORDER BY cv.currency`, offerID)
}

func (d *DB) testAccountPublisherEarnings(ctx context.Context, ownerID string) ([]MoneyAmount, error) {
	return d.moneyQuery(ctx, `SELECT le.currency, COALESCE(SUM(le.amount),0)
FROM ledger_entries le
JOIN conversions cv ON cv.id=le.conversion_id
JOIN owners o ON o.site_id=le.site_id
WHERE o.id=$1 AND cv.payment_mode='test'`+unconfirmedRestoreEarnings+`
GROUP BY le.currency ORDER BY le.currency`, ownerID)
}

func (d *DB) testAccountMakerEarnings(ctx context.Context, ownerID string) ([]MoneyAmount, error) {
	return d.moneyQuery(ctx, `SELECT cv.currency, COALESCE(SUM(cv.commission_amount),0)
FROM conversions cv
JOIN offers ofr ON ofr.id=cv.offer_id
JOIN products p ON p.id=ofr.product_id
JOIN owners o ON o.maker_id=p.maker_id
WHERE o.id=$1 AND cv.payment_mode='test' AND cv.commission_amount IS NOT NULL`+unconfirmedRestoreEarnings+`
GROUP BY cv.currency ORDER BY cv.currency`, ownerID)
}

func (d *DB) moneyQuery(ctx context.Context, query, id string) ([]MoneyAmount, error) {
	rows, err := d.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MoneyAmount{}
	for rows.Next() {
		var amount MoneyAmount
		if err = rows.Scan(&amount.Currency, &amount.Amount); err != nil {
			return nil, err
		}
		out = append(out, amount)
	}
	return out, rows.Err()
}

func sourcesFromMap(clicks map[string]string) []SourceCount {
	counts := map[string]int{}
	for _, referrer := range clicks {
		counts[entry.ClickSource(referrer)]++
	}
	out := make([]SourceCount, 0, len(counts))
	for source, n := range counts {
		out = append(out, SourceCount{Source: source, Clicks: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

func sumSources(items []SourceCount) int {
	total := 0
	for _, item := range items {
		total += item.Clicks
	}
	return total
}

func uuidArray(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
