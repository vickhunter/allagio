-- ORP outcome sweep markers. One row per emitted outcome: the weekly period
-- end is the dedup key, and one partial unique index keeps final:true to a
-- single send. Party data stays in orp_inbox; this table holds no click or
-- personal data, only the period bookkeeping the sweep needs to stay
-- idempotent.

CREATE TABLE IF NOT EXISTS orp_outcomes (
    deal_id    TEXT NOT NULL,
    period_from TIMESTAMPTZ NOT NULL,
    period_to  TIMESTAMPTZ NOT NULL,
    final      BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (deal_id, period_to)
);

CREATE UNIQUE INDEX IF NOT EXISTS orp_outcomes_one_final
    ON orp_outcomes (deal_id)
    WHERE final;
