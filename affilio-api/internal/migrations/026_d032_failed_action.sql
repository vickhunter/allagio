-- The plugin action that failed. Retry repeats only this action.
-- removed re-queues the removal. restored re-queues the re-insert.

ALTER TABLE placement_proposals
    ADD COLUMN failed_action TEXT NOT NULL DEFAULT '';

ALTER TABLE placement_proposals
    ADD CONSTRAINT placement_proposals_failed_action_check
    CHECK (failed_action IN ('', 'removed', 'restored'));
