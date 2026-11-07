//go:build integration

package pgtest_test

import (
	"context"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/pgtest"
	"github.com/tranmh/portasplitfinder/internal/store"
)

func TestIntegrationPgtestSchemaSeed(t *testing.T) {
	ctx := context.Background()
	dsn := pgtest.Schema(t)
	if err := store.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pgtest.Seed(t, dsn)

	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	products, err := db.ListProducts(ctx)
	if err != nil || len(products) != 2 {
		t.Fatalf("products = %d, err %v; want 2", len(products), err)
	}
	if n := pgtest.Count(t, dsn, "availability_snapshots"); n != 4 {
		t.Fatalf("snapshots = %d, want 4", n)
	}
}
