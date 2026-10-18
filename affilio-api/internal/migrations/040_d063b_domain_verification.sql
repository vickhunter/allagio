-- D-063b domain verification. Apply by hand.
-- AUTO_MIGRATE stays off in production. This file is safe to run more than once.
-- A site starts unverified. verified_at is set only after the domain serves
-- its token. One verified site per domain; other signups for that domain stay.

ALTER TABLE sites ADD COLUMN IF NOT EXISTS verification_token TEXT NOT NULL DEFAULT '';
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verification_host TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS sites_one_verified_domain
    ON sites (verification_host)
    WHERE verified_at IS NOT NULL AND verification_host <> '';
