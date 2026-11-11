-- 0.3 deals mint a tracked link on first activate. The column holds a
-- parties-shaped document ({"tracking":[...], "clicks":[...],
-- "link_receipts":[...]}) so one click pipeline serves 0.3 and 0.4.
-- A 0.4 row leaves this column NULL; its tracking stays inside parties.
ALTER TABLE orp_deals ADD COLUMN IF NOT EXISTS tracking jsonb;
