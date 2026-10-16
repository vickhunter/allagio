package database

import "context"

func (d *DB) InsertLandingEvent(ctx context.Context, name, source string) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO landing_events(name, source) VALUES($1,$2)`, name, source)
	return err
}
