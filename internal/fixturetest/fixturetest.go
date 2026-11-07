// Package fixturetest holds shared test doubles and builders for the domain
// types. It imports only domain, so every package's tests can use it without
// import cycles. FakeStore satisfies httpapi.DataStore structurally.
package fixturetest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// Fixed IDs and a fixed instant so fixtures are deterministic.
const (
	ProductID     = "11111111-1111-1111-1111-111111111111"
	ProductID2    = "11111111-1111-1111-1111-111111111112"
	StoreNearID   = "22222222-2222-2222-2222-222222222221" // Reutlingen-ish, close to 73035
	StoreFarID    = "22222222-2222-2222-2222-222222222222" // Berlin, far from 73035
	SourceStore   = "obi"
	SourceOnline  = "amazon"
	PLZGoeppingen = "73035"
)

// Now is the reference instant used by builders and FixedClock.
var Now = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// FixedClock is a manually advanced clock.
type FixedClock struct {
	mu sync.Mutex
	t  time.Time
}

// NewClock returns a clock set to t (Now if zero).
func NewClock(t time.Time) *FixedClock {
	if t.IsZero() {
		t = Now
	}
	return &FixedClock{t: t}
}

// Now returns the clock's current instant.
func (c *FixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Advance moves the clock forward by d.
func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// Product builds a product; opts mutate the default.
func Product(opts ...func(*domain.Product)) domain.Product {
	p := domain.Product{ID: ProductID, Name: "Midea PortaSplit 12000 BTU", Model: "PortaSplit 12000"}
	for _, o := range opts {
		o(&p)
	}
	return p
}

// Source builds a store-type source (obi) by default.
func Source(opts ...func(*domain.Source)) domain.Source {
	s := domain.Source{Key: SourceStore, Name: "OBI", Type: domain.SourceStore, BaseURL: "https://www.obi.de", LastStatus: domain.StatusNever}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// OnlineSource builds an online source (amazon).
func OnlineSource(opts ...func(*domain.Source)) domain.Source {
	s := domain.Source{Key: SourceOnline, Name: "Amazon", Type: domain.SourceOnline, BaseURL: "https://www.amazon.de", LastStatus: domain.StatusNever}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// StoreNear builds a store about 30 km from PLZ 73035.
func StoreNear(opts ...func(*domain.Store)) domain.Store {
	s := domain.Store{ID: StoreNearID, SourceKey: SourceStore, Name: "OBI Reutlingen", Street: Ptr("Hauptstr. 1"), PLZ: "72760", City: "Reutlingen", Lat: 48.4914, Lon: 9.2043}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// StoreFar builds a store in Berlin, roughly 500 km from PLZ 73035.
func StoreFar(opts ...func(*domain.Store)) domain.Store {
	s := domain.Store{ID: StoreFarID, SourceKey: SourceStore, Name: "OBI Berlin", PLZ: "10115", City: "Berlin", Lat: 52.5321, Lon: 13.3849}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// Snapshot builds an in-stock snapshot for ProductID at StoreNear.
func Snapshot(opts ...func(*domain.AvailabilitySnapshot)) domain.AvailabilitySnapshot {
	s := domain.AvailabilitySnapshot{
		SourceKey: SourceStore, StoreID: Ptr(StoreNearID), ProductID: ProductID,
		InStock: true, Quantity: Ptr(3), PriceCents: Ptr(49900), Currency: "EUR",
		URL: "https://www.obi.de/p/portasplit", CheckedAt: Now,
	}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// OnlineSnapshot builds an in-stock online snapshot (no store).
func OnlineSnapshot(opts ...func(*domain.AvailabilitySnapshot)) domain.AvailabilitySnapshot {
	s := Snapshot(func(s *domain.AvailabilitySnapshot) {
		s.SourceKey, s.StoreID, s.Quantity, s.URL = SourceOnline, nil, nil, "https://www.amazon.de/dp/portasplit"
	})
	for _, o := range opts {
		o(&s)
	}
	return s
}

// Subscription builds an unconfirmed subscription with fixed tokens.
func Subscription(opts ...func(*domain.Subscription)) domain.Subscription {
	s := domain.Subscription{
		Email: "kunde@example.com", PLZ: PLZGoeppingen, RadiusKm: 50,
		ConfirmToken: "confirm-token-0001", UnsubToken: "unsub-token-0001", CreatedAt: Now,
	}
	for _, o := range opts {
		o(&s)
	}
	return s
}

// FakeStore is an in-memory httpapi.DataStore with confirm/unsubscribe state
// mirroring the Postgres store. Set the Err fields to drive failure paths.
type FakeStore struct {
	mu        sync.Mutex
	Products  []domain.Product
	Sources   []domain.Source
	Stores    []domain.Store
	Snapshots []domain.AvailabilitySnapshot
	Subs      []domain.Subscription
	Clock     *FixedClock

	PingErr, ListErr, CreateErr, ConfirmErr, UnsubErr error
}

// NewFakeStore returns a store pre-filled with one product, a store and an
// online source, the near and far stores and one snapshot each.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		Products:  []domain.Product{Product()},
		Sources:   []domain.Source{Source(), OnlineSource()},
		Stores:    []domain.Store{StoreNear(), StoreFar()},
		Snapshots: []domain.AvailabilitySnapshot{Snapshot(), Snapshot(func(s *domain.AvailabilitySnapshot) { s.StoreID = Ptr(StoreFarID) }), OnlineSnapshot()},
		Clock:     NewClock(Now),
	}
}

// Ping implements DataStore.
func (f *FakeStore) Ping(context.Context) error { return f.PingErr }

// ListProducts implements DataStore.
func (f *FakeStore) ListProducts(context.Context) ([]domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Product{}, f.Products...), f.ListErr
}

// ListSources implements DataStore.
func (f *FakeStore) ListSources(context.Context) ([]domain.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Source{}, f.Sources...), f.ListErr
}

// ListStores implements DataStore.
func (f *FakeStore) ListStores(context.Context) ([]domain.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Store{}, f.Stores...), f.ListErr
}

// LatestSnapshots implements DataStore (the fake holds only latest snapshots).
func (f *FakeStore) LatestSnapshots(context.Context) ([]domain.AvailabilitySnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.AvailabilitySnapshot{}, f.Snapshots...), f.ListErr
}

