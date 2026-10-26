package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

var (
	aqaaSinkResp    AvailabilityResponse
	aqaaSinkEntry   StoreAvailability
	aqaaSinkInt     int
	aqaaSinkBool    bool
	aqaaSinkEntries []StoreAvailability
	aqaaSinkListing []domain.Listing
)

// aqaaBenchStore is an in-memory DataStore that counts the queries it serves.
type aqaaBenchStore struct {
	stores    []domain.Store
	sources   []domain.Source
	snapshots []domain.AvailabilitySnapshot
	byStore   map[string][]domain.AvailabilitySnapshot
	queries   int64
}

func (f *aqaaBenchStore) Ping(context.Context) error { return nil }
func (f *aqaaBenchStore) ListProducts(context.Context) ([]domain.Product, error) {
	return nil, nil
}
func (f *aqaaBenchStore) ListSources(context.Context) ([]domain.Source, error) {
	atomic.AddInt64(&f.queries, 1)
	return f.sources, nil
}
func (f *aqaaBenchStore) ListStores(context.Context) ([]domain.Store, error) {
	atomic.AddInt64(&f.queries, 1)
	return f.stores, nil
}
func (f *aqaaBenchStore) SearchStores(context.Context, string) ([]domain.Store, error) {
	atomic.AddInt64(&f.queries, 1)
	// The repository returns a fresh slice per call.
	out := make([]domain.Store, len(f.stores))
	copy(out, f.stores)
	return out, nil
}
func (f *aqaaBenchStore) LatestSnapshots(context.Context) ([]domain.AvailabilitySnapshot, error) {
	atomic.AddInt64(&f.queries, 1)
	return f.snapshots, nil
}
func (f *aqaaBenchStore) LatestSnapshotsForStore(_ context.Context, id string) ([]domain.AvailabilitySnapshot, error) {
	atomic.AddInt64(&f.queries, 1)
	return f.byStore[id], nil
}
func (f *aqaaBenchStore) CreateSubscription(context.Context, domain.Subscription) (string, error) {
	return "", nil
}
func (f *aqaaBenchStore) ConfirmSubscription(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (f *aqaaBenchStore) UnsubSubscription(context.Context, string) (bool, error) {
	return false, nil
}

// aqaaInputs builds nStores stores spread over southern Germany, perStore
// snapshots for each (alternating stock, some nil prices) and nOnline online
// snapshots over three online sources. Deterministic.
func aqaaInputs(nStores, perStore, nOnline int) availabilityInputs {
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	sources := []domain.Source{
		{Key: "obi", Name: "OBI", Type: domain.SourceStore},
		{Key: "bauhaus", Name: "BAUHAUS", Type: domain.SourceStore},
		{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline},
		{Key: "otto", Name: "Otto", Type: domain.SourceOnline},
		// "ebay" deliberately missing: unknown-source fallback.
	}
	stores := make([]domain.Store, nStores)
	snaps := make([]domain.AvailabilitySnapshot, 0, nStores*perStore+nOnline)
	for i := 0; i < nStores; i++ {
		id := fmt.Sprintf("store-%06d", i)
		src := "obi"
		if i%2 == 1 {
			src = "bauhaus"
		}
		stores[i] = domain.Store{
			ID:        id,
			SourceKey: src,
			Name:      fmt.Sprintf("Markt %d", (i*7919)%nStores),
			PLZ:       fmt.Sprintf("%05d", 70000+i%9000),
			City:      fmt.Sprintf("Stadt %d", i%300),
			Lat:       47.5 + float64((i*37)%400)/100,
			Lon:       7.5 + float64((i*53)%500)/100,
		}
		for j := 0; j < perStore; j++ {
			sid := id
			var price *int
			if (i+j)%5 != 0 {
				p := 50000 + (i*13+j*29)%20000
				price = &p
			}
			snaps = append(snaps, domain.AvailabilitySnapshot{
				ID:         fmt.Sprintf("s-%d-%d", i, j),
				SourceKey:  src,
				StoreID:    &sid,
				ProductID:  fmt.Sprintf("prod-%d", j%4),
				InStock:    (i+j)%3 == 0,
				PriceCents: price,
				Currency:   "EUR",
				URL:        "https://example.invalid/p",
				CheckedAt:  base.Add(time.Duration(j) * time.Minute),
			})
		}
	}
	online := []string{"amazon", "otto", "ebay"}
	for k := 0; k < nOnline; k++ {
		p := 45000 + (k*31)%30000
		snaps = append(snaps, domain.AvailabilitySnapshot{
			ID:         fmt.Sprintf("o-%d", k),
			SourceKey:  online[k%len(online)],
			ProductID:  fmt.Sprintf("prod-%d", k%4),
			InStock:    k%2 == 0,
			PriceCents: &p,
			Currency:   "EUR",
			CheckedAt:  base,
		})
	}
	// Interleave store and online snapshots like a DISTINCT ON result would.
	return availabilityInputs{snapshots: snaps, stores: stores, sources: sources}
}

func aqaaFakeFrom(in availabilityInputs) *aqaaBenchStore {
	byStore := make(map[string][]domain.AvailabilitySnapshot, len(in.stores))
	for _, s := range in.snapshots {
		if s.StoreID != nil {
			byStore[*s.StoreID] = append(byStore[*s.StoreID], s)
		}
	}
	return &aqaaBenchStore{stores: in.stores, sources: in.sources, snapshots: in.snapshots, byStore: byStore}
}

var aqaaSizes = []struct {
	stores, perStore, online int
}{
	{100, 10, 50},
	{1000, 10, 500},
	{5000, 10, 2000},
}

// P2 / t7: assembleAvailability calls storeEntry per store, each scanning
// all snapshots: stores × snapshots.
func BenchmarkAssembleAvailability(b *testing.B) {
	center := &Center{PLZ: "73035", Lat: 48.7, Lon: 9.65}
	for _, sz := range aqaaSizes {
		in := aqaaInputs(sz.stores, sz.perStore, sz.online)
		name := fmt.Sprintf("stores=%d/snaps=%d", sz.stores, len(in.snapshots))
		b.Run(name+"/center", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				aqaaSinkResp = assembleAvailability(in, center, 50, "")
			}
		})
		b.Run(name+"/noCenter_product", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				aqaaSinkResp = assembleAvailability(in, nil, 0, "prod-1")
			}
		})
	}
}

