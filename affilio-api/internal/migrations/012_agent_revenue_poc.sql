CREATE TABLE product_connections (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    secret_ciphertext BYTEA NOT NULL,
    live_mode BOOLEAN NOT NULL,
    currency VARCHAR(3) NOT NULL CHECK (currency IN ('USD','EUR','GBP')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE webhook_events ADD COLUMN product_id UUID REFERENCES products(id);
ALTER TABLE webhook_events ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE webhook_events ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE INDEX idx_webhook_pending ON webhook_events(next_attempt_at) WHERE processed_at IS NULL;
ALTER TABLE clicks ADD COLUMN terms_snapshot JSONB;
ALTER TABLE conversions ADD COLUMN gross_minor BIGINT;
ALTER TABLE conversions ADD COLUMN payment_mode VARCHAR(10) NOT NULL DEFAULT 'legacy' CHECK(payment_mode IN ('test','live','legacy'));
CREATE UNIQUE INDEX idx_stripe_payment_once ON conversions(provider_object_id)
    WHERE provider='stripe' AND kind='sale';
ALTER TABLE payouts ADD COLUMN received_at TIMESTAMPTZ;
ALTER TABLE payouts ADD COLUMN maker_id UUID REFERENCES makers(id);
ALTER TABLE payouts ADD COLUMN payment_mode VARCHAR(10) NOT NULL DEFAULT 'legacy';
ALTER TABLE campaigns ADD COLUMN requires_placement BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE placement_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    offer_id UUID NOT NULL REFERENCES offers(id),
    surface_url TEXT NOT NULL,
    context TEXT NOT NULL,
    copy TEXT NOT NULL,
    disclosure TEXT NOT NULL,
    reason TEXT NOT NULL,
    terms_snapshot JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','committed','revoked')),
    approval_evidence TEXT,
    approved_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    campaign_id UUID REFERENCES campaigns(id),
    receipt JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_placement_owner ON placement_proposals(site_id, created_at DESC);
ALTER TABLE clicks ADD COLUMN placement_id UUID REFERENCES placement_proposals(id);
CREATE INDEX idx_click_placement ON clicks(placement_id) WHERE placement_id IS NOT NULL;

CREATE TABLE publisher_agent_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