// CreateSubscription implements DataStore; IDs are sub-1, sub-2, ...
func (f *FakeStore) CreateSubscription(_ context.Context, s domain.Subscription) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateErr != nil {
		return "", f.CreateErr
	}
	s.ID = fmt.Sprintf("sub-%d", len(f.Subs)+1)
	s.CreatedAt = f.now()
	f.Subs = append(f.Subs, s)
	return s.ID, nil
}

// ConfirmSubscription implements DataStore. Confirming is idempotent for a
// known token; an unsubscribed row is not re-confirmed.
func (f *FakeStore) ConfirmSubscription(_ context.Context, token string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ConfirmErr != nil {
		return "", false, f.ConfirmErr
	}
	for i := range f.Subs {
		s := &f.Subs[i]
		if token == "" || s.ConfirmToken != token || s.UnsubbedAt != nil {
			continue
		}
		if s.ConfirmedAt == nil {
			s.ConfirmedAt = Ptr(f.now())
		}
		return s.Email, true, nil
	}
	return "", false, nil
}

// UnsubSubscription implements DataStore.
func (f *FakeStore) UnsubSubscription(_ context.Context, token string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.UnsubErr != nil {
		return false, f.UnsubErr
	}
	for i := range f.Subs {
		s := &f.Subs[i]
		if token == "" || s.UnsubToken != token {
			continue
		}
		if s.UnsubbedAt == nil {
			s.UnsubbedAt = Ptr(f.now())
		}
		return true, nil
	}
	return false, nil
}

// Subscriptions returns a copy of the stored subscriptions.
func (f *FakeStore) Subscriptions() []domain.Subscription {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Subscription{}, f.Subs...)
}

func (f *FakeStore) now() time.Time {
	if f.Clock == nil {
		return Now
	}
	return f.Clock.Now()
}
