-- Trust of a sale or refund is fixed by how the event arrived.
-- provider_verified is a provider-signed event. seller_signed_webhook is a seller HMAC webhook.
-- seller_reported is an API or manual report. A refund applies only at the sale's level or higher.

ALTER TABLE conversions ADD COLUMN IF NOT EXISTS trust_level TEXT;
ALTER TABLE conversions ADD COLUMN IF NOT EXISTS refund_status TEXT;
ALTER TABLE conversions ADD COLUMN IF NOT EXISTS refund_reason TEXT;

UPDATE conversions SET trust_level = CASE
    WHEN provider = 'stripe' AND payment_mode IN ('test', 'live') THEN 'provider_verified'
    WHEN payment_mode = 'legacy' THEN 'seller_signed_webhook'
    ELSE 'seller_reported'
END
WHERE trust_level IS NULL;

UPDATE conversions SET refund_status = 'applied'
WHERE kind = 'refund' AND refund_status IS NULL;

ALTER TABLE conversions ALTER COLUMN trust_level SET DEFAULT 'seller_reported';
ALTER TABLE conversions ALTER COLUMN trust_level SET NOT NULL;

ALTER TABLE conversions DROP CONSTRAINT IF EXISTS conversions_trust_level_check;
ALTER TABLE conversions ADD CONSTRAINT conversions_trust_level_check
    CHECK (trust_level IN ('provider_verified', 'seller_signed_webhook', 'seller_reported'));

ALTER TABLE conversions DROP CONSTRAINT IF EXISTS conversions_refund_status_check;
ALTER TABLE conversions ADD CONSTRAINT conversions_refund_status_check
    CHECK (refund_status IS NULL OR refund_status IN ('applied', 'pending_review', 'rejected'));
