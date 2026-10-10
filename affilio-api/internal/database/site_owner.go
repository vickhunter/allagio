package database

import (
	"context"
	"errors"
)

func (d *DB) StartSiteOwnerConnection(ctx context.Context, siteID, hash string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM site_owner_connections WHERE expires_at<NOW() OR site_id=$1`, siteID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO site_owner_connections(token_hash,site_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '10 minutes')`, hash, siteID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DB) SiteOwnerConnection(ctx context.Context, hash string) (string, string, error) {
	var id, domain string
	err := d.Pool.QueryRow(ctx, `SELECT s.id,s.domain FROM site_owner_connections c JOIN sites s ON s.id=c.site_id WHERE c.token_hash=$1 AND c.expires_at>NOW()`, hash).Scan(&id, &domain)
	return id, domain, err
}

func (d *DB) SiteHasOwner(ctx context.Context, siteID string) (bool, error) {
	var linked bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM owners WHERE site_id=$1)`, siteID).Scan(&linked)
	return linked, err
}

func (d *DB) CompleteSiteOwnerConnection(ctx context.Context, ownerID, hash, source string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var siteID string
	if err = tx.QueryRow(ctx, `DELETE FROM site_owner_connections WHERE token_hash=$1 AND expires_at>NOW() RETURNING site_id`, hash).Scan(&siteID); err != nil {
		return err
	}
	var current *string
	if err = tx.QueryRow(ctx, `SELECT site_id::text FROM owners WHERE id=$1 FOR UPDATE`, ownerID).Scan(&current); err != nil {
		return err
	}
	if current != nil && *current != siteID {
		return errors.New("publisher already connected to another site")
	}
	if source == "" {
		_, err = tx.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1`, ownerID, siteID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE owners SET site_id=$2, source=$3 WHERE id=$1`, ownerID, siteID, NormalizeAttributionSource(source))
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
