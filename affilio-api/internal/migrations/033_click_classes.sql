-- Click class is how a go-link hit is counted. Only browser is pay-per-click.
-- verified_agent and declared_agent stay on the row so a later sale can join.
-- unknown covers scanners, prefetch, and HEAD. Existing rows were not classed.

ALTER TABLE clicks ADD COLUMN IF NOT EXISTS click_class TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE clicks DROP CONSTRAINT IF EXISTS clicks_click_class_check;
ALTER TABLE clicks ADD CONSTRAINT clicks_click_class_check
    CHECK (click_class IN ('verified_agent', 'declared_agent', 'browser', 'unknown'));
