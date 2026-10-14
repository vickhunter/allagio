-- Seller reports share the provider name seller_report. An event id is unique
-- per offer for those reports, and stays unique per provider for signed webhooks.

ALTER TABLE conversions DROP CONSTRAINT IF EXISTS conversions_provider_provider_event_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS conversions_provider_event_global
    ON conversions (provider, provider_event_id)
    WHERE provider <> 'seller_report';

CREATE UNIQUE INDEX IF NOT EXISTS conversions_seller_report_event
    ON conversions (offer_id, provider_event_id)
    WHERE provider = 'seller_report';
