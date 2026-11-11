-- Share and revoke step modes for ORP per-step autonomy. Apply by hand.
-- share defaults to ask and revoke to auto, matching the spec's safe side.
-- Safe to run more than once. AUTO_MIGRATE stays off.

BEGIN;

ALTER TABLE agent_step_modes
    ADD COLUMN IF NOT EXISTS share TEXT NOT NULL DEFAULT 'ask',
    ADD COLUMN IF NOT EXISTS revoke TEXT NOT NULL DEFAULT 'auto';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'agent_step_modes_share') THEN
        ALTER TABLE agent_step_modes ADD CONSTRAINT agent_step_modes_share CHECK (share IN ('auto', 'ask', 'off'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'agent_step_modes_revoke') THEN
        ALTER TABLE agent_step_modes ADD CONSTRAINT agent_step_modes_revoke CHECK (revoke IN ('auto', 'ask', 'off'));
    END IF;
END $$;

COMMIT;
