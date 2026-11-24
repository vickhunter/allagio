-- Products are listed in find_partners by default. The owner turns a product
-- off from the owner page. Existing rows stay listed.
--
-- This migration does not touch party_keys. A party can hold more than one
-- active key (the primary key is owner_id plus kid, and GET /orp/keys returns
-- the set). First-bind races are serialized by a row lock in BindFirstPartyKey,
-- not by a unique index and not by removing rows.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS listed BOOLEAN NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS products_listed_exact_name
    ON products (lower(btrim(name)))
    WHERE listed;
