-- Pause history, restore-time relevance, and plugin acknowledgement of a placement.

CREATE TABLE pause_activations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    offer_id UUID REFERENCES offers(id) ON DELETE CASCADE,
    owner_id UUID REFERENCES owners(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK (site_id IS NOT NULL OR offer_id IS NOT NULL OR owner_id IS NOT NULL)
);

CREATE INDEX pause_activations_site_idx ON pause_activations (site_id, created_at);
CREATE INDEX pause_activations_offer_idx ON pause_activations (offer_id, created_at);
CREATE INDEX pause_activations_owner_idx ON pause_activations (owner_id, created_at);

ALTER TABLE placement_proposals ADD COLUMN relevance INTEGER;
ALTER TABLE placement_proposals ADD COLUMN plugin_wait TEXT NOT NULL DEFAULT '';
ALTER TABLE placement_proposals ADD CONSTRAINT placement_proposals_plugin_wait_check
    CHECK (plugin_wait IN ('', 'remove', 'restore', 'removed'));
ALTER TABLE placement_proposals ADD COLUMN plugin_wait_at TIMESTAMPTZ;
ALTER TABLE placement_proposals ADD COLUMN plugin_contact_at TIMESTAMPTZ;
