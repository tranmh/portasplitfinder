package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// newTestOBI builds an obiSource pointed at srv with a fast no-sleep client.
func newTestOBI(srv *httptest.Server) *obiSource {
	return &obiSource{
		client:  NewClient(WithBaseBackoff(0), WithHTTPClient(srv.Client())),
		baseURL: srv.URL,
	}
}

// mideaProduct is a product that matches the known-article mapping (article
// 8620890).
var mideaProduct = domain.Product{
	ID:    "11111111-1111-1111-1111-111111111111",
	Name:  "Midea Portasplit MAW-12P1",
	Model: "MAW-12P1",
}

func TestOBI_Meta(t *testing.T) {
	s := newOBISource()
	m := s.Meta()
	if m.Key != "obi" || m.Name != "OBI" || m.Type != domain.SourceOnline {
		t.Fatalf("unexpected meta: %+v", m)
	}
	if m.LastStatus != domain.StatusNever {
		t.Fatalf("LastStatus = %q", m.LastStatus)
	}
}

func TestOBI_InStock_KnownMapping(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write(readFixture(t, "obi_instock.html"))
	}))
	defer srv.Close()

	s := newTestOBI(srv)
	listings, stores, err := s.Fetch(context.Background(), []domain.Product{mideaProduct})
	if err != nil {
		t.Fatal(err)
	}
	if stores != nil {
		t.Fatalf("expected nil stores, got %v", stores)
	}
	if len(listings) != 1 {
		t.Fatalf("got %d listings, want 1", len(listings))
	}
	// Known mapping should resolve to the article page without a search request.
	if !strings.HasPrefix(gotPath, "/p/8620890/") {
		t.Fatalf("known mapping path = %q, want /p/8620890/...", gotPath)
	}

	l := listings[0]
	if l.SourceKey != "obi" {
		t.Errorf("SourceKey = %q", l.SourceKey)
	}
	if l.ProductID != mideaProduct.ID {
		t.Errorf("ProductID = %q, want %q", l.ProductID, mideaProduct.ID)
	}
	if l.StoreID != nil {
		t.Errorf("StoreID = %v, want nil", l.StoreID)
	}
	if !l.InStock {
		t.Errorf("InStock = false, want true (InStoreOnly counts as available)")
	}
	if l.Currency != "EUR" {
		t.Errorf("Currency = %q", l.Currency)
	}
	// 799.99 EUR -> 79999 cents
	if l.PriceCents == nil || *l.PriceCents != 79999 {
		t.Errorf("PriceCents = %v, want 79999", l.PriceCents)
	}
	if !strings.Contains(l.URL, "/p/8620890/") {
		t.Errorf("URL = %q, want OBI article URL", l.URL)
	}
}

func TestOBI_OutOfStock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(readFixture(t, "obi_outofstock.html"))
	}))
	defer srv.Close()

	l := fetchOne(t, newTestOBI(srv), mideaProduct)
	if l.InStock {
		t.Error("InStock = true, want false")
	}
	if l.PriceCents == nil || *l.PriceCents != 64900 {
		t.Errorf("PriceCents = %v, want 64900", l.PriceCents)
	}
}

func TestOBI_Graph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(readFixture(t, "obi_graph.html"))
	}))
	defer srv.Close()

	l := fetchOne(t, newTestOBI(srv), mideaProduct)
	if !l.InStock {
		t.Error("InStock = false, want true")
	}
	if l.PriceCents == nil || *l.PriceCents != 72950 {
		t.Errorf("PriceCents = %v, want 72950", l.PriceCents)
	}
	if l.Currency != "EUR" {
		t.Errorf("Currency = %q", l.Currency)
	}
}

