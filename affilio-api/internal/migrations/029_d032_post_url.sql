-- D-032a: post_url is where a placement go link sends the reader when the
-- offer itself is not live. Existing rows keep the surface URL they were
-- prepared with.
ALTER TABLE placement_proposals ADD COLUMN post_url TEXT NOT NULL DEFAULT '';
UPDATE placement_proposals SET post_url = surface_url WHERE post_url = '';
