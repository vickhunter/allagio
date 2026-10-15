package database

// ledger_entries.created_at and payouts.created_at / paid_at are still
// timestamp without time zone. NOW() writes the session wall clock into
// them. Read those columns back with the inverse so a scanned time.Time is
// the instant that wall clock named. clicks.clicked_at,
// conversions.occurred_at, and ledger_entries.clears_at are timestamptz and
// are stored and read as instants, so they do not use this cast.
const sessionWallAsInstant = `AT TIME ZONE current_setting('TimeZone')`
