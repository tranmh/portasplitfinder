// Package store provides pgx/v5 repositories over the PortaSplitFinder schema
// plus a goose-based migration helper using the embedded migration FS.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/tranmh/portasplitfinder/migrations"
)

// DB bundles a pgx connection pool and the repositories built over it.
type DB struct {
	Pool          *pgxpool.Pool
	Products      *ProductRepo
	Sources       *SourceRepo
	Stores        *StoreRepo
	Snapshots     *SnapshotRepo
	Subscriptions *SubscriptionRepo
	Notifications *NotificationRepo
}

// Open creates a pgx pool from the database URL and wires up the repositories.
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &DB{
		Pool:          pool,
		Products:      &ProductRepo{pool: pool},
		Sources:       &SourceRepo{pool: pool},
		Stores:        &StoreRepo{pool: pool},
		Snapshots:     &SnapshotRepo{pool: pool},
		Subscriptions: &SubscriptionRepo{pool: pool},
		Notifications: &NotificationRepo{pool: pool},
	}, nil
}

// WaitReady blocks until the database accepts a connection (open + ping) or
// maxWait elapses, retrying with bounded backoff. It makes startup resilient to
// transient failures — Docker's embedded DNS not yet resolving the `db` service,
// or Postgres still finishing boot — so a single hiccup no longer kills the
// process before Migrate/Open run. Returns the last error on timeout, or the
// context error if ctx is cancelled.
func WaitReady(ctx context.Context, databaseURL string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	var lastErr error
	for attempt := 0; ; attempt++ {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
		}
		if err == nil {
			return nil
		}
		lastErr = err

		delay := time.Duration(attempt+1) * 500 * time.Millisecond
		if delay > 3*time.Second {
			delay = 3 * time.Second
		}
		if time.Now().Add(delay).After(deadline) {
			return fmt.Errorf("store: db not ready after %s: %w", maxWait, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// Close releases the underlying pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Ping checks DB connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// migrateLockID is an arbitrary, app-specific key for the Postgres advisory lock
// that serializes concurrent migrators (e.g. api and poller starting together).
const migrateLockID = 776655443322

// Migrate runs all up migrations against the database URL using the embedded
// goose FS. It opens a short-lived database/sql connection because goose
// operates on *sql.DB. A session-level advisory lock serializes concurrent
// callers so the api and poller can both run migrations on boot safely.
func Migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("store: migrate open: %w", err)
	}
	defer db.Close()

	// Acquire a single connection and hold the advisory lock on it for the whole
	// migration so the lock is released when this connection is returned.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: migrate conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrateLockID); err != nil {
		return fmt.Errorf("store: migrate lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrateLockID)
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("store: migrate dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("store: migrate up: %w", err)
	}
	return nil
}

// ensure stdlib is imported for the pgx sql driver registration.
var _ = stdlib.GetDefaultDriver