// t6: storeEntry for a single store scans the whole snapshot list.
func BenchmarkStoreEntry(b *testing.B) {
	center := &Center{PLZ: "73035", Lat: 48.7, Lon: 9.65}
	for _, sz := range aqaaSizes {
		in := aqaaInputs(sz.stores, sz.perStore, sz.online)
		st := in.stores[len(in.stores)/2]
		b.Run(fmt.Sprintf("snaps=%d", len(in.snapshots)), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				aqaaSinkEntry = storeEntry(st, in.snapshots, center, "")
			}
		})
	}
}

// t6: sortStoreEntries with mixed nil / known distances and name tie-breaks.
func BenchmarkSortStoreEntries(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		src := make([]StoreAvailability, n)
		for i := range src {
			src[i].Store.Name = fmt.Sprintf("Markt %d", (i*7919)%n)
			if i%4 != 0 {
				d := float64((i*37)%500) / 10
				src[i].DistanceKm = &d
			}
		}
		work := make([]StoreAvailability, n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				copy(work, src)
				b.StartTimer()
				sortStoreEntries(work)
			}
			aqaaSinkEntries = work
		})
	}
}

// t6: sortListings with in-stock first, nil prices and source-key tie-breaks.
func BenchmarkSortListings(b *testing.B) {
	keys := []string{"obi", "bauhaus", "amazon", "otto"}
	for _, n := range []int{10, 100, 1000} {
		src := make([]domain.Listing, n)
		for i := range src {
			src[i].SourceKey = keys[i%len(keys)]
			src[i].InStock = i%3 == 0
			if i%5 != 0 {
				p := 50000 + (i*13)%200
				src[i].PriceCents = &p
			}
		}
		work := make([]domain.Listing, n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				copy(work, src)
				b.StartTimer()
				sortListings(work)
			}
			aqaaSinkListing = work
		})
	}
}

// t5: intParam over default, valid, out-of-range, non-numeric and overflow.
func BenchmarkIntParam(b *testing.B) {
	cases := []struct{ name, v string }{
		{"empty", ""},
		{"valid", "42"},
		{"aboveHi", "101"},
		{"nonNumeric", "abc"},
		{"overflow", "99999999999999999999999"},
	}
	for _, c := range cases {
		v := c.v
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				n, ok := intParam(v, defaultPageSize, 1, maxPageSize)
				aqaaSinkInt += n
				aqaaSinkBool = ok
			}
		})
	}
}

// P7: every availability request reloads all inputs and reassembles the
// response from scratch (plus JSON encoding); queries/op shows no caching.
func BenchmarkHandleAvailability(b *testing.B) {
	for _, sz := range aqaaSizes {
		in := aqaaInputs(sz.stores, sz.perStore, sz.online)
		fake := aqaaFakeFrom(in)
		srv := &Server{store: fake}
		req := httptest.NewRequest(http.MethodGet, "/api/availability?plz=73035&radius=500", nil)
		b.Run(fmt.Sprintf("stores=%d/snaps=%d", sz.stores, len(in.snapshots)), func(b *testing.B) {
			b.ReportAllocs()
			atomic.StoreInt64(&fake.queries, 0)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				srv.handleAvailability(rec, req)
				aqaaSinkInt += rec.Body.Len()
			}
			b.ReportMetric(float64(atomic.LoadInt64(&fake.queries))/float64(b.N), "queries/op")
		})
	}
}

// P1 / P4: handleStoreSearch builds an entry for every matching store and
// sorts them all for one page (grows with the store count), then issues one
// LatestSnapshotsForStore query per store on the page (queries/op grows with
// pageSize).
func BenchmarkHandleStoreSearch(b *testing.B) {
	for _, nStores := range []int{1000, 10000, 50000} {
		in := aqaaInputs(nStores, 4, 0)
		fake := aqaaFakeFrom(in)
		srv := &Server{store: fake}
		for _, ps := range []int{20, 100} {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/stores?plz=73035&page=1&pageSize=%d", ps), nil)
			b.Run(fmt.Sprintf("stores=%d/pageSize=%d", nStores, ps), func(b *testing.B) {
				b.ReportAllocs()
				atomic.StoreInt64(&fake.queries, 0)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rec := httptest.NewRecorder()
					srv.handleStoreSearch(rec, req)
					aqaaSinkInt += rec.Body.Len()
				}
				b.ReportMetric(float64(atomic.LoadInt64(&fake.queries))/float64(b.N), "queries/op")
			})
		}
	}
}
