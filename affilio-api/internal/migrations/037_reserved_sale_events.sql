-- A seller report keeps its event id after a signed sale upgrades that row.
-- Replaying the report cannot insert a second commission.

CREATE TABLE IF NOT EXISTS reserved_sale_events (
    offer_id UUID NOT NULL REFERENCES offers(id),
    provider_event_id VARCHAR(255) NOT NULL,
    conversion_id UUID NOT NULL REFERENCES conversions(id),
    PRIMARY KEY (offer_id, provider_event_id)
);

INSERT INTO reserved_sale_events (offer_id, provider_event_id, conversion_id)
SELECT offer_id, provider_event_id, id
FROM conversions
WHERE provider = 'seller_report'
  AND kind IN ('sale', 'renewal')
  AND offer_id IS NOT NULL
  AND provider_event_id IS NOT NULL
ON CONFLICT (offer_id, provider_event_id) DO NOTHING;
