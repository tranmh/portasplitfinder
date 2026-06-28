package store

import (
	"context"
	"fmt"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// Seed inserts/updates the given catalogue (products, sources, stores) into the
// database. It is idempotent (upserts by id/key). Used by `cmd/api --seed`.
func (db *DB) Seed(ctx context.Context, products []domain.Product, sources []domain.Source, stores []domain.Store) error {
	for _, p := range products {
		if err := db.Products.Upsert(ctx, p); err != nil {
			return fmt.Errorf("seed product %s: %w", p.ID, err)
		}
	}
	for _, s := range sources {
		if err := db.Sources.Upsert(ctx, s); err != nil {
			return fmt.Errorf("seed source %s: %w", s.Key, err)
		}
	}
	for _, st := range stores {
		if _, err := db.Stores.Upsert(ctx, st); err != nil {
			return fmt.Errorf("seed store %s: %w", st.Name, err)
		}
	}
	return nil
}
