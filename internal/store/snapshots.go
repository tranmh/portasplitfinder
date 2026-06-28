package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// SnapshotRepo writes availability snapshots and reads the latest per key.
type SnapshotRepo struct {
	pool *pgxpool.Pool
}

// Insert writes a single availability snapshot. If checkedAt is zero, now() is
// used by the DB default.
func (r *SnapshotRepo) Insert(ctx context.Context, s domain.AvailabilitySnapshot) error {
	checkedAt := s.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO availability_snapshots
			(source_key, store_id, product_id, in_stock, quantity, price_cents, currency, url, checked_at)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9)
	`, s.SourceKey, deref(s.StoreID), s.ProductID, s.InStock, s.Quantity, s.PriceCents, s.Currency, s.URL, checkedAt)
	if err != nil {
		return fmt.Errorf("snapshots insert: %w", err)
	}
	return nil
}

// Latest returns the latest snapshot per (source_key, store_id, product_id).
// storeID NULL is treated as its own key (online listings).
func (r *SnapshotRepo) Latest(ctx context.Context) ([]domain.AvailabilitySnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (source_key, store_id, product_id)
			id, source_key, store_id, product_id, in_stock, quantity, price_cents, currency, url, checked_at
		FROM availability_snapshots
		ORDER BY source_key, store_id, product_id, checked_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("snapshots latest: %w", err)
	}
	defer rows.Close()

	var out []domain.AvailabilitySnapshot
	for rows.Next() {
		var s domain.AvailabilitySnapshot
		if err := rows.Scan(&s.ID, &s.SourceKey, &s.StoreID, &s.ProductID, &s.InStock, &s.Quantity, &s.PriceCents, &s.Currency, &s.URL, &s.CheckedAt); err != nil {
			return nil, fmt.Errorf("snapshots scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LatestForKey returns the most recent snapshot for one (source, store, product)
// key, or ok=false if none exists. storeID may be empty for online listings.
func (r *SnapshotRepo) LatestForKey(ctx context.Context, sourceKey, storeID, productID string) (domain.AvailabilitySnapshot, bool, error) {
	var s domain.AvailabilitySnapshot
	err := r.pool.QueryRow(ctx, `
		SELECT id, source_key, store_id, product_id, in_stock, quantity, price_cents, currency, url, checked_at
		FROM availability_snapshots
		WHERE source_key = $1 AND store_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid AND product_id = $3
		ORDER BY checked_at DESC
		LIMIT 1
	`, sourceKey, storeID, productID).Scan(
		&s.ID, &s.SourceKey, &s.StoreID, &s.ProductID, &s.InStock, &s.Quantity, &s.PriceCents, &s.Currency, &s.URL, &s.CheckedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.AvailabilitySnapshot{}, false, nil
		}
		return domain.AvailabilitySnapshot{}, false, fmt.Errorf("snapshots latest for key: %w", err)
	}
	return s, true, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
