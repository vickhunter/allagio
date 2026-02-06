CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS affiliate_programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    website_url VARCHAR(512),
    commission_type VARCHAR(50) NOT NULL,
    commission_value DECIMAL(10,2),
    commission_unit VARCHAR(10),
    commission_currency VARCHAR(3) DEFAULT 'USD',
    cookie_duration_days INTEGER,
    network VARCHAR(100),
    signup_url VARCHAR(512),
    referral_url VARCHAR(512),
    primary_category VARCHAR(100) NOT NULL,
    subcategories TEXT[],
    tags TEXT[],
    keywords TEXT[],
    avg_conversion_rate DECIMAL(5,2),
    min_payout DECIMAL(10,2),
    payment_frequency VARCHAR(50),
    program_quality_score INTEGER DEFAULT 50,
    is_verified BOOLEAN DEFAULT false,
    status VARCHAR(20) DEFAULT 'active',
    last_verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_programs_category ON affiliate_programs(primary_category);
CREATE INDEX IF NOT EXISTS idx_programs_tags ON affiliate_programs USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_programs_keywords ON affiliate_programs USING GIN(keywords);
CREATE INDEX IF NOT EXISTS idx_programs_status ON affiliate_programs(status);

CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key VARCHAR(64) UNIQUE NOT NULL,
    api_key_previous VARCHAR(64),
    api_key_rotated_at TIMESTAMP,
    domain VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    plan VARCHAR(20) DEFAULT 'free',
    scan_status VARCHAR(20) DEFAULT 'pending',
    total_articles INTEGER DEFAULT 0,
    last_scan_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sites_api_key_previous ON sites(api_key_previous);

CREATE TABLE IF NOT EXISTS articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    wp_post_id BIGINT NOT NULL,
    title VARCHAR(512),
    url VARCHAR(512),
    content_hash VARCHAR(64),
    analysis JSONB,
    monetization_score INTEGER DEFAULT 0,
    existing_affiliate_links JSONB DEFAULT '[]',
    analyzed_at TIMESTAMP,
    last_analyzed_at TIMESTAMP,
    disclosure_inserted BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(site_id, wp_post_id)
);

CREATE INDEX IF NOT EXISTS idx_articles_site ON articles(site_id);
CREATE INDEX IF NOT EXISTS idx_articles_score ON articles(monetization_score);

CREATE TABLE IF NOT EXISTS recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    article_id UUID REFERENCES articles(id) ON DELETE CASCADE,
    program_id UUID REFERENCES affiliate_programs(id),
    relevance_score INTEGER NOT NULL,
    estimated_monthly_revenue DECIMAL(10,2),
    reason TEXT NOT NULL,
    insertion_type VARCHAR(50),
    generated_html TEXT,
    suggested_position TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    inserted_at TIMESTAMP,
    approved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recommendations_article ON recommendations(article_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_status ON recommendations(status);

CREATE TABLE IF NOT EXISTS clicks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id),
    article_id UUID REFERENCES articles(id),
    recommendation_id UUID REFERENCES recommendations(id),
    program_id UUID REFERENCES affiliate_programs(id),
    clicked_at TIMESTAMP DEFAULT NOW(),
    ip_hash VARCHAR(64),
    user_agent TEXT,
    referrer VARCHAR(512)
);

CREATE INDEX IF NOT EXISTS idx_clicks_site ON clicks(site_id);
CREATE INDEX IF NOT EXISTS idx_clicks_date ON clicks(clicked_at);

CREATE TABLE IF NOT EXISTS site_affiliate_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    program_id UUID REFERENCES affiliate_programs(id),
    affiliate_link VARCHAR(512) NOT NULL,
    custom_id VARCHAR(255),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(site_id, program_id)
);

CREATE TABLE IF NOT EXISTS user_programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    program_guess VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_programs_site_domain ON user_programs(site_id, domain);

CREATE TABLE IF NOT EXISTS user_program_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_program_id UUID REFERENCES user_programs(id) ON DELETE CASCADE,
    affiliate_link VARCHAR(512) NOT NULL,
    params JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS link_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    article_id UUID REFERENCES articles(id) ON DELETE CASCADE,
    url VARCHAR(1024) NOT NULL,
    domain VARCHAR(255),
    is_affiliate BOOLEAN DEFAULT false,
    confidence DECIMAL(4,2) DEFAULT 0.0,
    method VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS program_discovery_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain VARCHAR(255) NOT NULL,
    sample_url VARCHAR(1024),
    sites_count INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_program_discovery_domain ON program_discovery_queue(domain);

CREATE TABLE IF NOT EXISTS scan_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'pending',
    total INTEGER DEFAULT 0,
    processed INTEGER DEFAULT 0,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    error TEXT
);

CREATE TABLE IF NOT EXISTS site_key_rotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    old_key VARCHAR(64),
    new_key VARCHAR(64),
    rotated_by VARCHAR(255),
    note TEXT,
    rotated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_site_key_rotations_site ON site_key_rotations(site_id);
