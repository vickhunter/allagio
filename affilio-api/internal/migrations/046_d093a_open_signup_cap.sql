-- D-093a shared daily cap for POST /v1/agent-accounts.
-- Join tokens already store creator_ip_hash. This table counts account
-- signups in the same 24-hour window. Rows older than 24 hours are deleted.
-- Expired join-token hashes are cleared to an empty string. Additive.
-- AUTO_MIGRATE stays off unless it is set to true.

CREATE TABLE IF NOT EXISTS agent_open_signups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_ip_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_open_signups_ip_created
    ON agent_open_signups(creator_ip_hash, created_at);
