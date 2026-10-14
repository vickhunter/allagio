-- Allow the plugin to record that the block is in the post.

ALTER TABLE placement_proposals DROP CONSTRAINT placement_proposals_plugin_wait_check;
ALTER TABLE placement_proposals ADD CONSTRAINT placement_proposals_plugin_wait_check
    CHECK (plugin_wait IN ('', 'remove', 'restore', 'removed', 'placed'));
