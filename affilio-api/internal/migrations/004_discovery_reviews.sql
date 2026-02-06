CREATE TABLE IF NOT EXISTS program_discovery_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discovery_id UUID REFERENCES program_discovery_queue(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    note TEXT,
    reviewed_by VARCHAR(255),
    reviewed_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discovery_reviews_discovery ON program_discovery_reviews(discovery_id);
