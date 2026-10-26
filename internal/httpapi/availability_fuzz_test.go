package httpapi

import (
	"fmt"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// FuzzSortStoreEntries: after sorting, stores with a distance come first,
// ordered by distance then name; stores without one follow by name; no entry
// is lost or duplicated.
func FuzzSortStoreEntries(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3, 4, 5, 6})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{255, 1, 7, 128, 1, 7, 3, 9, 2, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		var stores []StoreAvailability
		for i := 0; i+2 < len(data) && len(stores) < 64; i += 3 {
			st := StoreAvailability{Store: domain.Store{ID: fmt.Sprint(len(stores)), Name: string(rune('a' + data[i+1]%4))}}
			if data[i]%3 != 0 {
				d := float64(data[i+2]%8) / 2
				st.DistanceKm = &d
			}
			stores = append(stores, st)
		}
		ids := map[string]bool{}
		for _, s := range stores {
			ids[s.Store.ID] = true
		}

		sortStoreEntries(stores)

		if len(stores) != len(ids) {
			t.Fatalf("len %d, want %d", len(stores), len(ids))
		}
		for _, s := range stores {
			if !ids[s.Store.ID] {
				t.Fatalf("entry %s lost or duplicated", s.Store.ID)
			}
			delete(ids, s.Store.ID)
		}
		for i := 1; i < len(stores); i++ {
			a, b := stores[i-1], stores[i]
			switch {
			case a.DistanceKm == nil && b.DistanceKm != nil:
				t.Errorf("position %d: store without distance before one with distance", i)
			case a.DistanceKm != nil && b.DistanceKm != nil:
				if *a.DistanceKm > *b.DistanceKm || (*a.DistanceKm == *b.DistanceKm && a.Store.Name > b.Store.Name) {
					t.Errorf("position %d: (%v,%q) before (%v,%q)", i, *a.DistanceKm, a.Store.Name, *b.DistanceKm, b.Store.Name)
				}
			case a.DistanceKm == nil && b.DistanceKm == nil:
				if a.Store.Name > b.Store.Name {
					t.Errorf("position %d: %q before %q", i, a.Store.Name, b.Store.Name)
				}
			}
		}
	})
}

// FuzzSortListings: after sorting, in-stock listings come first, then price
// ascending with nil prices last, then source key; no listing is lost.
func FuzzSortListings(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 5, 0, 0, 3, 1, 1, 0, 2})
	f.Add([]byte{0, 0, 0, 0, 0, 0})
	f.Add([]byte{255, 255, 255, 2, 7, 9, 1, 1, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		var ls []domain.Listing
		for i := 0; i+2 < len(data) && len(ls) < 64; i += 3 {
			l := domain.Listing{InStock: data[i]%2 == 1, SourceKey: string(rune('a' + data[i+2]%4)), URL: fmt.Sprint(len(ls))}
			if data[i+1]%4 != 0 {
				p := int(data[i+1] % 16)
				l.PriceCents = &p
			}
			ls = append(ls, l)
		}
		n := len(ls)

		sortListings(ls)

		seen := map[string]bool{}
		for _, l := range ls {
			seen[l.URL] = true
		}
		if len(ls) != n || len(seen) != n {
			t.Fatalf("listings lost or duplicated: %d unique of %d", len(seen), n)
		}
		for i := 1; i < len(ls); i++ {
			a, b := ls[i-1], ls[i]
			if a.InStock != b.InStock {
				if !a.InStock {
					t.Errorf("position %d: out-of-stock before in-stock", i)
				}
				continue
			}
			switch {
			case a.PriceCents == nil && b.PriceCents != nil:
				t.Errorf("position %d: nil price before %d", i, *b.PriceCents)
			case a.PriceCents != nil && b.PriceCents != nil && *a.PriceCents > *b.PriceCents:
				t.Errorf("position %d: price %d before %d", i, *a.PriceCents, *b.PriceCents)
			case (a.PriceCents == nil) == (b.PriceCents == nil) &&
				(a.PriceCents == nil || *a.PriceCents == *b.PriceCents) && a.SourceKey > b.SourceKey:
				t.Errorf("position %d: source %q before %q at equal price", i, a.SourceKey, b.SourceKey)
			}
		}
	})
}
