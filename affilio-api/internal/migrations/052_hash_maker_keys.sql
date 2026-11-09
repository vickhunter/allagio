-- Hash maker API keys: store SHA-256 hash + 8-char prefix, like sites (011).
-- Previous key gets an expiry window after rotation instead of living forever.

ALTER TABLE makers ADD COLUMN IF NOT EXISTS api_key_hash VARCHAR(64);
ALTER TABLE makers ADD COLUMN IF NOT EXISTS api_key_prefix VARCHAR(8);
ALTER TABLE makers ADD COLUMN IF NOT EXISTS api_key_prev_hash VARCHAR(64);
ALTER TABLE makers ADD COLUMN IF NOT EXISTS api_key_prev_expires_at TIMESTAMP;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='makers' AND column_name='api_key') THEN
        EXECUTE 'UPDATE makers SET api_key_hash = encode(digest(api_key, ''sha256''), ''hex''), api_key_prefix = LEFT(api_key, 8) WHERE api_key IS NOT NULL AND api_key_hash IS NULL';
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='makers' AND column_name='api_key_previous') THEN
        EXECUTE 'UPDATE makers SET api_key_prev_hash = encode(digest(api_key_previous, ''sha256''), ''hex'') WHERE api_key_previous IS NOT NULL AND api_key_prev_hash IS NULL';
    END IF;
END $$;

ALTER TABLE makers DROP COLUMN IF EXISTS api_key;
ALTER TABLE makers DROP COLUMN IF EXISTS api_key_previous;

CREATE INDEX IF NOT EXISTS idx_makers_api_key_prefix ON makers(api_key_prefix);
CREATE INDEX IF NOT EXISTS idx_makers_api_key_prev_hash ON makers(api_key_prev_hash);
