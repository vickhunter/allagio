-- Account-wide keys. Created only from a signed-in human session.
-- Project keys stay in project_keys and remain limited to one site or one offer.

CREATE TABLE account_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(8) NOT NULL,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_account_keys_owner ON account_keys(owner_id, created_at DESC);
