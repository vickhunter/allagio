-- A2A task transport (GAPS: agent card exists, transport doesn't). Each
-- tasks/send persists one task doc so tasks/get can read it back. owner_id
-- is null only for anonymous suggest tasks.
CREATE TABLE IF NOT EXISTS a2a_tasks (
    task_id TEXT PRIMARY KEY,
    owner_id UUID REFERENCES owners(id) ON DELETE CASCADE,
    session_id TEXT,
    doc JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS a2a_tasks_owner ON a2a_tasks(owner_id, updated_at DESC);
