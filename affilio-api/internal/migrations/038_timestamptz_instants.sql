-- clicks.clicked_at, conversions.occurred_at, and ledger_entries.clears_at
-- stored a timestamp without time zone. Treat that clock face as UTC and
-- keep the instant in timestamptz. Safe to run more than once. The reverse
-- is internal/migrations/down/038_timestamptz_instants.sql and is not applied
-- automatically.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'clicks' AND column_name = 'clicked_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE clicks
      ALTER COLUMN clicked_at TYPE timestamptz
      USING clicked_at AT TIME ZONE 'UTC';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'conversions' AND column_name = 'occurred_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE conversions
      ALTER COLUMN occurred_at TYPE timestamptz
      USING occurred_at AT TIME ZONE 'UTC';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'ledger_entries' AND column_name = 'clears_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE ledger_entries
      ALTER COLUMN clears_at TYPE timestamptz
      USING clears_at AT TIME ZONE 'UTC';
  END IF;
END $$;
