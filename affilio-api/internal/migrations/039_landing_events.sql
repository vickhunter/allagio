-- Landing conversion events from the public pages. Apply by hand.
-- AUTO_MIGRATE stays off in production. This file is safe to run more than once.

CREATE TABLE IF NOT EXISTS landing_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (name IN ('landing_view', 'signin_click', 'zip_download')),
    source TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT landing_events_source_tag CHECK (source = '' OR source ~ '^[a-z0-9][a-z0-9_-]{0,63}$')
);

CREATE INDEX IF NOT EXISTS landing_events_name_created ON landing_events (name, created_at DESC);
