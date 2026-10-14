package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PluginKeyRecord is a plugin key without its secret. The raw key is never stored.
type PluginKeyRecord struct {
	ID        string
	Prefix    string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// IssuePluginKeyIfAbsent stores a hash and prefix when the site has no active
// plugin key. issued is false when one is already active. The caller shows
// the raw key only when issued is true.
func (d *DB) IssuePluginKeyIfAbsent(ctx context.Context, siteID, keyHash, keyPrefix string) (issued bool, err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM sites WHERE id=$1 FOR UPDATE`, siteID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err != nil {
		return false, err
	}
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_keys WHERE site_id=$1 AND revoked_at IS NULL)`, siteID).Scan(&active); err != nil {
		return false, err
	}
	if active {
		return false, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO plugin_keys(key_hash, key_prefix, site_id) VALUES($1,$2,$3)`, keyHash, keyPrefix, siteID); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// RotatePluginKey revokes every active plugin key for the site and stores the
// new hash. The raw key is returned by the handler, once.
func (d *DB) RotatePluginKey(ctx context.Context, siteID, keyHash, keyPrefix string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM sites WHERE id=$1 FOR UPDATE`, siteID).Scan(&id)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE plugin_keys SET revoked_at=NOW() WHERE site_id=$1 AND revoked_at IS NULL`, siteID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO plugin_keys(key_hash, key_prefix, site_id) VALUES($1,$2,$3)`, keyHash, keyPrefix, siteID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevokeActivePluginKeys marks every active plugin key for the site revoked.
func (d *DB) RevokeActivePluginKeys(ctx context.Context, siteID string) (int64, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE plugin_keys SET revoked_at=NOW() WHERE site_id=$1 AND revoked_at IS NULL`, siteID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ListPluginKeys returns prefixes only. key_hash stays in the table.
func (d *DB) ListPluginKeys(ctx context.Context, siteID string) ([]PluginKeyRecord, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, key_prefix, created_at, revoked_at FROM plugin_keys WHERE site_id=$1 ORDER BY created_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PluginKeyRecord
	for rows.Next() {
		var row PluginKeyRecord
		if err = rows.Scan(&row.ID, &row.Prefix, &row.CreatedAt, &row.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
