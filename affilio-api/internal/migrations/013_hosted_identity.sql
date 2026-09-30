ALTER TABLE makers ALTER COLUMN api_key DROP NOT NULL;

CREATE TABLE owners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    site_id UUID UNIQUE REFERENCES sites(id) ON DELETE SET NULL,
    maker_id UUID UNIQUE REFERENCES makers(id) ON DELETE SET NULL,
    UNIQUE(issuer,subject)
);

CREATE TABLE owner_sessions (
    token_hash VARCHAR(64) PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    csrf TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE owner_login_attempts (
    state_hash VARCHAR(64) PRIMARY KEY,
    nonce TEXT NOT NULL,
    verifier TEXT NOT NULL,
    return_to TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE oauth_clients (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    redirect_uris TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_requests (
    id TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL REFERENCES oauth_clients(id),
    redirect_uri TEXT NOT NULL,
    resource TEXT NOT NULL,
    scope TEXT NOT NULL,
    challenge TEXT NOT NULL,
    state TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE oauth_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL REFERENCES oauth_clients(id),
    resource TEXT NOT NULL,
    scope TEXT NOT NULL,
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    maker_id UUID REFERENCES makers(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE oauth_codes (
    code_hash VARCHAR(64) PRIMARY KEY,
    grant_id UUID NOT NULL REFERENCES oauth_grants(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    challenge TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE TABLE oauth_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    grant_id UUID NOT NULL REFERENCES oauth_grants(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('access','refresh')),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE INDEX idx_oauth_grants_owner ON oauth_grants(owner_id);
ALTER TABLE placement_proposals ADD COLUMN approved_by UUID REFERENCES owners(id);
