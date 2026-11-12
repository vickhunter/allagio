CREATE TABLE IF NOT EXISTS orp_payouts (
  id text PRIMARY KEY,
  deal_id text NOT NULL,
  obligation text NOT NULL,
  basis_obligation text,
  payer_owner uuid NOT NULL,
  payer_party text NOT NULL,
  payee_owner uuid NOT NULL,
  payee_party text NOT NULL,
  state text NOT NULL CHECK (state IN ('held','payable','sent','paid','reversed','failed')),
  amount_minor bigint NOT NULL,
  currency text NOT NULL,
  rail text NOT NULL,
  basis text NOT NULL,
  hold_until timestamptz,
  sent_at timestamptz,
  answered_at timestamptz,
  answer text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS orp_payouts_dedup
  ON orp_payouts(deal_id, obligation, COALESCE(basis_obligation,''));
CREATE INDEX IF NOT EXISTS orp_payouts_payer ON orp_payouts(payer_owner, state);
CREATE INDEX IF NOT EXISTS orp_payouts_deal ON orp_payouts(deal_id);
