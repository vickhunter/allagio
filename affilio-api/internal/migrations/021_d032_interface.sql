-- Who revoked a placement, so publisher and maker undos are distinct.
ALTER TABLE placement_proposals ADD COLUMN revoked_by_side TEXT;
ALTER TABLE placement_proposals ADD CONSTRAINT placement_proposals_revoked_by_side
    CHECK (revoked_by_side IS NULL OR revoked_by_side IN ('publisher', 'maker'));
