package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Owner struct {
	ID      string
	Issuer  string
	Subject string
	Email   string
	Name    string
	SiteID  string
	MakerID string
	CSRF    string
	Source  string
}

type LoginAttempt struct{ Nonce, Verifier, ReturnTo string }
type OAuthClient struct {
	ID, Name     string
	RedirectURIs []string
}
type OAuthRequest struct{ ID, OwnerID, ClientID, RedirectURI, Resource, Scope, Challenge, State string }
type AgentGrant struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	Resource   string    `json:"resource"`
	Scope      string    `json:"scope"`
	SiteID     string    `json:"site_id,omitempty"`
	MakerID    string    `json:"maker_id,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
	// OwnerID is set for OAuth grants and for self-serve join tokens.
	// Join is true only for a token from POST /v1/agent-tokens.
	OwnerID string `json:"-"`
	Join    bool   `json:"-"`
}

func (d *DB) SaveLoginAttempt(ctx context.Context, hash string, a LoginAttempt) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO owner_login_attempts VALUES($1,$2,$3,$4,NOW()+INTERVAL '10 minutes')`, hash, a.Nonce, a.Verifier, a.ReturnTo)
	return err
}

func (d *DB) ConsumeLoginAttempt(ctx context.Context, hash string) (LoginAttempt, error) {
	var a LoginAttempt
	err := d.Pool.QueryRow(ctx, `DELETE FROM owner_login_attempts WHERE state_hash=$1 AND expires_at>NOW() RETURNING nonce,verifier,return_to`, hash).Scan(&a.Nonce, &a.Verifier, &a.ReturnTo)
	return a, err
}

func (d *DB) StartOwnerSession(ctx context.Context, o Owner, tokenHash, csrf string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	source := NormalizeAttributionSource(o.Source)
	err = tx.QueryRow(ctx, `INSERT INTO owners(issuer,subject,email,name,source) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(issuer,subject) DO UPDATE SET email=EXCLUDED.email,name=EXCLUDED.name,
source=CASE WHEN EXCLUDED.source <> 'direct' THEN EXCLUDED.source ELSE owners.source END
RETURNING id`, o.Issuer, o.Subject, o.Email, o.Name, source).Scan(&o.ID)
	if err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(o.Email))
	if email != "" {
		// A verified sign-in links an existing maker with this email when no
		// other owner already holds it. It does not take a publisher site.
		_, err = tx.Exec(ctx, `UPDATE owners o SET maker_id = picked.id
FROM (
  SELECT m.id FROM makers m
  WHERE lower(m.email) = lower($2)
    AND NOT EXISTS (SELECT 1 FROM owners other WHERE other.maker_id = m.id)
  ORDER BY m.created_at ASC
  LIMIT 1
) picked
WHERE o.id = $1 AND o.maker_id IS NULL`, o.ID, email)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO owner_sessions VALUES($1,$2,$3,NOW()+INTERVAL '8 hours')`, tokenHash, o.ID, csrf)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DB) OwnerSession(ctx context.Context, hash string) (*Owner, error) {
	o, _, err := d.OwnerSessionExpiry(ctx, hash)
	return o, err
}

// OwnerSessionExpiry returns the signed-in owner and the session expiry.
// Sessions last eight hours from the sign-in that created them.
func (d *DB) OwnerSessionExpiry(ctx context.Context, hash string) (*Owner, time.Time, error) {
	var o Owner
	var expires time.Time
	err := d.Pool.QueryRow(ctx, `SELECT o.id,o.issuer,o.subject,o.email,o.name,COALESCE(o.site_id::text,''),COALESCE(o.maker_id::text,''),s.csrf,s.expires_at
FROM owner_sessions s JOIN owners o ON o.id=s.owner_id WHERE s.token_hash=$1 AND s.expires_at>NOW()`, hash).Scan(&o.ID, &o.Issuer, &o.Subject, &o.Email, &o.Name, &o.SiteID, &o.MakerID, &o.CSRF, &expires)
	return &o, expires, err
}

func (d *DB) EndOwnerSession(ctx context.Context, hash string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM owner_sessions WHERE token_hash=$1`, hash)
	return err
}

