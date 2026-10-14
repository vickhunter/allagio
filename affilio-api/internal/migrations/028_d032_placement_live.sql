-- D-032a: one live-count rule for Live now, site totals, digest went-live,
-- and account totals. A placement is live only when a verified plugin
-- acknowledgement stored placed and the go link is redirecting.
CREATE VIEW placement_counts_as_live AS
SELECT pp.id
FROM placement_proposals pp
WHERE pp.plugin_wait = 'placed'
  AND pp.status = 'committed'
  AND pp.expires_at > NOW()
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
