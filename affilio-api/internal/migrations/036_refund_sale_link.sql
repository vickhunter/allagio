-- A refund belongs to the sale row it matched. Two sales can share an order id
-- and an offer, and a refund of one does not change the other.

ALTER TABLE conversions ADD COLUMN IF NOT EXISTS refunded_conversion_id UUID;

ALTER TABLE conversions DROP CONSTRAINT IF EXISTS conversions_refunded_conversion_id_fkey;
ALTER TABLE conversions ADD CONSTRAINT conversions_refunded_conversion_id_fkey
    FOREIGN KEY (refunded_conversion_id) REFERENCES conversions(id);

CREATE INDEX IF NOT EXISTS conversions_refund_sale
    ON conversions (refunded_conversion_id)
    WHERE kind = 'refund';

UPDATE conversions AS refund
SET refunded_conversion_id = sale.id
FROM conversions AS sale
WHERE refund.kind = 'refund'
  AND refund.refunded_conversion_id IS NULL
  AND refund.provider_object_id IS NOT NULL
  AND refund.provider_object_id <> ''
  AND sale.kind IN ('sale', 'renewal')
  AND sale.provider_object_id = refund.provider_object_id
  AND sale.offer_id = refund.offer_id
  AND (
    SELECT COUNT(*) FROM conversions AS other
    WHERE other.kind IN ('sale', 'renewal')
      AND other.provider_object_id = refund.provider_object_id
      AND other.offer_id = refund.offer_id
  ) = 1;