// OwnerNotify returns the owner's deal-waiting webhook url and signing secret.
func (d *DB) OwnerNotify(ctx context.Context, ownerID string) (url, secret string, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT notify_url, notify_secret FROM owners WHERE id=$1`, ownerID).Scan(&url, &secret)
	return url, secret, err
}

// SetOwnerNotify stores the webhook url; the secret rotates only when a new
// one is given so an empty secret keeps the existing one.
func (d *DB) SetOwnerNotify(ctx context.Context, ownerID, url, secret string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE owners SET notify_url=$2,
  notify_secret = CASE WHEN $3='' THEN notify_secret ELSE $3 END WHERE id=$1`, ownerID, url, secret)
	return err
}

func (d *DB) BindOwnerAccount(ctx context.Context, ownerID, role, accountID string) (bool, error) {
	query := `UPDATE owners SET site_id=$2 WHERE id=$1 AND site_id IS NULL`
	if role == "maker" {
		query = `UPDATE owners SET maker_id=$2 WHERE id=$1 AND maker_id IS NULL`
	} else if role != "publisher" {
		return false, errors.New("invalid account role")
	}
	res, err := d.Pool.Exec(ctx, query, ownerID, accountID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) CreateOwnerAccount(ctx context.Context, o *Owner, role, domain, keyHash, keyPrefix string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var siteID, makerID string
	err = tx.QueryRow(ctx, `SELECT COALESCE(site_id::text,''),COALESCE(maker_id::text,'') FROM owners WHERE id=$1 FOR UPDATE`, o.ID).Scan(&siteID, &makerID)
	if err != nil {
		return err
	}
	var id string
	switch role {
	case "publisher":
		if siteID != "" {
			return errors.New("publisher account already linked")
		}
		err = tx.QueryRow(ctx, `INSERT INTO sites(api_key_hash,api_key_prefix,domain,name) VALUES($1,$2,$3,$4) RETURNING id`, keyHash, keyPrefix, domain, o.Name).Scan(&id)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE owners SET site_id=$2 WHERE id=$1`, o.ID, id)
		}
		if err == nil {
			err = seedNewSitePublishAsk(ctx, tx, o.ID, id)
		}
	case "maker":
		if makerID != "" {
			return errors.New("maker account already linked")
		}
		err = tx.QueryRow(ctx, `INSERT INTO makers(email,name) VALUES($1,$2) RETURNING id`, strings.ToLower(o.Email), o.Name).Scan(&id)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE owners SET maker_id=$2 WHERE id=$1`, o.ID, id)
		}
	default:
		return errors.New("invalid account role")
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DB) SaveOAuthClient(ctx context.Context, c OAuthClient) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO oauth_clients(id,name,redirect_uris) VALUES($1,$2,$3)`, c.ID, c.Name, c.RedirectURIs)
	return err
}

func (d *DB) OAuthClient(ctx context.Context, id string) (OAuthClient, error) {
	var c OAuthClient
	err := d.Pool.QueryRow(ctx, `SELECT id,name,redirect_uris FROM oauth_clients WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.RedirectURIs)
	return c, err
}

func (d *DB) SaveOAuthRequest(ctx context.Context, a OAuthRequest) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO oauth_requests VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW()+INTERVAL '5 minutes')`, a.ID, a.OwnerID, a.ClientID, a.RedirectURI, a.Resource, a.Scope, a.Challenge, a.State)
	return err
}

func (d *DB) AuthorizeAgent(ctx context.Context, requestID, ownerID, codeHash string) (OAuthRequest, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return OAuthRequest{}, err
	}
	defer tx.Rollback(ctx)
	var a OAuthRequest
	err = tx.QueryRow(ctx, `DELETE FROM oauth_requests WHERE id=$1 AND owner_id=$2 AND expires_at>NOW()
RETURNING id,owner_id,client_id,redirect_uri,resource,scope,challenge,state`, requestID, ownerID).Scan(&a.ID, &a.OwnerID, &a.ClientID, &a.RedirectURI, &a.Resource, &a.Scope, &a.Challenge, &a.State)
	if err != nil {
		return a, err
	}
	var siteID, makerID *string
	err = tx.QueryRow(ctx, `SELECT site_id::text,maker_id::text FROM owners WHERE id=$1 FOR UPDATE`, ownerID).Scan(&siteID, &makerID)
	if err != nil {
		return a, err
	}
	if !strings.Contains(" "+a.Scope+" ", " publisher ") {
		siteID = nil
	}
	if !strings.Contains(" "+a.Scope+" ", " maker ") {
		makerID = nil
	}
	if strings.Contains(" "+a.Scope+" ", " publisher ") && siteID == nil || strings.Contains(" "+a.Scope+" ", " maker ") && makerID == nil {
		return a, errors.New("account not linked")
	}
	var grantID string
	err = tx.QueryRow(ctx, `INSERT INTO oauth_grants(owner_id,client_id,resource,scope,site_id,maker_id,expires_at)
