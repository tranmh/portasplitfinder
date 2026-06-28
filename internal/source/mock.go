package source

import (
	"context"
	"os"
	"strconv"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// Deterministic UUIDs for the mock catalogue. They are stable across processes
// so that the API (--seed), the poller, and the e2e tests all agree on the same
// product/store identities.
const (
	MockProductMideaID  = "11111111-1111-1111-1111-111111111111"
	MockProductDaikinID = "22222222-2222-2222-2222-222222222222"

	mockStoreBerlinID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	mockStoreMunichID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	mockStoreCologneID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	mockStoreHamburgID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	mockStoreFrankfurtID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
)

func strptr(s string) *string { return &s }
func intptr(i int) *int       { return &i }

// MockSource produces deterministic seed data covering one store chain (OBI)
// across real German cities plus several online retailers.
//
// # Stock-transition toggle (used by the e2e test)
//
// The Munich OBI store carries the Midea product OUT of stock by default. When
// the toggle is enabled the Munich listing flips to IN stock, letting the poller
// detect an out->in-stock transition and fire an availability email.
//
// Enable the toggle in one of two ways:
//   - construct with NewMockWithTransition(true), or
//   - set the env var MOCK_TRANSITION=1 (read by NewMock()).
//
// Everything else in the mock is fixed, so the transition is the only moving
// part and tests stay deterministic.
type MockSource struct {
	// transitionInStock flips the Munich/Midea store listing to in-stock.
	transitionInStock bool
}

// NewMock builds a mock source, reading the MOCK_TRANSITION env var to decide
// whether the transition listing starts in-stock.
func NewMock() *MockSource {
	on, _ := strconv.ParseBool(os.Getenv("MOCK_TRANSITION"))
	return &MockSource{transitionInStock: on}
}

// NewMockWithTransition builds a mock source with the transition toggle set
// explicitly (used by tests).
func NewMockWithTransition(inStock bool) *MockSource {
	return &MockSource{transitionInStock: inStock}
}

// Key implements Source.
func (m *MockSource) Key() string { return "mock" }

// Meta implements Source. The mock represents an OBI-like store chain.
func (m *MockSource) Meta() domain.Source {
	return domain.Source{
		Key:        "mock",
		Name:       "Mock (OBI + Online)",
		Type:       domain.SourceStore,
		BaseURL:    "https://example.invalid/mock",
		LastStatus: domain.StatusNever,
	}
}

// Products returns the catalogue of products the mock tracks. Used by --seed.
func (m *MockSource) Products() []domain.Product {
	return []domain.Product{
		{
			ID:       MockProductMideaID,
			Name:     "Midea Portasplit MAW-12P1",
			Model:    "MAW-12P1",
			ImageURL: strptr("https://example.invalid/img/midea-maw-12p1.jpg"),
		},
		{
			ID:       MockProductDaikinID,
			Name:     "Daikin Portasplit DPS-09",
			Model:    "DPS-09",
			ImageURL: strptr("https://example.invalid/img/daikin-dps-09.jpg"),
		},
	}
}

// Stores returns the physical OBI stores in the mock catalogue. Used by --seed.
func (m *MockSource) Stores() []domain.Store {
	return []domain.Store{
		{ID: mockStoreBerlinID, SourceKey: "mock", Name: "OBI Markt Berlin-Mitte", Street: strptr("Invalidenstraße 1"), PLZ: "10115", City: "Berlin", Lat: 52.5323, Lon: 13.3846},
		{ID: mockStoreMunichID, SourceKey: "mock", Name: "OBI Markt München-Zentrum", Street: strptr("Bayerstraße 10"), PLZ: "80331", City: "München", Lat: 48.1372, Lon: 11.5755},
		{ID: mockStoreCologneID, SourceKey: "mock", Name: "OBI Markt Köln-Innenstadt", Street: strptr("Domkloster 4"), PLZ: "50667", City: "Köln", Lat: 50.9413, Lon: 6.9583},
		{ID: mockStoreHamburgID, SourceKey: "mock", Name: "OBI Markt Hamburg-Altstadt", Street: strptr("Mönckebergstraße 7"), PLZ: "20095", City: "Hamburg", Lat: 53.5503, Lon: 9.9920},
		{ID: mockStoreFrankfurtID, SourceKey: "mock", Name: "OBI Markt Frankfurt-Zentrum", Street: strptr("Zeil 90"), PLZ: "60311", City: "Frankfurt", Lat: 50.1136, Lon: 8.6797},
	}
}

// onlineSourcesMeta returns the catalogue metadata for the online retailers the
// mock reports listings for. Used by --seed so /api/sources lists them.
func (m *MockSource) onlineSourcesMeta() []domain.Source {
	return []domain.Source{
		{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline, BaseURL: "https://www.amazon.de", LastStatus: domain.StatusNever},
		{Key: "ebay", Name: "eBay", Type: domain.SourceOnline, BaseURL: "https://www.ebay.de", LastStatus: domain.StatusNever},
		{Key: "guenstiger", Name: "guenstiger.de", Type: domain.SourceOnline, BaseURL: "https://www.guenstiger.de", LastStatus: domain.StatusNever},
		{Key: "idealo", Name: "idealo", Type: domain.SourceOnline, BaseURL: "https://www.idealo.de", LastStatus: domain.StatusNever},
	}
}

// SeedSources returns every source the mock catalogue needs in the DB: the mock
// store source itself plus the online retailers it reports.
func (m *MockSource) SeedSources() []domain.Source {
	return append([]domain.Source{m.Meta()}, m.onlineSourcesMeta()...)
}

// Fetch implements Source. checkedAt is left zero here; the poller stamps the
// current time when it writes snapshots, keeping the data deterministic.
func (m *MockSource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	stores := m.Stores()

	storeURL := func(store, product string) string {
		return "https://example.invalid/mock/store/" + store + "/product/" + product
	}
	onlineURL := func(retailer, product string) string {
		return "https://example.invalid/mock/" + retailer + "/product/" + product
	}

	listings := []domain.Listing{
		// --- Physical OBI store listings for the Midea product ---
		{SourceKey: "mock", ProductID: MockProductMideaID, StoreID: strptr(mockStoreBerlinID), InStock: true, Quantity: intptr(4), PriceCents: intptr(49900), Currency: "EUR", URL: storeURL(mockStoreBerlinID, MockProductMideaID)},
		// Munich is the TRANSITION listing: out of stock unless the toggle is on.
		{SourceKey: "mock", ProductID: MockProductMideaID, StoreID: strptr(mockStoreMunichID), InStock: m.transitionInStock, Quantity: mockQty(m.transitionInStock, 3), PriceCents: intptr(50900), Currency: "EUR", URL: storeURL(mockStoreMunichID, MockProductMideaID)},
		{SourceKey: "mock", ProductID: MockProductMideaID, StoreID: strptr(mockStoreCologneID), InStock: false, Quantity: intptr(0), PriceCents: intptr(48900), Currency: "EUR", URL: storeURL(mockStoreCologneID, MockProductMideaID)},
		{SourceKey: "mock", ProductID: MockProductMideaID, StoreID: strptr(mockStoreHamburgID), InStock: true, Quantity: intptr(1), PriceCents: intptr(50500), Currency: "EUR", URL: storeURL(mockStoreHamburgID, MockProductMideaID)},
		{SourceKey: "mock", ProductID: MockProductMideaID, StoreID: strptr(mockStoreFrankfurtID), InStock: false, Quantity: intptr(0), PriceCents: nil, Currency: "EUR", URL: storeURL(mockStoreFrankfurtID, MockProductMideaID)},

		// --- Physical OBI store listings for the Daikin product ---
		{SourceKey: "mock", ProductID: MockProductDaikinID, StoreID: strptr(mockStoreBerlinID), InStock: false, Quantity: intptr(0), PriceCents: intptr(57900), Currency: "EUR", URL: storeURL(mockStoreBerlinID, MockProductDaikinID)},
		{SourceKey: "mock", ProductID: MockProductDaikinID, StoreID: strptr(mockStoreCologneID), InStock: true, Quantity: intptr(2), PriceCents: intptr(56900), Currency: "EUR", URL: storeURL(mockStoreCologneID, MockProductDaikinID)},

		// --- Online listings: Midea ---
		{SourceKey: "amazon", ProductID: MockProductMideaID, StoreID: nil, InStock: true, Quantity: nil, PriceCents: intptr(47900), Currency: "EUR", URL: onlineURL("amazon", MockProductMideaID)},
		{SourceKey: "ebay", ProductID: MockProductMideaID, StoreID: nil, InStock: true, Quantity: intptr(7), PriceCents: intptr(46900), Currency: "EUR", URL: onlineURL("ebay", MockProductMideaID)},
		{SourceKey: "guenstiger", ProductID: MockProductMideaID, StoreID: nil, InStock: false, Quantity: nil, PriceCents: intptr(45900), Currency: "EUR", URL: onlineURL("guenstiger", MockProductMideaID)},
		{SourceKey: "idealo", ProductID: MockProductMideaID, StoreID: nil, InStock: true, Quantity: nil, PriceCents: intptr(46500), Currency: "EUR", URL: onlineURL("idealo", MockProductMideaID)},

		// --- Online listings: Daikin ---
		{SourceKey: "amazon", ProductID: MockProductDaikinID, StoreID: nil, InStock: true, Quantity: nil, PriceCents: intptr(55900), Currency: "EUR", URL: onlineURL("amazon", MockProductDaikinID)},
		{SourceKey: "idealo", ProductID: MockProductDaikinID, StoreID: nil, InStock: false, Quantity: nil, PriceCents: nil, Currency: "EUR", URL: onlineURL("idealo", MockProductDaikinID)},
	}

	return listings, stores, nil
}

func mockQty(inStock bool, q int) *int {
	if inStock {
		return intptr(q)
	}
	return intptr(0)
}
