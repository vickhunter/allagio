CREATE TABLE party_keys (
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    kid TEXT NOT NULL,
    jwk JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (owner_id, kid)
);
