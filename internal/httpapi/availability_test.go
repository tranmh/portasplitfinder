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
