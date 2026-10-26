package jobs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/notify"
)

var (
	benchSinkTransitions []transition
	benchSinkSent        int
)

// countingStore is an in-memory Store that counts every call, so the benchmarks
// can report the number of store round trips per poll as a metric.
type countingStore struct {
	calls    int
	prev     map[string]domain.AvailabilitySnapshot
	stores   map[string]domain.Store
	subs     []domain.Subscription
	products map[string]string
}

func (c *countingStore) ListProducts(context.Context) ([]domain.Product, error) {
	c.calls++
	return nil, nil
}
func (c *countingStore) UpsertSource(context.Context, domain.Source) error { c.calls++; return nil }
func (c *countingStore) UpsertStore(_ context.Context, s domain.Store) (string, error) {
	c.calls++
	return s.ID, nil
}
func (c *countingStore) InsertSnapshot(context.Context, domain.AvailabilitySnapshot) error {
	c.calls++
	return nil
}
func (c *countingStore) LatestForKey(_ context.Context, src, store, prod string) (domain.AvailabilitySnapshot, bool, error) {
	c.calls++
	s, ok := c.prev[key(src, store, prod)]
	return s, ok, nil
}
func (c *countingStore) UpdateSourceStatus(context.Context, string, time.Time, domain.LastStatus, *string) error {
	c.calls++
	return nil
}
func (c *countingStore) ListConfirmedSubscriptions(context.Context) ([]domain.Subscription, error) {
	c.calls++
	return c.subs, nil
}
func (c *countingStore) GetStore(_ context.Context, id string) (domain.Store, bool, error) {
	c.calls++
	s, ok := c.stores[id]
	return s, ok, nil
}
func (c *countingStore) GetProductName(_ context.Context, id string) (string, bool, error) {
	c.calls++
	n, ok := c.products[id]
	return n, ok, nil
}

// RecordNotification always claims the pair so every iteration does the full work.
func (c *countingStore) RecordNotification(context.Context, string, string, string, string, string) (bool, error) {
	c.calls++
	return true, nil
}

// staticSource returns a fixed set of listings and stores.
type staticSource struct {
	listings []domain.Listing
	stores   []domain.Store
}

func (s *staticSource) Key() string { return "obi" }
func (s *staticSource) Meta() domain.Source {
	return domain.Source{Key: "obi", Name: "OBI", Type: domain.SourceType("store")}
}
func (s *staticSource) Fetch(context.Context, []domain.Product) ([]domain.Listing, []domain.Store, error) {
	return s.listings, s.stores, nil
}

type nopMailer struct{}

func (nopMailer) Send(notify.Message) error { return nil }

// buildPollFixture builds markets×products listings (all in-stock, all previously
// out-of-stock) for markets around Munich, plus the matching fake store.
func buildPollFixture(markets, products int) (*staticSource, *countingStore) {
	src := &staticSource{}
	cs := &countingStore{
		prev:     map[string]domain.AvailabilitySnapshot{},
		stores:   map[string]domain.Store{},
		products: map[string]string{},
	}
	for m := 0; m < markets; m++ {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", m)
		st := domain.Store{ID: id, SourceKey: "obi", Name: fmt.Sprintf("OBI %d", m), City: "München",
			Lat: 48.1372 + float64(m%10)*0.01, Lon: 11.5755 + float64(m%7)*0.01}
		src.stores = append(src.stores, st)
		cs.stores[id] = st
		for p := 0; p < products; p++ {
			pid := fmt.Sprintf("p%d", p)
			cs.products[pid] = "Midea Portasplit"
			sid := id
			src.listings = append(src.listings, domain.Listing{
				SourceKey: "obi", ProductID: pid, StoreID: &sid, InStock: true, Currency: "EUR",
				URL: "https://example.invalid/p",
			})
			cs.prev[key("obi", id, pid)] = domain.AvailabilitySnapshot{InStock: false}
		}
	}
	return src, cs
}

func benchPoller(cs *countingStore) *Poller {
	return New(Config{
		Store:         cs,
		Mailer:        nopMailer{},
		PublicBaseURL: "https://example.invalid",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:           func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) },
	})
}

// BenchmarkPollerPollSource shows the per-listing LatestForKey + InsertSnapshot
// round trips (plus one UpsertStore per market): store-calls/op grows ~2 per listing.
func BenchmarkPollerPollSource(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("markets=%d_products=2", n), func(b *testing.B) {
			src, cs := buildPollFixture(n, 2)
			p := benchPoller(cs)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ts, err := p.pollSource(ctx, src, nil)
				if err != nil {
					b.Fatal(err)
				}
				benchSinkTransitions = ts
			}
			b.StopTimer()
			b.ReportMetric(float64(cs.calls)/float64(b.N), "store-calls/op")
		})
	}
}

// BenchmarkPollerNotify shows the transitions × subscriptions loop: each match
// does GetStore in matches() and again GetStore + GetProductName in
// sendAvailability, with no caching across the loop.
func BenchmarkPollerNotify(b *testing.B) {
	for _, tc := range []struct{ transitions, subs int }{{10, 10}, {100, 10}, {100, 100}, {500, 100}} {
		b.Run(fmt.Sprintf("transitions=%d_subs=%d", tc.transitions, tc.subs), func(b *testing.B) {
			src, cs := buildPollFixture(tc.transitions, 1)
			for s := 0; s < tc.subs; s++ {
				cs.subs = append(cs.subs, confirmedSub(fmt.Sprintf("s%d", s), fmt.Sprintf("u%d@x.de", s), "80331", 50, nil))
			}
			trs := make([]transition, 0, len(src.listings))
			for _, l := range src.listings {
				trs = append(trs, transition{listing: l})
			}
			p := benchPoller(cs)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sent, err := p.notify(ctx, trs)
				if err != nil {
					b.Fatal(err)
				}
				benchSinkSent = sent
			}
			b.StopTimer()
			b.ReportMetric(float64(cs.calls)/float64(b.N), "store-calls/op")
		})
	}
}
