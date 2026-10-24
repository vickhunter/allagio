-- sites.api_key_rotated_at and sites.api_key_previous_expires_at were
-- timestamp without time zone. A 24-hour grace that crosses a daylight-saving
-- change is then 23 or 25 hours in a non-UTC session. This reads the stored
-- faces as UTC, which is true only when the server timezone has always been UTC.
-- Apply in a UTC session. Safe to run more than once.
-- AUTO_MIGRATE stays off unless it is set to true.

BEGIN;

DO $$
BEGIN
  IF current_setting('TimeZone') NOT IN ('UTC', 'Etc/UTC', 'GMT', 'Etc/GMT', 'Etc/Universal') THEN
    RAISE EXCEPTION '049_site_key_grace_timestamptz requires a UTC session (current TimeZone is %).', current_setting('TimeZone');
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'sites' AND column_name = 'api_key_rotated_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE sites
      ALTER COLUMN api_key_rotated_at TYPE timestamptz
      USING api_key_rotated_at AT TIME ZONE 'UTC';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'sites' AND column_name = 'api_key_previous_expires_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE sites
      ALTER COLUMN api_key_previous_expires_at TYPE timestamptz
      USING api_key_previous_expires_at AT TIME ZONE 'UTC';
  END IF;
END $$;

COMMIT;
