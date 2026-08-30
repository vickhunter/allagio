-- Distributoor core: makers, products, readiness assessments, offers, campaigns.
-- Supply side of the network; offers are promoted into affiliate_programs where
-- the existing matcher/recommendation pipeline picks them up unchanged.

CREATE TABLE IF NOT EXISTS makers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    api_key VARCHAR(64) UNIQUE NOT NULL,
    api_key_previous VARCHAR(64),
    api_key_rotated_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_makers_api_key_previous ON makers(api_key_previous);

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    maker_id UUID REFERENCES makers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    tagline VARCHAR(512),
    website_url VARCHAR(512) NOT NULL,
    docs_url VARCHAR(512),
    pricing_url VARCHAR(512),
    product_type VARCHAR(50) NOT NULL,
    payment_provider VARCHAR(50),
    primary_category VARCHAR(100),
    tags TEXT[],
    keywords TEXT[],
    readiness_score INTEGER DEFAULT 0,
    readiness_status VARCHAR(20) DEFAULT 'unassessed',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_maker ON products(maker_id);

CREATE TABLE IF NOT EXISTS readiness_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'running',
    score INTEGER,
    checks JSONB DEFAULT '[]',
    ai_summary TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    finished_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_readiness_assessments_product ON readiness_assessments(product_id);

CREATE TABLE IF NOT EXISTS offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'draft',
    commission_type VARCHAR(50) NOT NULL,
    commission_value DECIMAL(10,2) NOT NULL,
    commission_unit VARCHAR(10),
    commission_currency VARCHAR(3) DEFAULT 'USD',
    cookie_duration_days INTEGER DEFAULT 30,
    refund_window_days INTEGER DEFAULT 30,
    landing_url VARCHAR(512) NOT NULL,
    ref_param VARCHAR(50) DEFAULT 'via',
    terms TEXT,
    creative_assets JSONB DEFAULT '[]',
    program_id UUID REFERENCES affiliate_programs(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_offers_product ON offers(product_id);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers(status);

CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id UUID REFERENCES offers(id) ON DELETE CASCADE,
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'active',
    tracking_code VARCHAR(32) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(offer_id, site_id)
);

CREATE INDEX IF NOT EXISTS idx_campaigns_site ON campaigns(site_id);

ALTER TABLE affiliate_programs ADD COLUMN IF NOT EXISTS source VARCHAR(20) DEFAULT 'curated';
ALTER TABLE affiliate_programs ADD COLUMN IF NOT EXISTS offer_id UUID REFERENCES offers(id);

CREATE INDEX IF NOT EXISTS idx_programs_offer ON affiliate_programs(offer_id);
