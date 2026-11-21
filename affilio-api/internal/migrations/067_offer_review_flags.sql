-- Persisted advisory flags from the automated listing checks. Submit lists
-- every offer; when a check flagged it, the reason/message live here so later
-- reads report the same outcome the submit response did.
ALTER TABLE offers
    ADD COLUMN review_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN review_message TEXT NOT NULL DEFAULT '';
