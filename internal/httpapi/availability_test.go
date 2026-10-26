package httpapi

import (
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
)

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func TestAssemble_SplitStoreVsOnline(t *testing.T) {
	stores := []domain.Store{
		{ID: "s1", SourceKey: "mock", Name: "OBI Berlin", PLZ: "10115", City: "Berlin", Lat: 52.53, Lon: 13.38},
	}
	sources := []domain.Source{
		{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline},
		{Key: "mock", Name: "Mock", Type: domain.SourceStore},
	}
	snaps := []domain.AvailabilitySnapshot{
		{SourceKey: "mock", StoreID: sp("s1"), ProductID: "p1", InStock: true, Currency: "EUR"},
		{SourceKey: "amazon", StoreID: nil, ProductID: "p1", InStock: true, Currency: "EUR"},
	}
	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores, sources: sources}, nil, 50, "")
	if len(resp.Stores) != 1 {
		t.Fatalf("stores = %d, want 1", len(resp.Stores))
	}
	if len(resp.Online) != 1 {
		t.Fatalf("online = %d, want 1", len(resp.Online))
	}
	if resp.Stores[0].DistanceKm != nil {
		t.Error("distance should be nil without center")
	}
	if resp.Online[0].Source.Name != "Amazon" {
		t.Errorf("online source = %q", resp.Online[0].Source.Name)
	}
}

func TestAssemble_RadiusFilter(t *testing.T) {
	stores := []domain.Store{
		{ID: "berlin", Name: "OBI Berlin", Lat: 52.53, Lon: 13.38},
		{ID: "munich", Name: "OBI Munich", Lat: 48.14, Lon: 11.58},
	}
	snaps := []domain.AvailabilitySnapshot{
		{SourceKey: "mock", StoreID: sp("berlin"), ProductID: "p1", InStock: true, Currency: "EUR"},
		{SourceKey: "mock", StoreID: sp("munich"), ProductID: "p1", InStock: true, Currency: "EUR"},
	}
	pt, _ := geo.Lookup("10115") // Berlin
	center := &Center{PLZ: "10115", Lat: pt.Lat, Lon: pt.Lon}

	// 100km radius: only Berlin store.
	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, center, 100, "")
	if len(resp.Stores) != 1 || resp.Stores[0].Store.ID != "berlin" {
		t.Fatalf("expected only Berlin within 100km, got %+v", resp.Stores)
	}
	if resp.Stores[0].DistanceKm == nil {
		t.Fatal("distance should be set with center")
	}

	// 600km radius: both, Berlin (nearest) first.
	resp = assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, center, 600, "")
	if len(resp.Stores) != 2 {
		t.Fatalf("expected 2 stores within 600km, got %d", len(resp.Stores))
	}
	if resp.Stores[0].Store.ID != "berlin" {
		t.Errorf("nearest-first ordering broken: %s before berlin", resp.Stores[0].Store.ID)
	}
}

func TestAssemble_ProductFilter(t *testing.T) {
	stores := []domain.Store{{ID: "s1", Name: "OBI"}}
	snaps := []domain.AvailabilitySnapshot{
		{SourceKey: "mock", StoreID: sp("s1"), ProductID: "p1", InStock: true, Currency: "EUR"},
		{SourceKey: "mock", StoreID: sp("s1"), ProductID: "p2", InStock: true, Currency: "EUR"},
	}
	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, nil, 50, "p1")
	if len(resp.Stores) != 1 {
		t.Fatalf("stores = %d", len(resp.Stores))
	}
	if len(resp.Stores[0].Listings) != 1 || resp.Stores[0].Listings[0].ProductID != "p1" {
		t.Fatalf("product filter failed: %+v", resp.Stores[0].Listings)
	}
}

func fp(f float64) *float64 { return &f }

