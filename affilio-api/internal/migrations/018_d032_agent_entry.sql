-- D-032 agent entry and D-031 approval modes. Additive only.

ALTER TABLE owners DROP CONSTRAINT owners_source_allowed;
ALTER TABLE owners ADD CONSTRAINT owners_source_allowed
    CHECK (source IN ('chatgpt', 'claude', 'cursor', 'grok', 'x', 'community', 'direct', 'cli', 'mcp'));

ALTER TABLE owners ADD COLUMN agent_only BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE owners ADD COLUMN human_login BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE owners ADD COLUMN human_login_at TIMESTAMPTZ;

ALTER TABLE placement_proposals ADD COLUMN approved_by_kind TEXT;
ALTER TABLE placement_proposals ADD CONSTRAINT placement_proposals_approved_by_kind
    CHECK (approved_by_kind IS NULL OR approved_by_kind IN ('owner', 'policy:auto', 'policy:yolo'));
ALTER TABLE placement_proposals ADD COLUMN policy_version INTEGER;
ALTER TABLE placement_proposals ADD COLUMN matched_rule TEXT;
ALTER TABLE placement_proposals ADD COLUMN auto_paused BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE device_authorizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code_hash VARCHAR(64) NOT NULL UNIQUE,
    user_code_hash VARCHAR(64) NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied', 'consumed')),
    interval_seconds INTEGER NOT NULL DEFAULT 5,
    expires_at TIMESTAMPTZ NOT NULL,
    last_polled_at TIMESTAMPTZ,
    owner_id UUID REFERENCES owners(id),
    scope TEXT CHECK (scope IS NULL OR scope IN ('site', 'offer')),
    site_id UUID REFERENCES sites(id),
    offer_id UUID REFERENCES offers(id),
    maker_id UUID REFERENCES makers(id),
    approved_at TIMESTAMPTZ,
    denied_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(8) NOT NULL,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK (scope IN ('site', 'offer')),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    offer_id UUID REFERENCES offers(id) ON DELETE CASCADE,
    maker_id UUID REFERENCES makers(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (scope = 'site' AND site_id IS NOT NULL AND offer_id IS NULL AND maker_id IS NULL)
        OR (scope = 'offer' AND offer_id IS NOT NULL AND maker_id IS NOT NULL AND site_id IS NULL)
    )
);
CREATE INDEX idx_project_keys_hash ON project_keys(key_hash) WHERE revoked_at IS NULL;

CREATE TABLE owner_setup (
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('maker', 'publisher')),
    install_path TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('manual', 'auto', 'yolo')),
    mode_shown TEXT NOT NULL CHECK (mode_shown IN ('manual', 'auto', 'yolo')),
    promote_text TEXT,
    avoid_topics TEXT[] NOT NULL DEFAULT '{}',
    commission TEXT,
    sites_channels TEXT[] NOT NULL DEFAULT '{}',
    never_topics TEXT[] NOT NULL DEFAULT '{}',
    box_position TEXT,
    saved_by UUID NOT NULL REFERENCES owners(id),
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (owner_id, role)
);

CREATE TABLE approval_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope TEXT NOT NULL CHECK (scope IN ('site', 'offer')),
    site_id UUID UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
    offer_id UUID UNIQUE REFERENCES offers(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('manual', 'auto', 'yolo')),
    version INTEGER NOT NULL DEFAULT 1,
    allow_makers TEXT[] NOT NULL DEFAULT '{}',
    deny_makers TEXT[] NOT NULL DEFAULT '{}',
    allow_categories TEXT[] NOT NULL DEFAULT '{}',
    deny_categories TEXT[] NOT NULL DEFAULT '{}',
    min_relevance INTEGER NOT NULL DEFAULT 40 CHECK (min_relevance >= 25 AND min_relevance <= 100),
    allowed_positions TEXT[] NOT NULL DEFAULT '{}',
    max_placements_per_30d INTEGER NOT NULL DEFAULT 3 CHECK (max_placements_per_30d >= 0 AND max_placements_per_30d <= 100),
    copy_edits_allowed BOOLEAN NOT NULL DEFAULT false,
    min_site_quality INTEGER NOT NULL DEFAULT 0 CHECK (min_site_quality >= 0 AND min_site_quality <= 100),
    allowed_niches TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (scope = 'site' AND site_id IS NOT NULL AND offer_id IS NULL)
        OR (scope = 'offer' AND offer_id IS NOT NULL AND site_id IS NULL)
    )
);

CREATE TABLE mode_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID,
    offer_id UUID,
    actor TEXT NOT NULL CHECK (actor IN ('owner_ui', 'agent_tool')),
    user_id UUID,
    from_mode TEXT,
    to_mode TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_mode_changes_site ON mode_changes(site_id, created_at DESC);

CREATE TABLE publisher_acceptances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id UUID NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    approved_by_kind TEXT NOT NULL CHECK (approved_by_kind IN ('owner', 'policy:auto', 'policy:yolo')),
    policy_version INTEGER NOT NULL,
    matched_rule TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (offer_id, site_id)
);

CREATE TABLE listing_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    maker_id UUID REFERENCES makers(id) ON DELETE CASCADE,
    owner_id UUID REFERENCES owners(id),
    product_id UUID REFERENCES products(id) ON DELETE SET NULL,
    card JSONB NOT NULL,
    signals JSONB NOT NULL,
    score INTEGER NOT NULL CHECK (score >= 0 AND score <= 100),
    payments_check TEXT NOT NULL CHECK (payments_check IN ('pass', 'fail', 'unknown')),
    offer_link_health TEXT NOT NULL CHECK (offer_link_health IN ('ok', 'broken', 'unknown')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE owner_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('digest_opened', 'dashboard_opened', 'placement_undo', 'offer_undo')),
    user_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    placement_id UUID,
    offer_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_owner_events_user ON owner_events(user_id, kind, created_at DESC);

CREATE TABLE undo_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('placement', 'offer')),
    placement_id UUID,
    offer_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE TABLE source_hits (
    source TEXT PRIMARY KEY,
    hits BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
