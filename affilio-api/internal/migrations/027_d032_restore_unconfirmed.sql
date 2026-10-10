ALTER TABLE placement_proposals
    ADD COLUMN restore_unconfirmed BOOLEAN NOT NULL DEFAULT false;
