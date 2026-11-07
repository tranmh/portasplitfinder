// Package pgtest gives integration tests an isolated, migrated Postgres schema
// on the run's database (AQAA_PG_URL) plus a deterministic SQL seed. It does
// not import internal/store, so store's own tests can use it.
package pgtest

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// SeedSQL inserts the fixturetest catalogue: products, obi/amazon sources,
// the near and far stores, snapshots and one confirmed subscription.
//
//go:embed testdata/seed.sql
var SeedSQL string

// URL returns AQAA_PG_URL or skips the test when it is unset.
func URL(t testing.TB) string {
	t.Helper()
	u := os.Getenv("AQAA_PG_URL")
	if u == "" {
		t.Skip("AQAA_PG_URL not set")
	}
	return u
}

// Schema creates a fresh schema, drops it at cleanup and returns a DSN whose
// search_path points at it. Run store.Migrate(ctx, dsn) on it before use.
func Schema(t testing.TB) string {
	t.Helper()
	base := URL(t)
	ctx := context.Background()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("pgtest: rand: %v", err)
	}
	name := "t_" + hex.EncodeToString(b)

	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+name); err != nil {
		t.Fatalf("pgtest: create schema: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), base)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		_, _ = c.Exec(context.Background(), "DROP SCHEMA "+name+" CASCADE")
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("pgtest: parse url: %v", err)
	}
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	return u.String()
}

// Exec runs sql (may hold several statements) against dsn.
func Exec(t testing.TB, dsn, sql string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("pgtest: exec: %v", err)
	}
}

// Seed loads SeedSQL into the (migrated) schema of dsn.
func Seed(t testing.TB, dsn string) { t.Helper(); Exec(t, dsn, SeedSQL) }

// Count returns SELECT count(*) FROM table.
func Count(t testing.TB, dsn, table string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	defer conn.Close(ctx)
	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil {
		t.Fatalf("pgtest: count %s: %v", table, err)
	}
	return n
}
