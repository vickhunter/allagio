-- D-093a self-serve agent join tokens. Additive. No backfill.
-- A token is stored only as a hash. The role is not chosen here: site_id and
-- maker_id on the owner stay empty until a later action fills them in.
-- AUTO_MIGRATE stays off unless it is set to true.

CREATE TABLE IF NOT EXISTS agent_join_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    token_prefix VARCHAR(8) NOT NULL,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    resource TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    creator_ip_hash VARCHAR(64) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_join_tokens_owner ON agent_join_tokens(owner_id);
CREATE INDEX IF NOT EXISTS idx_agent_join_tokens_ip_created ON agent_join_tokens(creator_ip_hash, created_at);