func entryIDs(es []StoreAvailability) []string {
	ids := make([]string, len(es))
	for i, e := range es {
		ids[i] = e.Store.ID
	}
	return ids
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortStoreEntries(t *testing.T) {
	entry := func(id, name string, d *float64) StoreAvailability {
		return StoreAvailability{Store: domain.Store{ID: id, Name: name}, DistanceKm: d}
	}
	tests := []struct {
		name string
		in   []StoreAvailability
		want []string
	}{
		{
			name: "nearest first",
			in:   []StoreAvailability{entry("far", "A", fp(30)), entry("near", "Z", fp(1.5)), entry("mid", "M", fp(10))},
			want: []string{"near", "mid", "far"},
		},
		{
			name: "equal distance broken by name",
			in:   []StoreAvailability{entry("b", "OBI B", fp(5)), entry("a", "OBI A", fp(5)), entry("c", "OBI C", fp(2))},
			want: []string{"c", "a", "b"},
		},
		{
			name: "nil distance after known distance",
			in:   []StoreAvailability{entry("nil1", "A", nil), entry("d", "Z", fp(400)), entry("nil2", "B", nil), entry("e", "Y", fp(0))},
			want: []string{"e", "d", "nil1", "nil2"},
		},
		{
			name: "no distances ordered by name",
			in:   []StoreAvailability{entry("c", "OBI Cottbus", nil), entry("a", "OBI Aachen", nil), entry("b", "OBI Bremen", nil)},
			want: []string{"a", "b", "c"},
		},
		{
			name: "zero distance first",
			in:   []StoreAvailability{entry("x", "A", fp(0.1)), entry("y", "B", fp(0))},
			want: []string{"y", "x"},
		},
		{
			name: "single entry",
			in:   []StoreAvailability{entry("only", "A", fp(1))},
			want: []string{"only"},
		},
		{
			name: "empty",
			in:   []StoreAvailability{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortStoreEntries(tt.in)
			if got := entryIDs(tt.in); !equalIDs(got, tt.want) {
				t.Fatalf("order = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortStoreEntries_NilSlice(t *testing.T) {
	var es []StoreAvailability
	sortStoreEntries(es) // must not panic
	if es != nil {
		t.Fatalf("expected nil slice to stay nil, got %v", es)
	}
}

func TestStoreEntry_FiltersByStore(t *testing.T) {
	st := domain.Store{ID: "s1", Name: "OBI Esslingen", Lat: 48.74, Lon: 9.31}
	snaps := []domain.AvailabilitySnapshot{
		snapFixture("a1", "p1", "s1", true),
		snapFixture("a2", "p1", "s2", true),  // other store
		snapFixture("a3", "p1", "", true),    // online (nil store)
		snapFixture("a4", "p2", "s1", false), // same store, other product
	}
	empty := snapFixture("a5", "p1", "", true)
	empty.StoreID = ptr("") // empty store ID is not this store
	snaps = append(snaps, empty)

	got := storeEntry(st, snaps, nil, "")
	if got.Store.ID != "s1" {
		t.Fatalf("store = %+v", got.Store)
	}
	if len(got.Listings) != 2 {
		t.Fatalf("listings = %d, want 2: %+v", len(got.Listings), got.Listings)
	}
	for _, l := range got.Listings {
		if l.StoreID == nil || *l.StoreID != "s1" {
			t.Errorf("listing from wrong store: %+v", l)
		}
	}
}

func TestStoreEntry_ProductFilter(t *testing.T) {
	st := domain.Store{ID: "s1", Name: "OBI"}
	snaps := []domain.AvailabilitySnapshot{
		snapFixture("a1", "p1", "s1", true),
		snapFixture("a2", "p2", "s1", true),
		snapFixture("a3", "p2", "s1", false),
	}
	tests := []struct {
		product string
		want    int
	}{
		{"", 3},
		{"p1", 1},
		{"p2", 2},
		{"p-unknown", 0},
	}
	for _, tt := range tests {
		t.Run("product="+tt.product, func(t *testing.T) {
			got := storeEntry(st, snaps, nil, tt.product)
			if len(got.Listings) != tt.want {
				t.Fatalf("listings = %d, want %d", len(got.Listings), tt.want)
			}
			for _, l := range got.Listings {
				if tt.product != "" && l.ProductID != tt.product {
					t.Errorf("listing for product %q leaked through filter %q", l.ProductID, tt.product)
				}
			}
		})
	}
}

func TestStoreEntry_NoSnapshotsGivesEmptyNonNilListings(t *testing.T) {
	got := storeEntry(domain.Store{ID: "s1"}, nil, nil, "")
	if got.Listings == nil {
		t.Fatal("listings must be an empty slice, not nil")
	}
	if len(got.Listings) != 0 {
		t.Fatalf("listings = %d, want 0", len(got.Listings))
	}
}

func TestStoreEntry_ListingOrder(t *testing.T) {
	st := domain.Store{ID: "s1"}
	mk := func(src string, inStock bool, price *int) domain.AvailabilitySnapshot {
		sn := snapFixture(src, "p1", "s1", inStock)
		sn.SourceKey = src
		sn.PriceCents = price
		return sn
	}
	snaps := []domain.AvailabilitySnapshot{
		mk("out-cheap", false, ip(100)),
		mk("in-noprice", true, nil),
		mk("in-b", true, ip(500)),
		mk("in-a", true, ip(500)),
		mk("in-cheap", true, ip(200)),
		mk("out-noprice", false, nil),
	}
	got := storeEntry(st, snaps, nil, "")
	want := []string{"in-cheap", "in-a", "in-b", "in-noprice", "out-cheap", "out-noprice"}
	if len(got.Listings) != len(want) {
		t.Fatalf("listings = %d, want %d", len(got.Listings), len(want))
	}
	for i, l := range got.Listings {
		if l.SourceKey != want[i] {
			t.Fatalf("listing[%d] = %q, want %q (all: %+v)", i, l.SourceKey, want[i], got.Listings)
		}
	}
}

func TestStoreEntry_ListingCopiesSnapshotFields(t *testing.T) {
	sn := snapFixture("a1", "p1", "s1", true)
	sn.Quantity = ip(7)
	sn.PriceCents = ip(129900)
	got := storeEntry(domain.Store{ID: "s1"}, []domain.AvailabilitySnapshot{sn}, nil, "")
	if len(got.Listings) != 1 {
		t.Fatalf("listings = %d", len(got.Listings))
	}
	l := got.Listings[0]
	if l.SourceKey != "obi" || l.ProductID != "p1" || !l.InStock || l.Currency != "EUR" ||
		l.URL != sn.URL || !l.CheckedAt.Equal(fixtureTime) ||
		l.Quantity == nil || *l.Quantity != 7 || l.PriceCents == nil || *l.PriceCents != 129900 ||
		l.StoreID == nil || *l.StoreID != "s1" {
		t.Fatalf("listing fields not copied: %+v", l)
	}
}

func TestStoreEntry_Distance(t *testing.T) {
	st := domain.Store{ID: "s3", Name: "OBI Berlin", Lat: 52.53, Lon: 13.38}

	t.Run("nil without center", func(t *testing.T) {
		if got := storeEntry(st, nil, nil, ""); got.DistanceKm != nil {
			t.Fatalf("distance = %v, want nil", *got.DistanceKm)
		}
	})
	t.Run("zero at store location", func(t *testing.T) {
		got := storeEntry(st, nil, &Center{PLZ: "x", Lat: st.Lat, Lon: st.Lon}, "")
		if got.DistanceKm == nil || *got.DistanceKm > 1e-9 {
			t.Fatalf("distance = %v, want 0", got.DistanceKm)
		}
	})
	t.Run("matches geo distance", func(t *testing.T) {
		pt, ok := geo.Lookup("10115")
		if !ok {
			t.Skip("plz 10115 not in geo table")
		}
		c := &Center{PLZ: "10115", Lat: pt.Lat, Lon: pt.Lon}
		got := storeEntry(st, nil, c, "")
		want := geo.DistanceKm(pt, geo.Point{Lat: st.Lat, Lon: st.Lon})
		if got.DistanceKm == nil || *got.DistanceKm != want {
			t.Fatalf("distance = %v, want %v", got.DistanceKm, want)
		}
	})
	t.Run("set even without listings", func(t *testing.T) {
		got := storeEntry(st, nil, &Center{Lat: 48.14, Lon: 11.58}, "")
		if got.DistanceKm == nil || *got.DistanceKm < 400 {
			t.Fatalf("distance Munich-Berlin = %v, want > 400km", got.DistanceKm)
		}
	})
}

func TestAssemble_OmitsStoresWithoutListings(t *testing.T) {
	stores := []domain.Store{
		{ID: "s1", Name: "OBI A", Lat: 52.53, Lon: 13.38},
		{ID: "s2", Name: "OBI B", Lat: 52.53, Lon: 13.38},
		{ID: "s3", Name: "OBI C", Lat: 52.53, Lon: 13.38},
	}
	snaps := []domain.AvailabilitySnapshot{
		snapFixture("a1", "p1", "s1", false),
		snapFixture("a2", "p2", "s3", true),
	}
	t.Run("all products", func(t *testing.T) {
		resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, nil, 50, "")
		if got := entryIDs(resp.Stores); !equalIDs(got, []string{"s1", "s3"}) {
			t.Fatalf("stores = %v, want [s1 s3]", got)
		}
	})
	t.Run("product filter drops stores without that product", func(t *testing.T) {
		resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, nil, 50, "p2")
		if got := entryIDs(resp.Stores); !equalIDs(got, []string{"s3"}) {
			t.Fatalf("stores = %v, want [s3]", got)
		}
	})
	t.Run("no snapshots", func(t *testing.T) {
		resp := assembleAvailability(availabilityInputs{stores: stores}, nil, 50, "")
		if len(resp.Stores) != 0 || len(resp.Online) != 0 {
			t.Fatalf("expected nothing, got stores=%v online=%v", resp.Stores, resp.Online)
		}
	})
}

func TestAssemble_NoCenterNoRadiusFilterAndNameOrder(t *testing.T) {
	stores := []domain.Store{
		{ID: "munich", Name: "OBI München", Lat: 48.14, Lon: 11.58},
		{ID: "berlin", Name: "OBI Berlin", Lat: 52.53, Lon: 13.38},
	}
	snaps := []domain.AvailabilitySnapshot{
		snapFixture("a1", "p1", "munich", true),
		snapFixture("a2", "p1", "berlin", true),
	}
	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores}, nil, 1, "")
	if got := entryIDs(resp.Stores); !equalIDs(got, []string{"berlin", "munich"}) {
		t.Fatalf("stores = %v, want [berlin munich] (no radius filter without plz, ordered by name)", got)
	}
	for _, s := range resp.Stores {
		if s.DistanceKm != nil {
			t.Errorf("store %s has distance %v without center", s.Store.ID, *s.DistanceKm)
		}
	}
	if resp.Center != nil {
		t.Errorf("center = %+v, want nil", resp.Center)
	}
}

func TestAssemble_RadiusOrderingWithCenter(t *testing.T) {
	// Center at Esslingen; Stuttgart ~10km, Munich ~190km, Berlin ~510km.
	center := &Center{PLZ: "73730", Lat: 48.74, Lon: 9.31}
	stores := []domain.Store{
		{ID: "berlin", Name: "OBI Berlin", Lat: 52.53, Lon: 13.38},
		{ID: "munich", Name: "OBI München", Lat: 48.14, Lon: 11.58},
		{ID: "stuttgart", Name: "OBI Stuttgart", Lat: 48.78, Lon: 9.18},
		{ID: "esslingen", Name: "OBI Esslingen", Lat: 48.74, Lon: 9.31},
	}
	var snaps []domain.AvailabilitySnapshot
	for _, st := range stores {
		snaps = append(snaps, snapFixture("a-"+st.ID, "p1", st.ID, true))
	}
	in := availabilityInputs{snapshots: snaps, stores: stores}
	tests := []struct {
		radius int
		want   []string
	}{
		{50, []string{"esslingen", "stuttgart"}},
		{300, []string{"esslingen", "stuttgart", "munich"}},
		{500, []string{"esslingen", "stuttgart", "munich"}},
		{600, []string{"esslingen", "stuttgart", "munich", "berlin"}},
	}
	for _, tt := range tests {
		resp := assembleAvailability(in, center, tt.radius, "")
		if got := entryIDs(resp.Stores); !equalIDs(got, tt.want) {
			t.Errorf("radius %d: stores = %v, want %v", tt.radius, got, tt.want)
		}
		for _, s := range resp.Stores {
			if s.DistanceKm == nil || *s.DistanceKm > float64(tt.radius) {
				t.Errorf("radius %d: store %s distance %v", tt.radius, s.Store.ID, s.DistanceKm)
			}
		}
		if resp.RadiusKm != tt.radius || resp.Center != center {
			t.Errorf("radius %d: echoed radius=%d center=%+v", tt.radius, resp.RadiusKm, resp.Center)
		}
	}
}

func TestAssemble_RadiusIsInclusive(t *testing.T) {
	center := &Center{PLZ: "x", Lat: 48.74, Lon: 9.31}
	st := domain.Store{ID: "s1", Name: "OBI", Lat: 48.74, Lon: 9.31}
	snaps := []domain.AvailabilitySnapshot{snapFixture("a1", "p1", "s1", true)}
	// Distance 0 is within any positive radius, including the minimum of 1km.
	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: []domain.Store{st}}, center, 1, "")
	if len(resp.Stores) != 1 {
		t.Fatalf("store at the center excluded with radius 1: %+v", resp.Stores)
	}
}