VALUES($1,$2,$3,$4,$5,$6,NOW()+INTERVAL '7 days') RETURNING id`, ownerID, a.ClientID, a.Resource, a.Scope, siteID, makerID).Scan(&grantID)
	if err != nil {
		return a, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO oauth_codes VALUES($1,$2,$3,$4,NOW()+INTERVAL '5 minutes',NULL)`, codeHash, grantID, a.RedirectURI, a.Challenge)
	if err != nil {
		return a, err
	}
	return a, tx.Commit(ctx)
}

func (d *DB) ExchangeAgentCode(ctx context.Context, hash, clientID, redirectURI, resource, challenge, accessHash, refreshHash string) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var grantID, scope string
	err = tx.QueryRow(ctx, `UPDATE oauth_codes c SET consumed_at=NOW() FROM oauth_grants g
WHERE c.code_hash=$1 AND c.grant_id=g.id AND g.client_id=$2 AND c.redirect_uri=$3 AND g.resource=$4 AND c.challenge=$5
AND c.consumed_at IS NULL AND c.expires_at>NOW() AND g.revoked_at IS NULL AND g.expires_at>NOW()
AND (g.maker_id IS NULL OR EXISTS(SELECT 1 FROM makers WHERE id=g.maker_id AND status='active')) RETURNING g.id,g.scope`, hash, clientID, redirectURI, resource, challenge).Scan(&grantID, &scope)
	if err != nil {
		return "", err
	}
	if err = insertAgentTokens(ctx, tx, grantID, accessHash, refreshHash); err != nil {
		return "", err
	}
	return scope, tx.Commit(ctx)
}

func insertAgentTokens(ctx context.Context, tx pgx.Tx, grantID, accessHash, refreshHash string) error {
	_, err := tx.Exec(ctx, `INSERT INTO oauth_tokens(token_hash,grant_id,kind,expires_at)
SELECT $2,id,'access',LEAST(expires_at,NOW()+INTERVAL '1 hour') FROM oauth_grants WHERE id=$1
UNION ALL SELECT $3,id,'refresh',expires_at FROM oauth_grants WHERE id=$1`, grantID, accessHash, refreshHash)
	return err
}

func (d *DB) RefreshAgentToken(ctx context.Context, hash, clientID, resource, accessHash, refreshHash string) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var grantID, scope string
	var consumed *time.Time
	err = tx.QueryRow(ctx, `SELECT g.id,g.scope,t.consumed_at FROM oauth_tokens t JOIN oauth_grants g ON g.id=t.grant_id
WHERE t.token_hash=$1 AND t.kind='refresh' AND g.client_id=$2 AND g.resource=$3
AND t.expires_at>NOW() AND g.expires_at>NOW() AND g.revoked_at IS NULL
AND (g.maker_id IS NULL OR EXISTS(SELECT 1 FROM makers WHERE id=g.maker_id AND status='active')) FOR UPDATE OF t,g`, hash, clientID, resource).Scan(&grantID, &scope, &consumed)
	if err != nil {
		return "", err
	}
	if consumed != nil {
		if _, err = tx.Exec(ctx, `UPDATE oauth_grants SET revoked_at=NOW() WHERE id=$1`, grantID); err != nil {
			return "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", errors.New("refresh token already used")
	}
	if _, err = tx.Exec(ctx, `UPDATE oauth_tokens SET consumed_at=NOW() WHERE token_hash=$1`, hash); err != nil {
		return "", err
	}
	if err = insertAgentTokens(ctx, tx, grantID, accessHash, refreshHash); err != nil {
		return "", err
	}
	return scope, tx.Commit(ctx)
}

