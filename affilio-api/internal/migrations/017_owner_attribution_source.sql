ALTER TABLE owners ADD COLUMN source TEXT NOT NULL DEFAULT 'direct';

ALTER TABLE owners ADD CONSTRAINT owners_source_allowed
    CHECK (source IN ('chatgpt', 'claude', 'cursor', 'grok', 'x', 'community', 'direct'));
