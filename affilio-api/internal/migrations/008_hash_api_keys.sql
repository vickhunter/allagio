-- Hash API keys: store SHA-256 hash + short prefix for indexed lookup.
-- Previous key now has an expiry window after rotation.

-- Add new columns for hashed key storage
ALTER TABLE sites ADD COLUMN IF NOT EXISTS api_key_hash VARCHAR(64);
ALTER TABLE sites ADD COLUMN IF NOT EXISTS api_key_prefix VARCHAR(8);
ALTER TABLE sites ADD COLUMN IF NOT EXISTS api_key_prev_hash VARCHAR(64);
ALTER TABLE sites ADD COLUMN IF NOT EXISTS api_key_prev_prefix VARCHAR(8);
ALTER TABLE sites ADD COLUMN IF NOT EXISTS api_key_previous_expires_at TIMESTAMP;

-- Backfill hashes from existing plaintext keys using pgcrypto
-- SHA-256 hex digest of existing keys
CREATE EXTENSION IF NOT EXISTS pgcrypto;

UPDATE sites
SET api_key_hash = encode(digest(api_key, 'sha256'), 'hex'),
    api_key_prefix = LEFT(api_key, 8)
WHERE api_key IS NOT NULL AND api_key_hash IS NULL;

UPDATE sites
SET api_key_prev_hash = encode(digest(api_key_previous, 'sha256'), 'hex'),
    api_key_prev_prefix = LEFT(api_key_previous, 8)
WHERE api_key_previous IS NOT NULL AND api_key_prev_hash IS NULL;

-- Drop plaintext key columns
ALTER TABLE sites DROP COLUMN IF EXISTS api_key;
ALTER TABLE sites DROP COLUMN IF EXISTS api_key_previous;

-- Add indexes for prefix-based lookup
CREATE INDEX IF NOT EXISTS idx_sites_api_key_prefix ON sites(api_key_prefix);
CREATE INDEX IF NOT EXISTS idx_sites_api_key_prev_prefix ON sites(api_key_prev_prefix);

-- Add unique constraint on hash
ALTER TABLE sites ADD CONSTRAINT uq_sites_api_key_hash UNIQUE (api_key_hash);

-- Truncate plaintext keys in rotation audit table to prefixes only
ALTER TABLE site_key_rotations ADD COLUMN IF NOT EXISTS old_key_prefix VARCHAR(8);
ALTER TABLE site_key_rotations ADD COLUMN IF NOT EXISTS new_key_prefix VARCHAR(8);

UPDATE site_key_rotations
SET old_key_prefix = LEFT(old_key, 8),
    new_key_prefix = LEFT(new_key, 8)
WHERE old_key_prefix IS NULL;

ALTER TABLE site_key_rotations DROP COLUMN IF EXISTS old_key;
ALTER TABLE site_key_rotations DROP COLUMN IF EXISTS new_key;
