package database

// Timestamp columns such as clicks.clicked_at, conversions.occurred_at, and
// ledger_entries.clears_at store a timestamp without time zone. NOW() writes
// the session wall clock into them. A Go time.Time is an absolute instant:
// pgx would otherwise store and scan the UTC wall clock, so a session in
// Europe/Rome shifts the instant by the offset. Bind the instant as
// timestamptz and project it into the session wall clock. Read the column
// back with the inverse so the scanned time.Time is that instant.
//
// The API does not pin the session TimeZone to UTC. The same clock is what
// the digest uses for offers.updated_at.
const (
	sessionWallFromInstant = `::timestamptz AT TIME ZONE current_setting('TimeZone')`
	sessionWallAsInstant   = `AT TIME ZONE current_setting('TimeZone')`
)
