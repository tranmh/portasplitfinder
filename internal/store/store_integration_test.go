//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

func seedBasics(t *testing.T, db *DB) (productID string, storeID string) {
	t.Helper()
	ctx := context.Background()
	pid := "11111111-1111-1111-1111-111111111111"
	if err := db.Products.Upsert(ctx, domain.Product{ID: pid, Name: "Midea Portasplit", Model: "MAW-12"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "mock", Name: "Mock", Type: domain.SourceStore, BaseURL: "http://x"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline, BaseURL: "http://a"}); err != nil {
		t.Fatal(err)
	}
	sid, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "mock", Name: "OBI Berlin", PLZ: "10115", City: "Berlin", Lat: 52.53, Lon: 13.38})
	if err != nil {
		t.Fatal(err)
	}
	return pid, sid
}

// snapshotRowsTouched returns how many availability_snapshots rows one
// LatestForKey call reads (sequential reads plus index heap fetches). repo's
// pool must hold a single connection so the stats come from that backend.
func snapshotRowsTouched(t *testing.T, repo *SnapshotRepo, storeID, productID string) int64 {
	t.Helper()
	ctx := context.Background()
	read := func() int64 {
		if _, err := repo.pool.Exec(ctx, `SELECT pg_stat_force_next_flush()`); err != nil {
			t.Fatal(err)
		}
		var n int64
		if err := repo.pool.QueryRow(ctx, `
			SELECT coalesce(seq_tup_read,0) + coalesce(idx_tup_fetch,0)
			FROM pg_stat_user_tables WHERE relname = 'availability_snapshots'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := read()
	if _, ok, err := repo.LatestForKey(ctx, "mock", storeID, productID); err != nil || !ok {
		t.Fatalf("latest for key: ok=%v err=%v", ok, err)
	}
	return read() - before
}

// LatestForKey is called once per listing every poll cycle, so the rows it
// reads must not grow with the snapshot history of the whole source.
func TestIntegration_SnapshotLatestForKeyScalesWithHistory(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()
	pid, sid := seedBasics(t, db)

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO stores (source_key, name, plz, city, lat, lon)
		SELECT 'mock', 'Store ' || g, '10115', 'Berlin', 52.5, 13.4 FROM generate_series(1, 200) g`); err != nil {
		t.Fatal(err)
	}
	addPolls := func(polls int) {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO availability_snapshots (source_key, store_id, product_id, in_stock, url, checked_at)
			SELECT 'mock', s.id, $1, false, 'u', now() - make_interval(mins => p)
			FROM stores s, generate_series(1, $2::int) p
			WHERE s.source_key = 'mock'`, pid, polls); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `ANALYZE availability_snapshots`); err != nil {
			t.Fatal(err)
		}
	}

	cfg := db.Pool.Config()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := &SnapshotRepo{pool: pool}

	addPolls(50)
	n := snapshotRowsTouched(t, repo, sid, pid)
	addPolls(50) // history doubles
	n2 := snapshotRowsTouched(t, repo, sid, pid)

	if n2 > 2*n+10 || n2 > 1000 {
		t.Fatalf("LatestForKey reads %d rows at N polls and %d at 2N; want a bounded index lookup, not a scan of the source's history", n, n2)
	}
}

func TestIntegration_ProductSourceStoreCRUD(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()

	pid, sid := seedBasics(t, db)

	products, err := db.Products.List(ctx)
	if err != nil || len(products) != 1 || products[0].ID != pid {
		t.Fatalf("products list: %v %+v", err, products)
	}
	sources, err := db.Sources.List(ctx)
	if err != nil || len(sources) != 2 {
		t.Fatalf("sources list: %v %+v", err, sources)
	}
	store, ok, err := db.Stores.Get(ctx, sid)
	if err != nil || !ok || store.City != "Berlin" {
		t.Fatalf("store get: %v %v %+v", err, ok, store)
	}

	// Re-upsert store by (source_key, name) keeps the same id.
	sid2, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "mock", Name: "OBI Berlin", PLZ: "10117", City: "Berlin", Lat: 52.51, Lon: 13.39})
	if err != nil || sid2 != sid {
		t.Fatalf("store upsert idempotency: %v id1=%s id2=%s", err, sid, sid2)
	}
}

func TestIntegration_SnapshotLatestPerKey(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()
	pid, sid := seedBasics(t, db)

	ip := func(i int) *int { return &i }
	// Two snapshots for the same store/product; latest should win.
	if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid, InStock: false, PriceCents: ip(100), Currency: "EUR", URL: "u1"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid, InStock: true, PriceCents: ip(200), Currency: "EUR", URL: "u2"}); err != nil {
		t.Fatal(err)
	}
	// One online snapshot.
	if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "amazon", ProductID: pid, InStock: true, Currency: "EUR", URL: "u3"}); err != nil {
		t.Fatal(err)
	}

	latest, err := db.Snapshots.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("expected 2 latest keys, got %d: %+v", len(latest), latest)
	}
	for _, s := range latest {
		if s.SourceKey == "mock" {
			if !s.InStock || s.URL != "u2" {
				t.Fatalf("latest mock snapshot wrong: %+v", s)
			}
		}
	}

	cur, ok, err := db.Snapshots.LatestForKey(ctx, "mock", sid, pid)
	if err != nil || !ok || cur.URL != "u2" {
		t.Fatalf("latest for key: %v %v %+v", err, ok, cur)
	}
	// Online key (empty store id).
	curOnline, ok, err := db.Snapshots.LatestForKey(ctx, "amazon", "", pid)
	if err != nil || !ok || curOnline.URL != "u3" {
		t.Fatalf("latest online key: %v %v %+v", err, ok, curOnline)
	}
}

func TestIntegration_SubscriptionTokenFlow(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()
	seedBasics(t, db)

	id, err := db.Subscriptions.Create(ctx, domain.Subscription{
		Email: "a@b.de", PLZ: "10115", RadiusKm: 50, ConfirmToken: "ct", UnsubToken: "ut",
	})
	if err != nil || id == "" {
		t.Fatalf("create: %v", err)
	}

	// Not yet confirmed.
	confirmed, err := db.Subscriptions.ListConfirmed(ctx)
	if err != nil || len(confirmed) != 0 {
		t.Fatalf("expected 0 confirmed, got %v %+v", err, confirmed)
	}

	email, ok, err := db.Subscriptions.ConfirmByToken(ctx, "ct")
	if err != nil || !ok || email != "a@b.de" {
		t.Fatalf("confirm: %v %v %q", err, ok, email)
	}
	confirmed, _ = db.Subscriptions.ListConfirmed(ctx)
	if len(confirmed) != 1 {
		t.Fatalf("expected 1 confirmed, got %d", len(confirmed))
	}

	// Unknown token.
	if _, ok, _ := db.Subscriptions.ConfirmByToken(ctx, "nope"); ok {
		t.Fatal("expected unknown confirm token to fail")
	}

	ok, err = db.Subscriptions.UnsubByToken(ctx, "ut")
	if err != nil || !ok {
		t.Fatalf("unsub: %v %v", err, ok)
	}
	confirmed, _ = db.Subscriptions.ListConfirmed(ctx)
	if len(confirmed) != 0 {
		t.Fatalf("expected 0 confirmed after unsub, got %d", len(confirmed))
	}
}

func TestIntegration_NotificationDedupe(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()
	pid, sid := seedBasics(t, db)

	subID, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "a@b.de", PLZ: "10115", RadiusKm: 50, ConfirmToken: "ct", UnsubToken: "ut"})
	if err != nil {
		t.Fatal(err)
	}

	ins, err := db.Notifications.Record(ctx, subID, "mock", sid, pid, "dk1")
	if err != nil || !ins {
		t.Fatalf("first record: %v inserted=%v", err, ins)
	}
	ins, err = db.Notifications.Record(ctx, subID, "mock", sid, pid, "dk1")
	if err != nil || ins {
		t.Fatalf("dupe record should not insert: %v inserted=%v", err, ins)
	}
	exists, err := db.Notifications.Exists(ctx, subID, "dk1")
	if err != nil || !exists {
		t.Fatalf("exists: %v %v", err, exists)
	}
}
