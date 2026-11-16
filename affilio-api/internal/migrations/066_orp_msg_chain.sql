-- ORP E8 message hash chain (distinct from payout settlement chain). One row per (deal, party): last_hash is the JCS hash of
-- that sender's last stored message in the deal, used_prev latches once the
-- sender has chained. No personal data — only message hashes.

CREATE TABLE IF NOT EXISTS orp_msg_chain (
    deal_id    TEXT NOT NULL,
    party      TEXT NOT NULL,
    last_hash  TEXT NOT NULL,
    used_prev  BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (deal_id, party)
);
