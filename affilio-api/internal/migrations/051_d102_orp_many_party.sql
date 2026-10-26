-- D-102: ORP 0.4 many-party document.
-- NULL means an ORP 0.3 two-party row. The 0.3 columns stay the 0.3 wire.
-- Seats, vacancies, tracking tokens, and click shares live here.
-- No email, name, or payment row. Offer Rails does not hold money.
-- Safe to run more than once.
ALTER TABLE orp_deals ADD COLUMN IF NOT EXISTS parties JSONB;

CREATE UNIQUE INDEX IF NOT EXISTS orp_deals_open_offer_acceptor_pending
    ON orp_deals (offer_id, acceptor_owner)
    WHERE offer_id IS NOT NULL AND state = 'pending';
