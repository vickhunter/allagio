-- Distributoor scale: fraud flags on conversions and trusted-maker fast path.

ALTER TABLE conversions ADD COLUMN IF NOT EXISTS flagged BOOLEAN DEFAULT false;
ALTER TABLE conversions ADD COLUMN IF NOT EXISTS flag_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_conversions_flagged ON conversions(flagged) WHERE flagged;

ALTER TABLE makers ADD COLUMN IF NOT EXISTS trusted BOOLEAN DEFAULT false;
