-- Products are listed in find_partners by default. The owner turns a product
-- off from the owner page. Existing rows stay listed.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS listed BOOLEAN NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS products_listed_exact_name
    ON products (lower(btrim(name)))
    WHERE listed;

-- A party has one active signing key. The join path binds the first key and
-- refuses a different one. Drop a later active row left by the old insert
-- race so the unique index can be created.
DELETE FROM party_keys AS extra
USING party_keys AS keep
WHERE extra.owner_id = keep.owner_id
  AND extra.revoked_at IS NULL
  AND keep.revoked_at IS NULL
  AND (extra.created_at, extra.kid) > (keep.created_at, keep.kid);

CREATE UNIQUE INDEX IF NOT EXISTS party_keys_one_active
    ON party_keys (owner_id)
    WHERE revoked_at IS NULL;
