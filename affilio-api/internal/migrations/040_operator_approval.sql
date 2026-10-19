-- Offer-review approval is an operator, not the site owner. The owner
-- session still records approved_by_kind owner.
ALTER TABLE placement_proposals DROP CONSTRAINT placement_proposals_approved_by_kind;
ALTER TABLE placement_proposals ADD CONSTRAINT placement_proposals_approved_by_kind
    CHECK (approved_by_kind IS NULL OR approved_by_kind IN ('owner', 'operator', 'policy:auto', 'policy:yolo'));