// AgentAccountBearer accepts the api_key returned by POST /v1/agent-accounts.
// A publisher key is scope publisher for that site. A maker key is scope maker
// for that maker. Plugin keys and ordinary site keys are not grants.
func (d *DB) AgentAccountBearer(ctx context.Context, keyHash, rawKey, resource string) (*AgentGrant, error) {
	if keyHash == "" || rawKey == "" {
		return nil, errors.New("missing agent account key")
	}
	var siteID string
	err := d.Pool.QueryRow(ctx, `
SELECT s.id::text
FROM sites s
JOIN owners o ON o.site_id = s.id AND o.agent_only
WHERE s.api_key_hash = $1
LIMIT 1`, keyHash).Scan(&siteID)
	if err == nil {
		return &AgentGrant{ClientName: "agent account", Resource: resource, Scope: "publisher", SiteID: siteID, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var makerID string
	err = d.Pool.QueryRow(ctx, `
SELECT m.id::text
FROM makers m
JOIN owners o ON o.maker_id = m.id AND o.agent_only
WHERE m.status = 'active' AND (m.api_key_hash = $1 OR (m.api_key_prev_hash = $1 AND m.api_key_prev_expires_at > NOW()))
LIMIT 1`, keyHash).Scan(&makerID)
	if err != nil {
		return nil, err
	}
	return &AgentGrant{ClientName: "agent account", Resource: resource, Scope: "maker", MakerID: makerID, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (d *DB) AgentToken(ctx context.Context, hash, resource string) (*AgentGrant, error) {
	var g AgentGrant
	err := d.Pool.QueryRow(ctx, `SELECT g.id,g.client_id,c.name,g.resource,g.scope,COALESCE(g.site_id::text,''),COALESCE(g.maker_id::text,''),LEAST(t.expires_at,g.expires_at),COALESCE(g.owner_id::text,'')
FROM oauth_tokens t JOIN oauth_grants g ON g.id=t.grant_id JOIN oauth_clients c ON c.id=g.client_id
WHERE t.token_hash=$1 AND g.resource=$2 AND t.kind='access' AND t.expires_at>NOW() AND g.expires_at>NOW() AND g.revoked_at IS NULL
AND (g.maker_id IS NULL OR EXISTS(SELECT 1 FROM makers WHERE id=g.maker_id AND status='active'))`, hash, resource).Scan(&g.ID, &g.ClientID, &g.ClientName, &g.Resource, &g.Scope, &g.SiteID, &g.MakerID, &g.ExpiresAt, &g.OwnerID)
	return &g, err
}

func (d *DB) OwnerGrants(ctx context.Context, ownerID string) ([]AgentGrant, error) {
	rows, err := d.Pool.Query(ctx, `SELECT g.id,c.name,g.scope,g.expires_at FROM oauth_grants g JOIN oauth_clients c ON c.id=g.client_id WHERE g.owner_id=$1 AND g.revoked_at IS NULL AND g.expires_at>NOW() ORDER BY g.expires_at DESC LIMIT 50`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentGrant{}
	for rows.Next() {
		var g AgentGrant
		if err = rows.Scan(&g.ID, &g.ClientName, &g.Scope, &g.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) RevokeOwnerGrant(ctx context.Context, ownerID, grantID string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE oauth_grants SET revoked_at=NOW() WHERE id=$1 AND owner_id=$2 AND revoked_at IS NULL`, grantID, ownerID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) ApproveOwnerPlacement(ctx context.Context, o *Owner, id string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE placement_proposals pp SET status='approved',approved_at=NOW(),approved_by=$3,
approved_by_kind='owner', matched_rule='owner',
policy_version=COALESCE((SELECT version FROM approval_policies WHERE site_id=pp.site_id),0),
approval_evidence=jsonb_build_object('method','owner_session','owner_id',$3::text,'issuer',$4::text,'subject',$5::text)::text
FROM offers f JOIN products p ON p.id=f.product_id JOIN makers m ON m.id=p.maker_id
WHERE pp.id=$1 AND pp.site_id=$2 AND pp.offer_id=f.id AND pp.status='pending' AND pp.expires_at>NOW()
AND lower(m.email)<>lower($6) AND NOT EXISTS(SELECT 1 FROM owners WHERE id=$3 AND maker_id=m.id)`, id, o.SiteID, o.ID, o.Issuer, o.Subject, o.Email)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

// CreatePartyClaim stores a single-use claim code for the owner. Any earlier
// unclaimed code for the same owner is replaced.
func (d *DB) CreatePartyClaim(ctx context.Context, ownerID, codeHash string, expires time.Time) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM orp_party_claims WHERE owner_id=$1 AND claimed_at IS NULL`, ownerID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO orp_party_claims(code_hash,owner_id,expires_at) VALUES($1,$2,$3)`, codeHash, ownerID, expires); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PartyClaimState describes a claim code for the claim page. OwnerAgentOnly
// tells the page whether claiming actually changes anything.
type PartyClaimState struct {
	OwnerID        string
	PartyID        string
	State          string // ok, expired, claimed, missing
	OwnerAgentOnly bool
	OwnerHuman     bool
}

func (d *DB) PartyClaimState(ctx context.Context, codeHash string, now time.Time) PartyClaimState {
	var s PartyClaimState
	var expires time.Time
	var claimed *time.Time
	err := d.Pool.QueryRow(ctx, `SELECT owner_id,expires_at,claimed_at FROM orp_party_claims WHERE code_hash=$1`, codeHash).Scan(&s.OwnerID, &expires, &claimed)
	if err != nil {
		s.State = "missing"
		return s
	}
	s.State = "ok"
	if claimed != nil {
		s.State = "claimed"
		return s
	}
	if now.After(expires) {
		s.State = "expired"
		return s
	}
	_ = d.Pool.QueryRow(ctx, `SELECT COALESCE(agent_only,false),COALESCE(human_login,false) FROM owners WHERE id=$1`, s.OwnerID).Scan(&s.OwnerAgentOnly, &s.OwnerHuman)
	return s
}

// ClaimParty moves the person's sign-in identity onto the claim's agent
// owner and starts a session for it. The person's former owner row keeps
// its site/maker links when the agent owner already has its own.
// Returns the claimed owner id or a problem word: expired, claimed,
// missing, taken.
func (d *DB) ClaimParty(ctx context.Context, codeHash, personOwnerID, tokenHash, csrf string, now time.Time) (string, string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	var agentID string
	var expires time.Time
	var claimed *time.Time
	err = tx.QueryRow(ctx, `SELECT owner_id,expires_at,claimed_at FROM orp_party_claims WHERE code_hash=$1 FOR UPDATE`, codeHash).Scan(&agentID, &expires, &claimed)
	if err != nil {
		return "", "missing", nil
	}
	if claimed != nil {
		return "", "claimed", nil
	}
	if now.After(expires) {
		return "", "expired", nil
	}
	var human bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE(human_login,false) FROM owners WHERE id=$1 FOR UPDATE`, agentID).Scan(&human); err != nil {
		return "", "", err
	}
	if human && agentID != personOwnerID {
		return "", "taken", nil
	}
	if agentID == personOwnerID {
		if _, err = tx.Exec(ctx, `UPDATE orp_party_claims SET claimed_at=$2,claimed_by=$3 WHERE code_hash=$1`, codeHash, now, personOwnerID); err != nil {
			return "", "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO owner_sessions(token_hash,owner_id,csrf,expires_at) VALUES($1,$2,$3,$4)`, tokenHash, agentID, csrf, now.Add(8*time.Hour)); err != nil {
			return "", "", err
		}
		return agentID, "", tx.Commit(ctx)
	}
	var issuer, subject string
	var email, name *string
	var siteID, makerID *string
	if err = tx.QueryRow(ctx, `SELECT issuer,subject,email,name,site_id,maker_id FROM owners WHERE id=$1 FOR UPDATE`, personOwnerID).Scan(&issuer, &subject, &email, &name, &siteID, &makerID); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE owners SET issuer='claimed:'||id::text, subject='claimed:'||id::text, site_id=NULL, maker_id=NULL WHERE id=$1`, personOwnerID); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE owners SET issuer=$2, subject=$3, email=$4, name=$5,
site_id=COALESCE(site_id,$6), maker_id=COALESCE(maker_id,$7),
human_login=true, human_login_at=COALESCE(human_login_at,$8) WHERE id=$1`, agentID, issuer, subject, email, name, siteID, makerID, now); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE orp_party_claims SET claimed_at=$2,claimed_by=$3 WHERE code_hash=$1`, codeHash, now, personOwnerID); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO owner_sessions(token_hash,owner_id,csrf,expires_at) VALUES($1,$2,$3,$4)`, tokenHash, agentID, csrf, now.Add(8*time.Hour)); err != nil {
		return "", "", err
	}
	return agentID, "", tx.Commit(ctx)
}
