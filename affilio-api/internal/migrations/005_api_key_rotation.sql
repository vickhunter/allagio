ALTER TABLE sites
ADD COLUMN IF NOT EXISTS api_key_previous VARCHAR(64),
ADD COLUMN IF NOT EXISTS api_key_rotated_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_sites_api_key_previous ON sites(api_key_previous);

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
