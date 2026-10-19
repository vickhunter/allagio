-- D-062 per-step autonomy. Apply by hand.
-- AUTO_MIGRATE stays off. This file is safe to run more than once.
-- A missing row means every step is auto. This file does not rewrite existing rows.

CREATE TABLE IF NOT EXISTS agent_step_modes (
    scope_key TEXT PRIMARY KEY,
    owner_id UUID REFERENCES owners(id) ON DELETE CASCADE,
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    site_signup TEXT NOT NULL DEFAULT 'auto',
    offer_match TEXT NOT NULL DEFAULT 'auto',
    placement_publish TEXT NOT NULL DEFAULT 'auto',
    link_change TEXT NOT NULL DEFAULT 'auto',
    payout TEXT NOT NULL DEFAULT 'auto',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_step_modes_scope CHECK (
        (owner_id IS NOT NULL AND site_id IS NOT NULL AND scope_key = owner_id::text || ':' || site_id::text)
        OR (owner_id IS NOT NULL AND site_id IS NULL AND scope_key = 'owner:' || owner_id::text)
        OR (owner_id IS NULL AND site_id IS NOT NULL AND scope_key = 'site:' || site_id::text)
    ),
    CONSTRAINT agent_step_modes_site_signup CHECK (site_signup IN ('auto', 'ask', 'off')),
    CONSTRAINT agent_step_modes_offer_match CHECK (offer_match IN ('auto', 'ask', 'off')),
    CONSTRAINT agent_step_modes_placement_publish CHECK (placement_publish IN ('auto', 'ask', 'off')),
    CONSTRAINT agent_step_modes_link_change CHECK (link_change IN ('auto', 'ask', 'off')),
    CONSTRAINT agent_step_modes_payout CHECK (payout IN ('auto', 'ask', 'off'))
);

CREATE INDEX IF NOT EXISTS agent_step_modes_site ON agent_step_modes (site_id) WHERE site_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS agent_step_modes_owner ON agent_step_modes (owner_id) WHERE owner_id IS NOT NULL;

-- Ask for site signup, offer match, link change, and payout waits here.
-- Placement publish reuses placement_proposals status pending and the operator approve route.
CREATE TABLE IF NOT EXISTS agent_step_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step TEXT NOT NULL,
    owner_id UUID REFERENCES owners(id) ON DELETE CASCADE,
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ,
    CONSTRAINT agent_step_approvals_step CHECK (step IN ('site_signup', 'offer_match', 'placement_publish', 'link_change', 'payout')),
    CONSTRAINT agent_step_approvals_status CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS agent_step_approvals_pending
    ON agent_step_approvals (created_at)
    WHERE status = 'pending';
