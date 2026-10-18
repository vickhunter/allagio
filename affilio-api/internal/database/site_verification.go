package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SiteVerification is the domain-ownership record for one site.
type SiteVerification struct {
	ID         string
	Domain     string
	Token      string
	VerifiedAt *time.Time
}

func newVerificationToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (d *DB) GetSiteVerification(ctx context.Context, siteID string) (SiteVerification, error) {
	var v SiteVerification
	err := d.Pool.QueryRow(ctx, `SELECT id::text, domain, COALESCE(verification_token,''), verified_at FROM sites WHERE id=$1`, siteID).Scan(&v.ID, &v.Domain, &v.Token, &v.VerifiedAt)
	return v, err
}

// EnsureSiteVerificationToken returns the site's token, issuing one when the
// column is still empty. A token that is already stored is left in place.
func (d *DB) EnsureSiteVerificationToken(ctx context.Context, siteID string) (string, error) {
	token, err := newVerificationToken()
	if err != nil {
		return "", err
	}
	var stored string
	err = d.Pool.QueryRow(ctx, `UPDATE sites SET verification_token=$2 WHERE id=$1 AND verification_token='' RETURNING verification_token`, siteID, token).Scan(&stored)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = d.Pool.QueryRow(ctx, `SELECT verification_token FROM sites WHERE id=$1`, siteID).Scan(&stored)
	return stored, err
}

func (d *DB) DomainVerified(ctx context.Context, domain string) (bool, error) {
	var verified bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM sites WHERE lower(domain)=lower($1) AND verified_at IS NOT NULL
	)`, domain).Scan(&verified)
	return verified, err
}

// MarkSiteVerified sets verified_at when this site is still unverified and no
// other site on the domain is verified. A miss is pgx.ErrNoRows.
func (d *DB) MarkSiteVerified(ctx context.Context, siteID string) (time.Time, error) {
	var at time.Time
	err := d.Pool.QueryRow(ctx, `
UPDATE sites AS s
SET verified_at = NOW()
WHERE s.id = $1
  AND s.verified_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sites other
    WHERE lower(other.domain) = lower(s.domain)
      AND other.verified_at IS NOT NULL
  )
RETURNING s.verified_at`, siteID).Scan(&at)
	return at, err
}
