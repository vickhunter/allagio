-- D-032 UI wiring. Digest display fields and structured setup answers.
-- Readiness signals and publisher types to avoid stay out until after the MVP.

ALTER TABLE placement_proposals ADD COLUMN position TEXT NOT NULL DEFAULT '';
ALTER TABLE placement_proposals ADD COLUMN post_title TEXT NOT NULL DEFAULT '';
ALTER TABLE placement_proposals ADD COLUMN link_text TEXT NOT NULL DEFAULT '';
ALTER TABLE placement_proposals ADD COLUMN label TEXT NOT NULL DEFAULT '';

ALTER TABLE owner_setup ADD COLUMN project_name TEXT NOT NULL DEFAULT '';
ALTER TABLE owner_setup ADD COLUMN listing_url TEXT NOT NULL DEFAULT '';
ALTER TABLE owner_setup ADD COLUMN commission_type TEXT NOT NULL DEFAULT '';
ALTER TABLE owner_setup ADD COLUMN commission_value NUMERIC;
ALTER TABLE owner_setup ADD COLUMN commission_currency TEXT NOT NULL DEFAULT '';
ALTER TABLE owner_setup ADD COLUMN listing_price_value NUMERIC;
ALTER TABLE owner_setup ADD COLUMN listing_price_currency TEXT NOT NULL DEFAULT '';

ALTER TABLE owner_setup ADD CONSTRAINT owner_setup_commission_type
    CHECK (commission_type IN ('', 'percent', 'fixed'));
ALTER TABLE owner_setup ADD CONSTRAINT owner_setup_commission_currency
    CHECK (commission_currency = '' OR commission_currency ~ '^[A-Z]{3}$');
ALTER TABLE owner_setup ADD CONSTRAINT owner_setup_listing_price_currency
    CHECK (listing_price_currency = '' OR listing_price_currency ~ '^[A-Z]{3}$');
ALTER TABLE owner_setup ADD CONSTRAINT owner_setup_fixed_currency
    CHECK (commission_type <> 'fixed' OR commission_currency <> '');
