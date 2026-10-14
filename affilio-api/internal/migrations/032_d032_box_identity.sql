-- One box per post can key on the WordPress post id. Undo stores the
-- plugin wait it replaced so a redo before the plugin syncs can put that
-- wait back when the box is still on the page.

ALTER TABLE placement_proposals ADD COLUMN IF NOT EXISTS wp_post_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE placement_proposals ADD COLUMN IF NOT EXISTS plugin_wait_before TEXT NOT NULL DEFAULT '';
