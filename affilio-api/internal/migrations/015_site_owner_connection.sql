CREATE TABLE site_owner_connections (
    token_hash VARCHAR(64) PRIMARY KEY,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_site_owner_connections_expiry ON site_owner_connections(expires_at);
ALTER TABLE sites ADD COLUMN payout_instructions TEXT NOT NULL DEFAULT '';
