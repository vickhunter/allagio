-- Restore is a 6-second window on the server undo timestamp, for the same identity.
DO $$
DECLARE
    cons text;
BEGIN
    SELECT conname INTO cons
    FROM pg_constraint
    WHERE conrelid = 'owner_events'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%digest_opened%';
    IF cons IS NOT NULL THEN
        EXECUTE format('ALTER TABLE owner_events DROP CONSTRAINT %I', cons);
    END IF;
END $$;

ALTER TABLE owner_events ADD CONSTRAINT owner_events_kind_check
    CHECK (kind IN ('digest_opened', 'dashboard_opened', 'placement_undo', 'offer_undo', 'placement_restore'));
ALTER TABLE owner_events ADD COLUMN actor TEXT;

ALTER TABLE placement_proposals ADD COLUMN status_before_undo TEXT;
ALTER TABLE placement_proposals ADD COLUMN undone_at TIMESTAMPTZ;
ALTER TABLE placement_proposals ADD COLUMN undone_by_user UUID;
ALTER TABLE placement_proposals ADD COLUMN undone_by_key_hash VARCHAR(64);
ALTER TABLE placement_proposals ADD COLUMN restored_by TEXT;
