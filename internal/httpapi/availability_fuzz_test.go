package httpapi

import (
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// entriesFromBytes builds store entries from pairs of bytes: a short name and a
// distance (nil when the byte is odd above 200).
func entriesFromBytes(data []byte) []StoreAvailability {
	var out []StoreAvailability
	for i := 0; i+1 < len(data); i += 2 {
		e := StoreAvailability{Store: domain.Store{ID: string(rune('a' + i/2%26)), Name: string([]byte{'A' + data[i]%4})}}
		if data[i+1] <= 200 {
			d := float64(data[i+1]%8) / 2
			e.DistanceKm = &d
		}
		out = append(out, e)
	}
	return out
}

func outOfOrder(a, b StoreAvailability) bool {
	switch {
	case a.DistanceKm != nil && b.DistanceKm != nil:
		if *a.DistanceKm != *b.DistanceKm {
			return *a.DistanceKm > *b.DistanceKm
		}
		return a.Store.Name > b.Store.Name
	case a.DistanceKm == nil && b.DistanceKm != nil:
		return true
	case a.DistanceKm != nil && b.DistanceKm == nil:
		return false
	default:
		return a.Store.Name > b.Store.Name
	}
}

// FuzzSortStoreEntries: the result is a permutation of the input ordered nearest
// first, ties by name, and entries without a distance after those with one.
func FuzzSortStoreEntries(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1})
	f.Add([]byte{1, 4, 0, 4, 2, 255, 3, 0})
	f.Add([]byte{3, 250, 2, 251, 1, 252, 0, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		entries := entriesFromBytes(data)
		before := map[string]int{}
		for _, e := range entries {
			before[e.Store.ID+"|"+e.Store.Name]++
		}
		sortStoreEntries(entries)
		for _, e := range entries {
			before[e.Store.ID+"|"+e.Store.Name]--
		}
		for k, c := range before {
			if c != 0 {
				t.Fatalf("input %v: entry %q count changed by %d", data, k, -c)
			}
		}
		for i := 1; i < len(entries); i++ {
			if outOfOrder(entries[i-1], entries[i]) {
				t.Fatalf("input %v: entries %d and %d out of order: %+v, %+v", data, i-1, i, entries[i-1], entries[i])
			}
		}
	})
}

// FuzzAssembleAvailability: every returned store has listings, all belonging to
// it and to the product filter, lies within the radius when a center is given and
// keeps the order; online listings are exactly the matching snapshots without a store.
func FuzzAssembleAvailability(f *testing.F) {
	f.Add([]byte{}, true, 50, false)
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, true, 50, true)
	f.Add([]byte{10, 200, 30, 40, 5, 6, 7, 8, 255, 0, 1, 2}, false, 0, false)
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1}, true, 500, true)
	f.Fuzz(func(t *testing.T, data []byte, withCenter bool, radiusKm int, filter bool) {
		// Up to 4 stores near (48, 9), placed by the first bytes.
		var stores []domain.Store
		ids := []string{"s0", "s1", "s2", "s3"}
		for i := 0; i < 4 && i < len(data); i++ {
			stores = append(stores, domain.Store{
				ID: ids[i], Name: string([]byte{'A' + data[i]%3}),
				Lat: 48 + float64(data[i])/50, Lon: 9 + float64(data[i]%16)/10,
			})
		}
		var snaps []domain.AvailabilitySnapshot
		for i := 4; i+1 < len(data); i += 2 {
			b, c := data[i], data[i+1]
			s := domain.AvailabilitySnapshot{
				SourceKey: string([]byte{'x' + b%3}),
				ProductID: string([]byte{'p' + c%2}),
				InStock:   c&4 != 0,
			}
			switch k := b % 6; {
			case k < 4:
				id := ids[k]
				s.StoreID = &id
			case k == 4:
				empty := ""
				s.StoreID = &empty
			}
			if c&8 != 0 {
				p := int(c >> 4)
				s.PriceCents = &p
			}
			snaps = append(snaps, s)
		}
		var center *Center
		if withCenter {
			center = &Center{PLZ: "70173", Lat: 48.7, Lon: 9.2}
		}
		productID := ""
		if filter {
			productID = "p"
		}
		resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, center, radiusKm, productID)

		for i, sa := range resp.Stores {
			if len(sa.Listings) == 0 {
				t.Fatalf("input %v: store %s returned without listings", data, sa.Store.ID)
			}
			for _, l := range sa.Listings {
				if l.StoreID == nil || *l.StoreID != sa.Store.ID {
					t.Fatalf("input %v: store %s carries a listing of store %v", data, sa.Store.ID, l.StoreID)
				}
				if productID != "" && l.ProductID != productID {
					t.Fatalf("input %v: listing of product %q despite filter %q", data, l.ProductID, productID)
				}
			}
			if (sa.DistanceKm != nil) != withCenter {
				t.Fatalf("input %v: store %s distance set = %v, center given = %v", data, sa.Store.ID, sa.DistanceKm != nil, withCenter)
			}
			if sa.DistanceKm != nil && radiusKm > 0 && *sa.DistanceKm > float64(radiusKm) {
				t.Fatalf("input %v: store %s at %.2f km beyond radius %d", data, sa.Store.ID, *sa.DistanceKm, radiusKm)
			}
			if i > 0 && outOfOrder(resp.Stores[i-1], sa) {
				t.Fatalf("input %v: stores %d and %d out of order", data, i-1, i)
			}
		}

		wantOnline := 0
		for _, s := range snaps {
			if (productID == "" || s.ProductID == productID) && (s.StoreID == nil || *s.StoreID == "") {
				wantOnline++
			}
		}
		gotOnline := 0
		for i, o := range resp.Online {
			if i > 0 && resp.Online[i-1].Source.Key >= o.Source.Key {
				t.Fatalf("input %v: online sources not strictly ordered by key", data)
			}
			for _, l := range o.Listings {
				if l.StoreID != nil && *l.StoreID != "" {
					t.Fatalf("input %v: online listing with store %q", data, *l.StoreID)
				}
				if l.SourceKey != o.Source.Key {
					t.Fatalf("input %v: listing of source %q grouped under %q", data, l.SourceKey, o.Source.Key)
				}
				gotOnline++
			}
		}
		if gotOnline != wantOnline {
			t.Fatalf("input %v: %d online listings, want %d", data, gotOnline, wantOnline)
		}
	})
}
