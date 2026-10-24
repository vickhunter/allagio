-- D-092c: test-mode ORP offers, deals, idempotency, and inboxes.
-- Apply by hand. AUTO_MIGRATE stays off unless it is set to true.
-- Safe to run more than once. These tables do not hold money or personal data.
-- Party ids, terms, and public proof text only. No email, name, address, or payment row.

CREATE TABLE IF NOT EXISTS orp_offers (
    id TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    party TEXT NOT NULL,
    terms JSONB NOT NULL,
    terms_hash TEXT NOT NULL,
    deal_limit INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS orp_deals (
    id TEXT PRIMARY KEY,
    offer_id TEXT,
    offerer_owner UUID REFERENCES owners(id) ON DELETE CASCADE,
    acceptor_owner UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    offerer_party TEXT NOT NULL,
    acceptor_party TEXT NOT NULL,
    terms JSONB NOT NULL,
    terms_hash TEXT NOT NULL,
    state TEXT NOT NULL,
    waiting JSONB,
    obligations JSONB NOT NULL,
    bound JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS orp_messages (
    party TEXT NOT NULL,
    message_id TEXT NOT NULL,
    body_hash TEXT NOT NULL,
    status INTEGER NOT NULL,
    reply JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (party, message_id)
);

CREATE TABLE IF NOT EXISTS orp_inbox (
    id BIGSERIAL PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    message JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS orp_deals_acceptor_idx ON orp_deals (acceptor_owner);
CREATE INDEX IF NOT EXISTS orp_deals_offerer_idx ON orp_deals (offerer_owner);
CREATE INDEX IF NOT EXISTS orp_inbox_owner_idx ON orp_inbox (owner_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS orp_deals_open_offer_acceptor
    ON orp_deals (offer_id, acceptor_owner)
    WHERE offer_id IS NOT NULL AND state IN ('proposed', 'awaiting', 'active');
