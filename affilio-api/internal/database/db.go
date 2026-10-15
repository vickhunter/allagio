package database

import (
	"context"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
	// Now, when set, replaces the process wall clock for instants this
	// process writes. Production leaves it nil.
	Now func() time.Time
}

// Clock returns the injected instant, or the wall clock when none is set.
func (d *DB) Clock() time.Time {
	if d != nil && d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func Connect(ctx context.Context, url string) (*DB, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Railway private networking uses IPv6. Bound that attempt so a
		// blackholed AAAA record cannot consume the caller's whole deadline
		// before the IPv4 dial starts.
		return dialWithFallback(ctx, (&net.Dialer{}).DialContext, addr, 2*time.Second)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() {
	d.Pool.Close()
}

// dialWithFallback tries tcp6, then tcp. firstWait caps the IPv6 attempt.
// The parent context still bounds the IPv4 dial.
func dialWithFallback(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), addr string, firstWait time.Duration) (net.Conn, error) {
	first, cancel := context.WithTimeout(ctx, firstWait)
	conn, err := dial(first, "tcp6", addr)
	cancel()
	if err == nil {
		return conn, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return dial(ctx, "tcp", addr)
}
