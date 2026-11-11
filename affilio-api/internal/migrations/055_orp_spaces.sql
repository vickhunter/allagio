CREATE TABLE IF NOT EXISTS orp_spaces (
    id TEXT NOT NULL,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    party TEXT NOT NULL,
    space JSONB NOT NULL,
    listed BOOLEAN NOT NULL DEFAULT false,
    withdrawn_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (owner_id, id)
);

CREATE INDEX IF NOT EXISTS orp_spaces_listed_idx ON orp_spaces (listed);
