-- Distributoor attribution & payouts: raw webhook storage, conversions joined
-- back to clicks, and a commission ledger with refund-window holds.

ALTER TABLE clicks ADD COLUMN IF NOT EXISTS campaign_id UUID REFERENCES campaigns(id);

CREATE INDEX IF NOT EXISTS idx_clicks_campaign ON clicks(campaign_id);

CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    processed_at TIMESTAMP,
    error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider, provider_event_id)
);

CREATE TABLE IF NOT EXISTS conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id UUID REFERENCES offers(id),
    campaign_id UUID REFERENCES campaigns(id),
    click_id UUID REFERENCES clicks(id),
    provider VARCHAR(50) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    provider_object_id VARCHAR(255),
    kind VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    commission_amount DECIMAL(10,2),
    occurred_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider, provider_event_id)
);

CREATE INDEX IF NOT EXISTS idx_conversions_campaign ON conversions(campaign_id);
CREATE INDEX IF NOT EXISTS idx_conversions_object ON conversions(provider, provider_object_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id),
    conversion_id UUID REFERENCES conversions(id),
    kind VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(20) DEFAULT 'pending',
    clears_at TIMESTAMP,
    payout_id UUID,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_site ON ledger_entries(site_id);
CREATE INDEX IF NOT EXISTS idx_ledger_status ON ledger_entries(status);

CREATE TABLE IF NOT EXISTS payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id),
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    method VARCHAR(50),
    provider_reference VARCHAR(255),
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW(),
    paid_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payouts_site ON payouts(site_id);
