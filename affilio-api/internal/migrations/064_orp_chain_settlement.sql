-- dev.allagio.settle-chain: a payout on a chain rail carries the payer's
-- transaction reference, and the host reads the chain to confirm it. The
-- host holds no keys and moves no money. chain_tx is <caip2>:<hash>; one
-- transaction settles one payout. orp_payout_wallets is the payee's public
-- receiving address per network. It is shown to a payer of a payable payout
-- and never appears in an ORP message.

ALTER TABLE orp_payouts ADD COLUMN IF NOT EXISTS ref TEXT;
ALTER TABLE orp_payouts ADD COLUMN IF NOT EXISTS chain_tx TEXT;
ALTER TABLE orp_payouts ADD COLUMN IF NOT EXISTS chain_status TEXT
  CHECK (chain_status IN ('pending','verified','mismatch','unavailable'));
ALTER TABLE orp_payouts ADD COLUMN IF NOT EXISTS chain_reason TEXT;
ALTER TABLE orp_payouts ADD COLUMN IF NOT EXISTS chain_checked_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS orp_payouts_chain_tx
  ON orp_payouts(chain_tx) WHERE chain_tx IS NOT NULL;
CREATE INDEX IF NOT EXISTS orp_payouts_chain_open
  ON orp_payouts(chain_checked_at) WHERE state='sent' AND chain_tx IS NOT NULL;

CREATE TABLE IF NOT EXISTS orp_payout_wallets (
  owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (owner_id, network)
);
