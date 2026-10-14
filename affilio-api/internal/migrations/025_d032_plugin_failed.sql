-- The plugin's post re-read failure. Empty means the placement is not failed.
-- plugin_wait stays the pending action so a later verified ack still knows
-- whether to record placed or removed.

ALTER TABLE placement_proposals
    ADD COLUMN plugin_failed_reason TEXT NOT NULL DEFAULT '';