func TestOBI_AggregateOffer_LowPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(readFixture(t, "obi_aggregate.html"))
	}))
	defer srv.Close()

	l := fetchOne(t, newTestOBI(srv), mideaProduct)
	if !l.InStock {
		t.Error("InStock = false, want true")
	}
	// lowPrice 559.99 -> 55999 cents
	if l.PriceCents == nil || *l.PriceCents != 55999 {
		t.Errorf("PriceCents = %v, want 55999 (lowPrice)", l.PriceCents)
	}
}

func TestOBI_Blocked_SkipsProduct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // OBI blocks -> 404
	}))
	defer srv.Close()

	s := newTestOBI(srv)
	listings, _, err := s.Fetch(context.Background(), []domain.Product{mideaProduct})
	// Single product, it failed -> source-level error returned.
	if err == nil {
		t.Fatal("expected source-level error when all products fail")
	}
	if len(listings) != 0 {
		t.Fatalf("got %d listings, want 0", len(listings))
	}
}

func TestOBI_PartialFailure_KeepsGoing(t *testing.T) {
	// Known product (mapped path) succeeds; an unknown product's search 404s.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/p/8620890/") {
			_, _ = w.Write(readFixture(t, "obi_instock.html"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	unknown := domain.Product{ID: "x", Name: "Nonexistent Gadget 5000", Model: "NG-5000"}
	s := newTestOBI(srv)
	listings, _, err := s.Fetch(context.Background(), []domain.Product{mideaProduct, unknown})
	if err != nil {
		t.Fatalf("partial failure should not be a source error, got %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("got %d listings, want 1 (the known product)", len(listings))
	}
	if listings[0].ProductID != mideaProduct.ID {
		t.Errorf("kept wrong product: %q", listings[0].ProductID)
	}
}

func TestOBI_SearchFallback(t *testing.T) {
	var sawSearch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/"):
			sawSearch = true
			_, _ = w.Write(readFixture(t, "obi_search.html"))
		case strings.HasPrefix(r.URL.Path, "/p/9999004/"):
			_, _ = w.Write(readFixture(t, "obi_search_product.html"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Unknown product -> search fallback -> first /p/ link -> product page.
	unknown := domain.Product{ID: "p-search", Name: "Klimageraet", Model: ""}
	l := fetchOne(t, newTestOBI(srv), unknown)
	if !sawSearch {
		t.Fatal("expected a search request")
	}
	if l.ProductID != "p-search" {
		t.Errorf("ProductID = %q", l.ProductID)
	}
	if !strings.Contains(l.URL, "/p/9999004/") {
		t.Errorf("resolved URL = %q, want the first search result", l.URL)
	}
	if l.PriceCents == nil || *l.PriceCents != 39999 {
		t.Errorf("PriceCents = %v, want 39999", l.PriceCents)
	}
}

func TestOBI_EmptyProducts_NoError(t *testing.T) {
	s := newOBISource()
	listings, stores, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty products should not error, got %v", err)
	}
	if len(listings) != 0 || stores != nil {
		t.Fatalf("expected no listings/stores, got %v / %v", listings, stores)
	}
}

func TestAvailabilityInStock(t *testing.T) {
	cases := map[string]bool{
		"http://schema.org/InStock":             true,
		"https://schema.org/InStock":            true,
		"http://schema.org/InStoreOnly":         true,
		"https://schema.org/OnlineOnly":         true,
		"https://schema.org/PreOrder":           true,
		"https://schema.org/LimitedAvailability": true,
		"http://schema.org/OutOfStock":          false,
		"https://schema.org/SoldOut":            false,
		"https://schema.org/Discontinued":       false,
		"":                                      false,
	}
	for in, want := range cases {
		if got := availabilityInStock(in); got != want {
			t.Errorf("availabilityInStock(%q) = %v, want %v", in, got, want)
		}
	}
}

// fetchOne fetches a single product and asserts exactly one listing.
func fetchOne(t *testing.T, s *obiSource, p domain.Product) domain.Listing {
	t.Helper()
	listings, _, err := s.Fetch(context.Background(), []domain.Product{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("got %d listings, want 1", len(listings))
	}
	return listings[0]
}
