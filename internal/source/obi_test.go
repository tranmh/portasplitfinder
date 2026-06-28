package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

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

// directoryJSON is a small OBI market directory covering the placement cases:
// own coordinates, zero coordinates with a known PLZ (geo fallback), zero
// coordinates with an unknown PLZ (skipped), and an inactive market (skipped).
const directoryJSON = `{"stores":[
 {"storeNumber":"003","name":"Olpe","street":"In der Trift 17","zip":"57462","city":"Olpe","lat":51.0222,"lon":7.8434,"isActive":true,"url":"https://x/markt/olpe"},
 {"storeNumber":"005","name":"Herford","street":"A 2","zip":"32049","city":"Herford","lat":52.1,"lon":8.6,"isActive":true},
 {"storeNumber":"100","name":"Failville","street":"B 3","zip":"12345","city":"Failville","lat":50.0,"lon":10.0,"isActive":true},
 {"storeNumber":"200","name":"Berlin-Mitte","street":"Invalidenstr. 1","zip":"10115","city":"Berlin","lat":0,"lon":0,"isActive":true},
 {"storeNumber":"300","name":"Nowhere","street":"C 4","zip":"00000","city":"Nowhere","lat":0,"lon":0,"isActive":true},
 {"storeNumber":"400","name":"Closed","street":"D 5","zip":"99999","city":"Closed","lat":48.0,"lon":11.0,"isActive":false}
]}`

// availabilityJSON returns a store-specific-articles response for the given
// stock/price (price 0 => omitted).
func availabilityJSON(stock int) string {
	return fmt.Sprintf(`[{"articleNumber":"8620890","currency":{"isoCode":"EUR","symbol":"€"},"name":"Midea PortaSplit","storeSpecificArticle":{"price":799.99,"stock":%d}}]`, stock)
}

// obiTestHandler serves the directory and per-market availability endpoints.
// Store 100 returns 404 (a per-market failure); stocks: 003 -> 2, 005 -> 0,
// 200 -> 1.
func obiTestHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	stocks := map[string]int{"003": 2, "005": 0, "200": 1}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/disc/store/locator/country/DE":
			_, _ = w.Write([]byte(directoryJSON))
		case strings.Contains(r.URL.Path, "/store-specific-articles/v2/"):
			num := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if num == "100" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(availabilityJSON(stocks[num])))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestOBI_Meta(t *testing.T) {
	m := newOBISource().Meta()
	if m.Key != "obi" || m.Name != "OBI" || m.Type != domain.SourceStore {
		t.Fatalf("unexpected meta: %+v", m)
	}
	if m.LastStatus != domain.StatusNever {
		t.Fatalf("LastStatus = %q", m.LastStatus)
	}
}

func TestOBI_PerMarketAvailability(t *testing.T) {
	srv := httptest.NewServer(obiTestHandler(t))
	defer srv.Close()

	s := newTestOBI(srv)
	listings, stores, err := s.Fetch(context.Background(), []domain.Product{mideaProduct})
	if err != nil {
		t.Fatal(err)
	}

	// Placeable markets: 003, 005, 100, 200 (300 unknown PLZ -> skipped,
	// 400 inactive -> skipped).
	if len(stores) != 4 {
		t.Fatalf("got %d stores, want 4", len(stores))
	}
	// Store 200 has zero coords in the directory; it should be geocoded from PLZ.
	for _, st := range stores {
		if st.SourceKey != "obi" {
			t.Errorf("store %q SourceKey = %q", st.Name, st.SourceKey)
		}
		if st.Lat == 0 || st.Lon == 0 {
			t.Errorf("store %q has zero coordinates", st.Name)
		}
		if !strings.HasPrefix(st.Name, "OBI ") {
			t.Errorf("store name not prefixed: %q", st.Name)
		}
	}

	// Listings: 003, 005, 200 (100 failed with 404 -> skipped).
	if len(listings) != 3 {
		t.Fatalf("got %d listings, want 3", len(listings))
	}

	byStore := map[string]domain.Listing{}
	for _, l := range listings {
		if l.SourceKey != "obi" || l.ProductID != mideaProduct.ID {
			t.Errorf("unexpected listing: %+v", l)
		}
		if l.StoreID == nil {
			t.Fatalf("listing has nil StoreID: %+v", l)
		}
		byStore[*l.StoreID] = l
	}

	// In-stock market 003: stock 2, price 79999 cents.
	l003 := byStore[obiStoreID("003")]
	if !l003.InStock {
		t.Error("store 003 InStock = false, want true")
	}
	if l003.Quantity == nil || *l003.Quantity != 2 {
		t.Errorf("store 003 Quantity = %v, want 2", l003.Quantity)
	}
	if l003.PriceCents == nil || *l003.PriceCents != 79999 {
		t.Errorf("store 003 PriceCents = %v, want 79999", l003.PriceCents)
	}
	if l003.Currency != "EUR" {
		t.Errorf("store 003 Currency = %q", l003.Currency)
	}

	// Out-of-stock market 005: stock 0.
	l005 := byStore[obiStoreID("005")]
	if l005.InStock {
		t.Error("store 005 InStock = true, want false")
	}
	if l005.Quantity == nil || *l005.Quantity != 0 {
		t.Errorf("store 005 Quantity = %v, want 0", l005.Quantity)
	}

	// Every listing's StoreID must reference a returned store.
	storeIDs := map[string]bool{}
	for _, st := range stores {
		storeIDs[st.ID] = true
	}
	for _, l := range listings {
		if !storeIDs[*l.StoreID] {
			t.Errorf("listing StoreID %q has no matching store", *l.StoreID)
		}
	}
}

