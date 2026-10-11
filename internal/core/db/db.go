// Package db provides the application-wide pgx connection pool.
// All repositories must obtain their connection from this pool — never
// create a new pgxpool.Pool outside this package.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the shared pgx connection pool type alias for ergonomic imports.
type Pool = pgxpool.Pool

// Connect creates and validates a pgx connection pool using the given DSN.
// The DSN must point to the PgBouncer transaction pooler (port 6543) and
// include ?pgbouncer=true&connection_limit=1.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Connect: parse config: %w", err)
	}

	// Tune pool for PgBouncer transaction mode.
	// MinConns=0: don't pre-create connections that PgBouncer will silently kill after idle timeout.
	// MaxConnIdleTime: evict idle connections before PgBouncer's ~5 min idle timeout to prevent
	// stale connections causing INTERNAL_ERROR on the first request after inactivity.
	// MaxConns=25: full-details makes up to 3 sequential queries per request; 10 was too low under concurrent load.
	// HealthCheckPeriod=30s: detect stale connections faster — a 60s window was too large.
	// MaxConnIdleTime=3m: extra buffer below PgBouncer's 5 min timeout, accounting for check scheduling jitter.
	// PgBouncer transaction pooling doesn't persist prepared statements across
	// connections. SimpleProtocol skips the Prepare/Execute round-trip entirely,
	// sending queries as plain text — the only safe mode under transaction pooling.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	cfg.MaxConns = 25
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 3 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	// Ping before every acquire — if PgBouncer killed the connection while it was
	// idle in the pool, discard it immediately rather than failing the first query.
	cfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		return conn.Ping(ctx) == nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db.Connect: open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db.Connect: ping: %w", err)
	}

	return pool, nil
}
