-- Reverse of 038_timestamptz_instants.sql. Not applied by migrate.Apply.
-- Run this by hand only, and only after SHOW timezone is UTC, Etc/UTC, GMT,
-- Etc/GMT, or Etc/Universal. The guard and the ALTERs are one transaction,
-- so a failed guard does not autocommit the later statements under psql -f.
-- Apply with:
--   psql -v ON_ERROR_STOP=1 -1 -f internal/migrations/down/038_timestamptz_instants.sql
-- Safe to run more than once.

BEGIN;

DO $$
BEGIN
  IF current_setting('TimeZone') NOT IN ('UTC', 'Etc/UTC', 'GMT', 'Etc/GMT', 'Etc/Universal') THEN
    RAISE EXCEPTION 'down/038_timestamptz_instants requires a UTC session (current TimeZone is %). Verify the server and role default timezone is UTC with SHOW timezone, then apply this down script in that session. Stored timestamp faces are written as UTC.', current_setting('TimeZone');
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'clicks' AND column_name = 'clicked_at'
      AND data_type = 'timestamp with time zone'
  ) THEN
    ALTER TABLE clicks
      ALTER COLUMN clicked_at TYPE timestamp WITHOUT TIME ZONE
      USING clicked_at AT TIME ZONE 'UTC';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'conversions' AND column_name = 'occurred_at'
      AND data_type = 'timestamp with time zone'
  ) THEN
    ALTER TABLE conversions
      ALTER COLUMN occurred_at TYPE timestamp WITHOUT TIME ZONE
      USING occurred_at AT TIME ZONE 'UTC';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'ledger_entries' AND column_name = 'clears_at'
      AND data_type = 'timestamp with time zone'
  ) THEN
    ALTER TABLE ledger_entries
      ALTER COLUMN clears_at TYPE timestamp WITHOUT TIME ZONE
      USING clears_at AT TIME ZONE 'UTC';
  END IF;
END $$;

COMMIT;
