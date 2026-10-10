-- D-032a must-fixes: a committed placement stays in the live count after the
-- 7-day prepare window. Plugin acknowledgements use a plugin key, not the
-- site key the local MCP uses.

CREATE OR REPLACE VIEW placement_counts_as_live AS
SELECT pp.id
FROM placement_proposals pp
WHERE pp.plugin_wait = 'placed'
  AND pp.status = 'committed'
  AND NOT pp.restore_unconfirmed
  AND EXISTS (
      SELECT 1
      FROM campaigns c
      JOIN offers o ON o.id = c.offer_id
      WHERE c.id = pp.campaign_id
        AND c.site_id = pp.site_id
        AND c.offer_id = pp.offer_id
        AND c.status = 'active'
        AND o.status = 'listed'
  )
  AND NOT EXISTS (
      SELECT 1 FROM approval_policies pol
      WHERE pol.site_id = pp.site_id AND pol.kill_switch
  )
  AND NOT EXISTS (
      SELECT 1 FROM approval_policies pol
      WHERE pol.offer_id = pp.offer_id AND pol.kill_switch
  );

CREATE TABLE IF NOT EXISTS plugin_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS plugin_keys_site_idx ON plugin_keys (site_id) WHERE revoked_at IS NULL;
