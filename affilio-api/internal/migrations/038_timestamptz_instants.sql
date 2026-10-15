-- clicks.clicked_at, conversions.occurred_at, and ledger_entries.clears_at
-- stored a timestamp without time zone. This migration reads those faces as
-- UTC. That reading is true only when the server and role default timezone
-- has always been UTC. Before applying, run SHOW timezone in the production
-- session and stop unless it is UTC, Etc/UTC, GMT, Etc/GMT, Etc/Universal,
-- Universal, or Zulu. The script refuses any other session. Safe to run more
-- than once. The reverse is internal/migrations/down/038_timestamptz_instants.sql
-- and is not applied automatically. Apply that down script in a UTC session too.

DO $$
BEGIN
  IF current_setting('TimeZone') NOT IN ('UTC', 'Etc/UTC', 'GMT', 'Etc/GMT', 'Etc/Universal', 'Universal', 'Zulu') THEN
    RAISE EXCEPTION '038_timestamptz_instants requires a UTC session (current TimeZone is %). Verify the server and role default timezone is UTC with SHOW timezone, then apply this migration in that session. Stored timestamp faces are read as UTC.', current_setting('TimeZone');
  END IF;
END $$;

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
