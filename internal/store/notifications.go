package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationRepo records sent notifications and supports dedupe checks.
type NotificationRepo struct {
	pool *pgxpool.Pool
}

// Record inserts a notification record. It returns inserted=false (without
// error) if a record with the same (subscription_id, dedupe_key) already exists,
// so callers can use this as an atomic "claim" for dedupe.
func (r *NotificationRepo) Record(ctx context.Context, subscriptionID, sourceKey, storeID, productID, dedupeKey string) (inserted bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key)
		VALUES ($1, $2, NULLIF($3,'')::uuid, $4, $5)
		ON CONFLICT (subscription_id, dedupe_key) DO NOTHING
	`, subscriptionID, sourceKey, storeID, productID, dedupeKey)
	if err != nil {
		return false, fmt.Errorf("notifications record: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Exists reports whether a notification with the dedupe key was already sent for
// the subscription.
func (r *NotificationRepo) Exists(ctx context.Context, subscriptionID, dedupeKey string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM notifications WHERE subscription_id = $1 AND dedupe_key = $2)
	`, subscriptionID, dedupeKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("notifications exists: %w", err)
	}
	return exists, nil
}
