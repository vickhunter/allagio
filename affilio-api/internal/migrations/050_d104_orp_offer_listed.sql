-- D-104: a standing offer with list:ask is stored but not discoverable.
-- listed is false while the offerer's placement publish (the list step) is ask.
-- A saved auto sets listed true. Rows that already exist stay listed.
-- Discover and accept skip unlisted rows. Safe to run more than once.
-- AUTO_MIGRATE stays off unless it is set to true.
-- No money, no personal data, and CLAIMS_CHECK is unchanged.

ALTER TABLE orp_offers ADD COLUMN IF NOT EXISTS listed BOOLEAN NOT NULL DEFAULT true;
