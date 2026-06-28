//go:build integration

package store

import (
	"context"
	"testing"

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
