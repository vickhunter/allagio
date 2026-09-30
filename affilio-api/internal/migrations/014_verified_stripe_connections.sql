ALTER TABLE product_connections ADD COLUMN stripe_account_id TEXT;
ALTER TABLE product_connections ADD COLUMN stripe_price_id TEXT;
ALTER TABLE product_connections ADD COLUMN stripe_endpoint_id TEXT;
ALTER TABLE product_connections ADD COLUMN configuration_verified_at TIMESTAMPTZ;
