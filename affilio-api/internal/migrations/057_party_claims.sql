-- Per-party claim links. An agent-only owner mints a code with its own
-- bearer and hands the link to its human; the human signs in and takes
-- over that owner row (GAPS item 12). Codes are stored hashed, expire in
-- 24h, and are single use.
CREATE TABLE IF NOT EXISTS orp_party_claims (
    code_hash TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ,
    claimed_by UUID REFERENCES owners(id)
);
CREATE INDEX IF NOT EXISTS orp_party_claims_owner ON orp_party_claims(owner_id) WHERE claimed_at IS NULL;
