// Package fixturetest holds shared test doubles and fixtures: domain builders
// with fixed IDs, an in-memory store satisfying httpapi.DataStore and
// jobs.Store, a fixed clock and a SQL seed script for integration tests.
package fixturetest

import (
	_ "embed"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// Fixed IDs, matching testdata/seed.sql.
const (
	ProductID      = "11111111-1111-1111-1111-111111111111"
	OtherProductID = "22222222-2222-2222-2222-222222222222"
	StoreID        = "33333333-3333-3333-3333-333333333333"
	SubscriptionID = "44444444-4444-4444-4444-444444444444"
	SourceKeyStore = "mock"
	SourceKeyWeb   = "mockonline"
	PLZ            = "73035" // Göppingen
)

// Epoch is the fixed "now" of the fixtures.
var Epoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// SeedSQL seeds products, sources, a store, snapshots and a confirmed
// subscription with the IDs above. It is idempotent (ON CONFLICT DO NOTHING).
//
//go:embed testdata/seed.sql
var SeedSQL string

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// Product builds a product; mutators override fields.
func Product(mut ...func(*domain.Product)) domain.Product {
	p := domain.Product{ID: ProductID, Name: "Midea PortaSplit", Model: "PortaSplit 12000"}
	for _, m := range mut {
		m(&p)
	}
	return p
}

// Source builds a store-type source.
func Source(mut ...func(*domain.Source)) domain.Source {
	s := domain.Source{Key: SourceKeyStore, Name: "Mock Store", Type: domain.SourceStore,
		BaseURL: "https://mock.example", LastStatus: domain.StatusNever}
	for _, m := range mut {
		m(&s)
	}
	return s
}

// Store builds a physical store located in PLZ 73035.
func Store(mut ...func(*domain.Store)) domain.Store {
	s := domain.Store{ID: StoreID, SourceKey: SourceKeyStore, Name: "Mock Göppingen",
		Street: Ptr("Hauptstr. 1"), PLZ: PLZ, City: "Göppingen", Lat: 48.7036, Lon: 9.6527}
	for _, m := range mut {
		m(&s)
	}
	return s
}

// Snapshot builds an in-stock snapshot for Product at Store, checked at Epoch.
func Snapshot(mut ...func(*domain.AvailabilitySnapshot)) domain.AvailabilitySnapshot {
	s := domain.AvailabilitySnapshot{SourceKey: SourceKeyStore, StoreID: Ptr(StoreID), ProductID: ProductID,
		InStock: true, Quantity: Ptr(3), PriceCents: Ptr(59900), Currency: "EUR",
		URL: "https://mock.example/p", CheckedAt: Epoch}
	for _, m := range mut {
		m(&s)
	}
	return s
}

// Subscription builds an unconfirmed subscription with fixed tokens.
func Subscription(mut ...func(*domain.Subscription)) domain.Subscription {
	s := domain.Subscription{ID: SubscriptionID, Email: "user@example.com", PLZ: PLZ, RadiusKm: 50,
		ConfirmToken: "confirm-token", UnsubToken: "unsub-token", CreatedAt: Epoch}
	for _, m := range mut {
		m(&s)
	}
	return s
}

// Confirmed marks a subscription confirmed at Epoch.
func Confirmed(s *domain.Subscription) { s.ConfirmedAt = Ptr(Epoch) }

// Clock is a manually advanced clock.
type Clock struct{ T time.Time }

// NewClock returns a clock set to Epoch.
func NewClock() *Clock { return &Clock{T: Epoch} }

// Now returns the current fake time.
func (c *Clock) Now() time.Time { return c.T }

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) { c.T = c.T.Add(d) }
