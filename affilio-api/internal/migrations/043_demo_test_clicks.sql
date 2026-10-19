-- Demo sites and test go-link clicks. Apply by hand before a binary that
-- writes click_class test. AUTO_MIGRATE stays off. Safe to run more than once.
-- Existing sites stay non-demo. Existing clicks stay in their current class.

ALTER TABLE sites ADD COLUMN IF NOT EXISTS demo BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE clicks DROP CONSTRAINT IF EXISTS clicks_click_class_check;
ALTER TABLE clicks ADD CONSTRAINT clicks_click_class_check
    CHECK (click_class IN ('verified_agent', 'declared_agent', 'browser', 'unknown', 'test'));
