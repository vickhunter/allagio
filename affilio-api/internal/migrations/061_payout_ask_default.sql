-- A site with no saved step-mode row stores placement_publish ask
-- and payout ask. Money steps wait on the owner; the rest stay auto.
-- Apply by hand. AUTO_MIGRATE stays off unless it is set to true.
-- Safe to run more than once. A stored row is left as stored, including auto.
-- This inserts one site-scoped row. It does not change an existing row.
-- A site that already has a site row, or an owner-wide row for its owner, is skipped.

INSERT INTO agent_step_modes (
    scope_key,
    owner_id,
    site_id,
    site_signup,
    offer_match,
    placement_publish,
    link_change,
    payout
)
SELECT
    'site:' || s.id::text,
    NULL,
    s.id,
    'auto',
    'auto',
    'ask',
    'auto',
    'ask'
FROM sites s
WHERE NOT EXISTS (
    SELECT 1
    FROM agent_step_modes m
    WHERE m.site_id = s.id
       OR (
            m.site_id IS NULL
            AND m.owner_id IN (SELECT o.id FROM owners o WHERE o.site_id = s.id)
       )
)
ON CONFLICT (scope_key) DO NOTHING;
