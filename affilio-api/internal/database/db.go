package database

import (
	"context"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*DB, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{}
		// Railway private networking uses IPv6; try tcp6 first, then fall back.
		conn, err := d.DialContext(ctx, "tcp6", addr)
		if err != nil {
			conn, err = d.DialContext(ctx, "tcp", addr)
		}
		return conn, err
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