func TestOBI_DirectoryFailure_IsSourceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // OBI blocks the directory
	}))
	defer srv.Close()

	_, _, err := newTestOBI(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err == nil {
		t.Fatal("expected a source-level error when the directory fails")
	}
}

func TestOBI_AllMarketsFail_IsSourceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/disc/store/locator/country/DE" {
			_, _ = w.Write([]byte(directoryJSON))
			return
		}
		w.WriteHeader(http.StatusNotFound) // every availability lookup fails
	}))
	defer srv.Close()

	listings, _, err := newTestOBI(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err == nil {
		t.Fatal("expected a source-level error when every market lookup fails")
	}
	if len(listings) != 0 {
		t.Fatalf("got %d listings, want 0", len(listings))
	}
}

func TestOBI_EmptyProducts_NoError(t *testing.T) {
	listings, stores, err := newOBISource().Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty products should not error, got %v", err)
	}
	if len(listings) != 0 || stores != nil {
		t.Fatalf("expected no listings/stores, got %v / %v", listings, stores)
	}
}

func TestOBI_UnmappableProduct_NoError(t *testing.T) {
	// An unknown product whose search finds no OBI link is skipped (no error),
	// and with no mappable products Fetch never touches the directory.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			_, _ = w.Write([]byte("<html>no products here</html>"))
			return
		}
		t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	unknown := domain.Product{ID: "x", Name: "Nonexistent Gadget 5000", Model: "NG-5000"}
	listings, stores, err := newTestOBI(srv).Fetch(context.Background(), []domain.Product{unknown})
	if err != nil {
		t.Fatalf("unmappable product should not error, got %v", err)
	}
	if len(listings) != 0 || stores != nil {
		t.Fatalf("expected no listings/stores, got %v / %v", listings, stores)
	}
}

func TestOBI_SearchFallback_ResolvesArticle(t *testing.T) {
	var sawSearch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/"):
			sawSearch = true
			_, _ = w.Write([]byte(`<a href="/p/9999004/klimageraet-xy">Klimagerät</a>`))
		case r.URL.Path == "/api/disc/store/locator/country/DE":
			_, _ = w.Write([]byte(directoryJSON))
		case strings.Contains(r.URL.Path, "/store-specific-articles/v2/"):
			// The resolved article (9999004) is queried for each market.
			if !strings.Contains(r.URL.RawQuery, "9999004") {
				t.Errorf("availability query = %q, want article 9999004", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"articleNumber":"9999004","currency":{"isoCode":"EUR","symbol":"€"},"storeSpecificArticle":{"price":399.99,"stock":5}}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	unknown := domain.Product{ID: "p-search", Name: "Klimageraet", Model: ""}
	listings, _, err := newTestOBI(srv).Fetch(context.Background(), []domain.Product{unknown})
	if err != nil {
		t.Fatal(err)
	}
	if !sawSearch {
		t.Fatal("expected a search request")
	}
	if len(listings) == 0 {
		t.Fatal("expected listings from the resolved article")
	}
	l := listings[0]
	if l.ProductID != "p-search" {
		t.Errorf("ProductID = %q", l.ProductID)
	}
	if !strings.Contains(l.URL, "/p/9999004/") {
		t.Errorf("listing URL = %q, want the resolved product page", l.URL)
	}
	if l.PriceCents == nil || *l.PriceCents != 39999 {
		t.Errorf("PriceCents = %v, want 39999", l.PriceCents)
	}
}

func TestOBI_StoreID_Deterministic(t *testing.T) {
	if a, b := obiStoreID("003"), obiStoreID("003"); a != b {
		t.Fatalf("store id not deterministic: %q != %q", a, b)
	}
	if obiStoreID("003") == obiStoreID("005") {
		t.Fatal("distinct markets share a store id")
	}
}
