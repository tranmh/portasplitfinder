//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// requireIntegration skips the test unless RUN_INTEGRATION=1.
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run integration tests")
	}
}

// startPostgres spins up a throwaway Postgres container, runs migrations, and
// returns an open *DB plus a cleanup func.
func startPostgres(t *testing.T) (*DB, func()) {
	t.Helper()
	requireIntegration(t)

	ctx := context.Background()
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("portasplit"),
		postgres.WithUsername("portasplit"),
		postgres.WithPassword("portasplit"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = container.Terminate(ctx)
	}
	return db, cleanup
}