func TestAssemble_OnlineGroupedBySource(t *testing.T) {
	sources := []domain.Source{
		{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline},
		{Key: "otto", Name: "Otto", Type: domain.SourceOnline},
	}
	mk := func(src, product string, inStock bool, price *int) domain.AvailabilitySnapshot {
		sn := snapFixture(src+product, product, "", inStock)
		sn.SourceKey = src
		sn.PriceCents = price
		return sn
	}
	emptyStore := mk("otto", "p2", true, ip(900))
	emptyStore.StoreID = ptr("") // empty store ID counts as online
	snaps := []domain.AvailabilitySnapshot{
		mk("otto", "p1", false, ip(100)),
		mk("amazon", "p1", true, ip(500)),
		mk("zshop", "p1", true, nil), // source not in the source list
		emptyStore,
		mk("amazon", "p2", true, ip(300)),
		snapFixture("store", "p1", "s1", true), // physical: not online
	}
	stores := []domain.Store{{ID: "s1", Name: "OBI"}}

	resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores, sources: sources}, nil, 50, "")
	if len(resp.Stores) != 1 || len(resp.Stores[0].Listings) != 1 {
		t.Fatalf("stores = %+v", resp.Stores)
	}
	if len(resp.Online) != 3 {
		t.Fatalf("online groups = %d, want 3: %+v", len(resp.Online), resp.Online)
	}
	wantKeys := []string{"amazon", "otto", "zshop"}
	for i, g := range resp.Online {
		if g.Source.Key != wantKeys[i] {
			t.Fatalf("online[%d] = %q, want %q", i, g.Source.Key, wantKeys[i])
		}
		for _, l := range g.Listings {
			if l.SourceKey != g.Source.Key {
				t.Errorf("listing of %q in group %q", l.SourceKey, g.Source.Key)
			}
			if l.StoreID != nil && *l.StoreID != "" {
				t.Errorf("physical listing in online group: %+v", l)
			}
		}
	}
	if resp.Online[0].Source.Name != "Amazon" || len(resp.Online[0].Listings) != 2 ||
		*resp.Online[0].Listings[0].PriceCents != 300 {
		t.Errorf("amazon group = %+v", resp.Online[0])
	}
	// otto: in-stock p2 before out-of-stock p1.
	if len(resp.Online[1].Listings) != 2 || !resp.Online[1].Listings[0].InStock || resp.Online[1].Listings[1].InStock {
		t.Errorf("otto group not in-stock first: %+v", resp.Online[1].Listings)
	}
	// Unknown source falls back to an online source named after its key.
	if z := resp.Online[2].Source; z.Name != "zshop" || z.Type != domain.SourceOnline {
		t.Errorf("fallback source = %+v", z)
	}

	t.Run("product filter applies to online", func(t *testing.T) {
		resp := assembleAvailability(availabilityInputs{snapshots: snaps, stores: stores, sources: sources}, nil, 50, "p2")
		if len(resp.Online) != 2 || resp.Online[0].Source.Key != "amazon" || resp.Online[1].Source.Key != "otto" {
			t.Fatalf("online = %+v", resp.Online)
		}
		for _, g := range resp.Online {
			for _, l := range g.Listings {
				if l.ProductID != "p2" {
					t.Errorf("listing for %q leaked through filter", l.ProductID)
				}
			}
		}
		if len(resp.Stores) != 0 {
			t.Errorf("stores = %+v, want none for p2", resp.Stores)
		}
	})
}

func TestSortListings_InStockThenPrice(t *testing.T) {
	ls := []domain.Listing{
		{SourceKey: "c", InStock: false, PriceCents: ip(100)},
		{SourceKey: "b", InStock: true, PriceCents: ip(300)},
		{SourceKey: "a", InStock: true, PriceCents: ip(200)},
	}
	sortListings(ls)
	if !ls[0].InStock || !ls[1].InStock || ls[2].InStock {
		t.Fatalf("in-stock not sorted first: %+v", ls)
	}
	if *ls[0].PriceCents != 200 || *ls[1].PriceCents != 300 {
		t.Fatalf("price ascending broken among in-stock: %+v", ls)
	}
}
