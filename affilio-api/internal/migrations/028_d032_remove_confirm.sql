-- D-032a: a failed restore can be left removed. plugin_state is removed
-- immediately, and remove_confirm asks the plugin to re-read and confirm.
ALTER TABLE placement_proposals
    ADD COLUMN remove_confirm BOOLEAN NOT NULL DEFAULT false;
