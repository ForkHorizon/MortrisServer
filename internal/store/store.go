// Package store wires PostgreSQL connection pools and applies migrations.
// Section 8.1 calls for separate bounded pools per role/purpose — this
// package hands back a plain *pgxpool.Pool per caller-supplied DSN rather
// than owning role-specific types, since the only difference between them
// is the DSN and MaxConns the caller already controls.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a bounded connection pool and verifies connectivity before
// returning, so a bad DSN fails at startup instead of on the first request.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
