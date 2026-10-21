package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// AgentJoinTTL is how long a self-serve join token stays valid.
	AgentJoinTTL = 24 * time.Hour
	// AgentJoinDailyCap is the number of join tokens one address hash can
	// create in 24 hours. Revoked tokens still count.
	AgentJoinDailyCap = 30
	agentJoinScope    = "agent"
)

// ErrJoinLimited is the daily cap for POST /v1/agent-tokens.
var ErrJoinLimited = errors.New("join limited")

// AgentJoinIssued is the row created with a new join token. The plaintext
// token is not stored.
type AgentJoinIssued struct {
	OwnerID   string
	TokenID   string
	Scope     string
	ExpiresAt time.Time
}

// AgentJoinTokenRow is a stored token without its hash.
type AgentJoinTokenRow struct {
	ID        string
	Prefix    string
	Scope     string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

// CreateAgentJoinToken inserts an agent-only owner with no site and no maker,
// and one hashed bearer token. The caller supplies the token hash.
func (d *DB) CreateAgentJoinToken(ctx context.Context, tokenHash, prefix, ipHash, resource, name, subject, email string) (AgentJoinIssued, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return AgentJoinIssued{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "join:"+ipHash); err != nil {
		return AgentJoinIssued{}, err
	}
	var n int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM agent_join_tokens WHERE creator_ip_hash=$1 AND created_at > NOW() - INTERVAL '24 hours'`, ipHash).Scan(&n); err != nil {
		return AgentJoinIssued{}, err
	}
	if n >= AgentJoinDailyCap {
		return AgentJoinIssued{}, ErrJoinLimited
	}
	var out AgentJoinIssued
	err = tx.QueryRow(ctx, `INSERT INTO owners(issuer, subject, email, name, source, agent_only, human_login)
VALUES('https://offer-rails.agent', $1, $2, $3, 'mcp', true, false) RETURNING id::text`, subject, email, name).Scan(&out.OwnerID)
	if err != nil {
		return AgentJoinIssued{}, err
	}
	secs := int(AgentJoinTTL.Seconds())
	err = tx.QueryRow(ctx, `INSERT INTO agent_join_tokens(token_hash, token_prefix, owner_id, scope, resource, expires_at, creator_ip_hash)
VALUES($1, $2, $3, $4, $5, NOW() + ($6 * INTERVAL '1 second'), $7)
RETURNING id::text, scope, expires_at`, tokenHash, prefix, out.OwnerID, agentJoinScope, resource, secs, ipHash).Scan(&out.TokenID, &out.Scope, &out.ExpiresAt)
	if err != nil {
		return AgentJoinIssued{}, err
	}
	return out, tx.Commit(ctx)
}

// AgentJoinToken loads a live join grant. The resource must match the server
// that minted the token. A revoked, expired, or unknown hash is not a grant.
func (d *DB) AgentJoinToken(ctx context.Context, hash, resource string) (*AgentGrant, error) {
	var g AgentGrant
	err := d.Pool.QueryRow(ctx, `SELECT t.id::text, 'agent-join', 'agent join', t.resource, t.scope,
COALESCE(o.site_id::text, ''), COALESCE(o.maker_id::text, ''), t.expires_at, o.id::text
FROM agent_join_tokens t
JOIN owners o ON o.id = t.owner_id
WHERE t.token_hash=$1 AND t.resource=$2 AND t.expires_at > NOW() AND t.revoked_at IS NULL
AND o.agent_only
AND (o.maker_id IS NULL OR EXISTS(SELECT 1 FROM makers m WHERE m.id = o.maker_id AND m.status = 'active'))`, hash, resource).Scan(
		&g.ID, &g.ClientID, &g.ClientName, &g.Resource, &g.Scope, &g.SiteID, &g.MakerID, &g.ExpiresAt, &g.OwnerID)
	if err != nil {
		return nil, err
	}
	g.Join = true
	return &g, nil
}

// ResolveBearer checks an OAuth access token, then a join token, then an
// agent-account key. The first match wins.
func (d *DB) ResolveBearer(ctx context.Context, hash, raw, resource string) (*AgentGrant, error) {
	g, err := d.AgentToken(ctx, hash, resource)
	if err == nil {
		return g, nil
	}
	g, err = d.AgentJoinToken(ctx, hash, resource)
	if err == nil {
		return g, nil
	}
	return d.AgentAccountBearer(ctx, hash, raw, resource)
}

// RevokeAgentJoinToken revokes the presented token. A second call with the
// same hash still reports the id. An unknown hash is ErrNoRows.
func (d *DB) RevokeAgentJoinToken(ctx context.Context, hash string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `UPDATE agent_join_tokens SET revoked_at=NOW() WHERE token_hash=$1 AND revoked_at IS NULL RETURNING id::text`, hash).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = d.Pool.QueryRow(ctx, `SELECT id::text FROM agent_join_tokens WHERE token_hash=$1`, hash).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// RevokeAgentJoinTokenID revokes one token that belongs to ownerID.
// A token owned by someone else is ErrNoRows.
func (d *DB) RevokeAgentJoinTokenID(ctx context.Context, ownerID, tokenID string) (bool, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT id::text FROM agent_join_tokens WHERE id=$1 AND owner_id=$2`, tokenID, ownerID).Scan(&id)
	if err != nil {
		return false, err
	}
	_, err = d.Pool.Exec(ctx, `UPDATE agent_join_tokens SET revoked_at=COALESCE(revoked_at, NOW()) WHERE id=$1 AND owner_id=$2`, tokenID, ownerID)
	return err == nil, err
}

