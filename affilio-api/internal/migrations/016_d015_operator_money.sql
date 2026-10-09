-- D-015 additive hardening.
-- Apply only after a Neon snapshot of the database. This file adds columns
-- and tables. It does not rename or drop anything.
-- Snapshot requirement: take a Neon snapshot (or an equivalent logical backup)
-- and confirm it can be restored before applying this migration in any
-- deployed database. Do not deploy this migration without that snapshot.

CREATE TABLE operators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    owner_id UUID UNIQUE REFERENCES owners(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE placement_proposals ADD COLUMN approved_by_operator UUID REFERENCES operators(id);
ALTER TABLE payouts ADD COLUMN marked_paid_by UUID REFERENCES operators(id);

ALTER TABLE sites ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE sites ADD COLUMN payment_fingerprint TEXT;
ALTER TABLE makers ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE makers ADD COLUMN payment_fingerprint TEXT;

ALTER TABLE offers ADD COLUMN commission_value_minor BIGINT;
ALTER TABLE conversions ADD COLUMN amount_minor BIGINT;
ALTER TABLE conversions ADD COLUMN commission_minor BIGINT;
ALTER TABLE ledger_entries ADD COLUMN amount_minor BIGINT;
ALTER TABLE payouts ADD COLUMN amount_minor BIGINT;

UPDATE offers SET commission_value_minor = ROUND(commission_value * 100)::bigint WHERE commission_value_minor IS NULL;
UPDATE conversions SET amount_minor = ROUND(amount * 100)::bigint WHERE amount_minor IS NULL;
UPDATE conversions SET commission_minor = ROUND(COALESCE(commission_amount, 0) * 100)::bigint WHERE commission_minor IS NULL;
UPDATE ledger_entries SET amount_minor = ROUND(amount * 100)::bigint WHERE amount_minor IS NULL;
UPDATE payouts SET amount_minor = ROUND(amount * 100)::bigint WHERE amount_minor IS NULL;
