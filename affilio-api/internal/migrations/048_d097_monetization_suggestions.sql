-- D-097: a drafting suggestion stores a hash of the stripped text and the
-- keywords that overlapped an offer. The draft itself is not a column.
-- Emails, handles, and phone numbers are stripped before the hash.
-- Apply by hand. AUTO_MIGRATE stays off unless it is set to true.
-- Safe to run more than once.

CREATE TABLE IF NOT EXISTS monetization_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    draft_sha256 TEXT NOT NULL,
    keywords TEXT[] NOT NULL DEFAULT '{}',
    surface TEXT NOT NULL DEFAULT '',
    lang TEXT NOT NULL DEFAULT '',
    offer_ids TEXT[] NOT NULL DEFAULT '{}',
    mode TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monetization_suggestions_hash_hex CHECK (draft_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT monetization_suggestions_mode CHECK (mode IN ('auto', 'ask', 'off'))
);

CREATE TABLE IF NOT EXISTS monetization_suggestion_links (
    code TEXT PRIMARY KEY,
    suggestion_id UUID NOT NULL REFERENCES monetization_suggestions(id) ON DELETE CASCADE,
    offer_id TEXT NOT NULL,
    destination TEXT NOT NULL,
    clicks INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monetization_suggestion_links_https CHECK (destination LIKE 'https://%'),
    CONSTRAINT monetization_suggestion_links_code CHECK (code ~ '^sg[0-9a-f]{20}$')
);

CREATE INDEX IF NOT EXISTS idx_monetization_suggestions_owner
    ON monetization_suggestions(owner_id, created_at DESC);