// ListAgentJoinTokens returns this owner's tokens, newest first. Hashes stay
// in the database.
func (d *DB) ListAgentJoinTokens(ctx context.Context, ownerID string) ([]AgentJoinTokenRow, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, token_prefix, scope, expires_at, revoked_at IS NOT NULL, created_at
FROM agent_join_tokens WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 50`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentJoinTokenRow{}
	for rows.Next() {
		var row AgentJoinTokenRow
		if err = rows.Scan(&row.ID, &row.Prefix, &row.Scope, &row.ExpiresAt, &row.Revoked, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// EnsureJoinPublisher attaches a publisher site to an agent-only owner that
// does not have one yet. The join token scope gains publisher.
func (d *DB) EnsureJoinPublisher(ctx context.Context, ownerID string) (string, string, error) {
	return d.ensureJoinSide(ctx, ownerID, "publisher")
}

// EnsureJoinMaker attaches a maker to an agent-only owner that does not have
// one yet. The join token scope gains maker.
func (d *DB) EnsureJoinMaker(ctx context.Context, ownerID string) (string, string, error) {
	return d.ensureJoinSide(ctx, ownerID, "maker")
}

func (d *DB) ensureJoinSide(ctx context.Context, ownerID, side string) (string, string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	var name, siteID, makerID string
	var agentOnly bool
	err = tx.QueryRow(ctx, `SELECT name, COALESCE(site_id::text, ''), COALESCE(maker_id::text, ''), agent_only
FROM owners WHERE id=$1 FOR UPDATE`, ownerID).Scan(&name, &siteID, &makerID, &agentOnly)
	if err != nil {
		return "", "", err
	}
	if !agentOnly {
		return "", "", errors.New("not an agent join account")
	}
	switch side {
	case "publisher":
		if siteID == "" {
			siteID, err = insertJoinSite(ctx, tx, ownerID, name)
			if err != nil {
				return "", "", err
			}
			if _, err = tx.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1 AND site_id IS NULL`, ownerID, siteID); err != nil {
				return "", "", err
			}
			if err = seedNewSitePublishAsk(ctx, tx, ownerID, siteID); err != nil {
				return "", "", err
			}
		}
	case "maker":
		if makerID == "" {
			makerID, err = insertJoinMaker(ctx, tx, ownerID, name)
			if err != nil {
				return "", "", err
			}
			if _, err = tx.Exec(ctx, `UPDATE owners SET maker_id=$2 WHERE id=$1 AND maker_id IS NULL`, ownerID, makerID); err != nil {
				return "", "", err
			}
		}
	default:
		return "", "", errors.New("unknown join side")
	}
	scope, err := appendJoinScope(ctx, tx, ownerID, side)
	if err != nil {
		return "", "", err
	}
	id := siteID
	if side == "maker" {
		id = makerID
	}
	return id, scope, tx.Commit(ctx)
}

func insertJoinSite(ctx context.Context, tx pgx.Tx, ownerID, name string) (string, error) {
	compact := strings.ReplaceAll(ownerID, "-", "")
	if len(compact) < 12 {
		return "", errors.New("owner id is not a uuid")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf)
	hash := hex.EncodeToString(sum[:])
	token, err := newVerificationToken()
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO sites(api_key_hash, api_key_prefix, domain, name, verification_token)
VALUES($1, $2, $3, $4, $5) RETURNING id::text`, hash, hash[:8], "j"+compact[:12]+".invalid", name, token).Scan(&id)
	return id, err
}

func insertJoinMaker(ctx context.Context, tx pgx.Tx, ownerID, name string) (string, error) {
	compact := strings.ReplaceAll(ownerID, "-", "")
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO makers(email, name) VALUES($1, $2) RETURNING id::text`, "join-"+compact+"@agents.invalid", name).Scan(&id)
	return id, err
}

func appendJoinScope(ctx context.Context, tx pgx.Tx, ownerID, word string) (string, error) {
	var scope string
	err := tx.QueryRow(ctx, `UPDATE agent_join_tokens
SET scope = CASE
  WHEN position($2 in ' ' || scope || ' ') > 0 THEN scope
  ELSE trim(scope || ' ' || $3)
END
WHERE owner_id=$1
RETURNING scope`, ownerID, " "+word+" ", word).Scan(&scope)
	return scope, err
}
