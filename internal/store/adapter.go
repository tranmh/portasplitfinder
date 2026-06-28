package store

import (
	"context"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// The methods below adapt *DB to the narrow interfaces expected by internal/httpapi
// (DataStore) and internal/jobs (Store), without those packages importing pgx.

// --- httpapi.DataStore ---

// ListProducts implements httpapi.DataStore.
func (db *DB) ListProducts(ctx context.Context) ([]domain.Product, error) {
	return db.Products.List(ctx)
}

// ListSources implements httpapi.DataStore.
func (db *DB) ListSources(ctx context.Context) ([]domain.Source, error) {
	return db.Sources.List(ctx)
}

// ListStores implements httpapi.DataStore / read helper.
func (db *DB) ListStores(ctx context.Context) ([]domain.Store, error) {
	return db.Stores.List(ctx)
}

// LatestSnapshots implements httpapi.DataStore.
func (db *DB) LatestSnapshots(ctx context.Context) ([]domain.AvailabilitySnapshot, error) {
	return db.Snapshots.Latest(ctx)
}

// CreateSubscription implements httpapi.DataStore.
func (db *DB) CreateSubscription(ctx context.Context, s domain.Subscription) (string, error) {
	return db.Subscriptions.Create(ctx, s)
}

// ConfirmSubscription implements httpapi.DataStore.
func (db *DB) ConfirmSubscription(ctx context.Context, token string) (string, bool, error) {
	return db.Subscriptions.ConfirmByToken(ctx, token)
}

// UnsubSubscription implements httpapi.DataStore.
func (db *DB) UnsubSubscription(ctx context.Context, token string) (bool, error) {
	return db.Subscriptions.UnsubByToken(ctx, token)
}

// --- jobs.Store ---

// UpsertStore implements jobs.Store.
func (db *DB) UpsertStore(ctx context.Context, s domain.Store) (string, error) {
	return db.Stores.Upsert(ctx, s)
}

// InsertSnapshot implements jobs.Store.
func (db *DB) InsertSnapshot(ctx context.Context, snap domain.AvailabilitySnapshot) error {
	return db.Snapshots.Insert(ctx, snap)
}

// LatestForKey implements jobs.Store.
func (db *DB) LatestForKey(ctx context.Context, sourceKey, storeID, productID string) (domain.AvailabilitySnapshot, bool, error) {
	return db.Snapshots.LatestForKey(ctx, sourceKey, storeID, productID)
}

// UpdateSourceStatus implements jobs.Store.
func (db *DB) UpdateSourceStatus(ctx context.Context, key string, checkedAt time.Time, status domain.LastStatus, lastErr *string) error {
	return db.Sources.UpdateStatus(ctx, key, checkedAt, status, lastErr)
}

// ListConfirmedSubscriptions implements jobs.Store.
func (db *DB) ListConfirmedSubscriptions(ctx context.Context) ([]domain.Subscription, error) {
	return db.Subscriptions.ListConfirmed(ctx)
}

// GetStore implements jobs.Store.
func (db *DB) GetStore(ctx context.Context, id string) (domain.Store, bool, error) {
	return db.Stores.Get(ctx, id)
}

// GetProductName implements jobs.Store.
func (db *DB) GetProductName(ctx context.Context, id string) (string, bool, error) {
	products, err := db.Products.List(ctx)
	if err != nil {
		return "", false, err
	}
	for _, p := range products {
		if p.ID == id {
			return p.Name, true, nil
		}
	}
	return "", false, nil
}

// RecordNotification implements jobs.Store.
func (db *DB) RecordNotification(ctx context.Context, subscriptionID, sourceKey, storeID, productID, dedupeKey string) (bool, error) {
	return db.Notifications.Record(ctx, subscriptionID, sourceKey, storeID, productID, dedupeKey)
}
