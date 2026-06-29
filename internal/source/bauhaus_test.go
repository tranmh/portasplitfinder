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

// newTestBauhaus builds a bauhausSource pointed at srv with a fast no-sleep client.
func newTestBauhaus(srv *httptest.Server) *bauhausSource {
	return &bauhausSource{
		client:  NewClient(WithBaseBackoff(0), WithHTTPClient(srv.Client())),
		baseURL: srv.URL,
	}
}

// bauhausDirectoryJSON is a small BAUHAUS store directory covering the placement
// cases, with all placeable markets inside the 50 km anchor radius (PLZ 73035):
// geolocation coordinates; zero coordinates with a known nearby PLZ (geo
// fallback); zero coordinates with an unknown PLZ (skipped). The store with no
// "active" field is treated as active.
const bauhausDirectoryJSON = `{"stores":[
 {"storeId":"003","name":"Göppingen","address":{"street":"In der Trift 17","zip":"73035","city":"Göppingen"},"geolocation":{"latitude":48.7000,"longitude":9.6667}},
 {"storeId":"005","name":"Esslingen","address":{"street":"A 2","zip":"73728","city":"Esslingen"},"geolocation":{"latitude":48.7394,"longitude":9.3061}},
 {"storeId":"200","name":"Kirchheim","street":"Invalidenstr. 1","zip":"73230","city":"Kirchheim","geolocation":{"latitude":0,"longitude":0}},
 {"storeId":"300","name":"Nowhere","address":{"street":"C 4","zip":"00000","city":"Nowhere"},"geolocation":{"latitude":0,"longitude":0}}
]}`

// bauhausAvailabilityJSON returns an availability-detail response whose
// reservation branch carries the given state (and a price).
func bauhausAvailabilityJSON(state string) string {
	return fmt.Sprintf(`{"availabilityMap":{"reservation":{"state":%q,"price":799.99},"online-order":{"state":"NOT_AVAILABLE"}}}`, state)
}

// bauhausTestHandler serves the directory and per-store availability endpoints.
// Store 200 returns 404 (a per-store failure); states: 003 -> AVAILABLE (in
// stock), 005 -> NOT_AVAILABLE (out of stock).
func bauhausTestHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	states := map[string]string{"003": "AVAILABLE", "005": "NOT_AVAILABLE"}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/stores-search":
			_, _ = w.Write([]byte(bauhausDirectoryJSON))
		case strings.Contains(r.URL.Path, "/api/product-availability/availability-detail/"):
			id := r.URL.Query().Get("storeId")
			if id == "200" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(bauhausAvailabilityJSON(states[id])))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestBauhaus_Meta(t *testing.T) {
	m := newBauhausSource().Meta()
	if m.Key != "bauhaus" || m.Name != "BAUHAUS" || m.Type != domain.SourceStore {
		t.Fatalf("unexpected meta: %+v", m)
	}
	if m.BaseURL != "https://www.bauhaus.info" {
		t.Fatalf("BaseURL = %q", m.BaseURL)
	}
	if m.LastStatus != domain.StatusNever {
		t.Fatalf("LastStatus = %q", m.LastStatus)
	}
}

func TestBauhaus_PerStoreAvailability(t *testing.T) {
	srv := httptest.NewServer(bauhausTestHandler(t))
	defer srv.Close()

	s := newTestBauhaus(srv)
	listings, stores, err := s.Fetch(context.Background(), []domain.Product{mideaProduct})
	if err != nil {
		t.Fatal(err)
	}

	// Placeable stores: 003, 005, 200 (300 has zero coords + unknown PLZ -> skipped).
	if len(stores) != 3 {
		t.Fatalf("got %d stores, want 3", len(stores))
	}
	// Store 200 has zero coords in the directory; it should be geocoded from PLZ.
	for _, st := range stores {
		if st.SourceKey != "bauhaus" {
			t.Errorf("store %q SourceKey = %q", st.Name, st.SourceKey)
		}
		if st.Lat == 0 || st.Lon == 0 {
			t.Errorf("store %q has zero coordinates", st.Name)
		}
		if !strings.HasPrefix(st.Name, "BAUHAUS ") {
			t.Errorf("store name not prefixed: %q", st.Name)
		}
	}

	// Listings: 003, 005 (200 failed with 404 -> skipped).
	if len(listings) != 2 {
		t.Fatalf("got %d listings, want 2", len(listings))
	}

	byStore := map[string]domain.Listing{}
	for _, l := range listings {
		if l.SourceKey != "bauhaus" || l.ProductID != mideaProduct.ID {
			t.Errorf("unexpected listing: %+v", l)
		}
		if l.StoreID == nil {
			t.Fatalf("listing has nil StoreID: %+v", l)
		}
		byStore[*l.StoreID] = l
	}

	// In-stock store 003: AVAILABLE, price 79999 cents, no quantity.
	l003 := byStore[bauhausStoreID("003")]
	if !l003.InStock {
		t.Error("store 003 InStock = false, want true")
	}
	if l003.Quantity != nil {
		t.Errorf("store 003 Quantity = %v, want nil", l003.Quantity)
	}
	if l003.PriceCents == nil || *l003.PriceCents != 79999 {
		t.Errorf("store 003 PriceCents = %v, want 79999", l003.PriceCents)
	}
	if l003.Currency != "EUR" {
		t.Errorf("store 003 Currency = %q", l003.Currency)
	}

	// Out-of-stock store 005: NOT_AVAILABLE.
	l005 := byStore[bauhausStoreID("005")]
	if l005.InStock {
		t.Error("store 005 InStock = true, want false")
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

// TestBauhaus_BareArrayDirectory confirms the defensive decoder accepts a bare
// top-level array (no {"stores":...} wrapper).
func TestBauhaus_BareArrayDirectory(t *testing.T) {
	const bareJSON = `[{"storeId":"003","name":"Göppingen","address":{"street":"x","zip":"73035","city":"Göppingen"},"geolocation":{"latitude":48.7000,"longitude":9.6667}}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/stores-search":
			_, _ = w.Write([]byte(bareJSON))
		case strings.Contains(r.URL.Path, "/availability-detail/"):
			_, _ = w.Write([]byte(bauhausAvailabilityJSON("AVAILABLE")))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	listings, stores, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err != nil {
		t.Fatal(err)
	}
	if len(stores) != 1 || len(listings) != 1 {
		t.Fatalf("got %d stores / %d listings, want 1 / 1", len(stores), len(listings))
	}
}

// TestBauhaus_RadiusFilter confirms Fachcentren outside the 50 km anchor radius
// (PLZ 73035) are dropped: a near Göppingen store is kept, a far Berlin store is
// filtered out.
func TestBauhaus_RadiusFilter(t *testing.T) {
	const dir = `{"stores":[
 {"storeId":"003","name":"Göppingen","address":{"zip":"73035","city":"Göppingen"},"geolocation":{"latitude":48.7000,"longitude":9.6667}},
 {"storeId":"999","name":"Berlin","address":{"zip":"10115","city":"Berlin"},"geolocation":{"latitude":52.5323,"longitude":13.3846}}
]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/stores-search":
			_, _ = w.Write([]byte(dir))
		case strings.Contains(r.URL.Path, "/availability-detail/"):
			_, _ = w.Write([]byte(bauhausAvailabilityJSON("AVAILABLE")))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	_, stores, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err != nil {
		t.Fatal(err)
	}
	if len(stores) != 1 {
		t.Fatalf("got %d stores, want 1 (only the in-radius market)", len(stores))
	}
	if stores[0].City != "Göppingen" {
		t.Fatalf("kept the wrong store: %+v", stores[0])
	}
}

func TestBauhaus_DirectoryFailure_IsSourceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // Cloudflare blocks the directory
	}))
	defer srv.Close()

	_, _, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err == nil {
		t.Fatal("expected a source-level error when the directory fails")
	}
}

func TestBauhaus_AllStoresFail_IsSourceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stores-search" {
			_, _ = w.Write([]byte(bauhausDirectoryJSON))
			return
		}
		w.WriteHeader(http.StatusForbidden) // every availability lookup is blocked
	}))
	defer srv.Close()

	listings, _, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{mideaProduct})
	if err == nil {
		t.Fatal("expected a source-level error when every store lookup fails")
	}
	if len(listings) != 0 {
		t.Fatalf("got %d listings, want 0", len(listings))
	}
}

func TestBauhaus_EmptyProducts_NoError(t *testing.T) {
	listings, stores, err := newBauhausSource().Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty products should not error, got %v", err)
	}
	if len(listings) != 0 || stores != nil {
		t.Fatalf("expected no listings/stores, got %v / %v", listings, stores)
	}
}

func TestBauhaus_UnmappableProduct_NoError(t *testing.T) {
	// An unknown product whose search finds no BAUHAUS link is skipped (no error),
	// and with no mappable products Fetch never touches the directory.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search") {
			_, _ = w.Write([]byte("<html>no products here</html>"))
			return
		}
		t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	unknown := domain.Product{ID: "x", Name: "Nonexistent Gadget 5000", Model: "NG-5000"}
	listings, stores, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{unknown})
	if err != nil {
		t.Fatalf("unmappable product should not error, got %v", err)
	}
	if len(listings) != 0 || stores != nil {
		t.Fatalf("expected no listings/stores, got %v / %v", listings, stores)
	}
}

func TestBauhaus_SearchFallback_ResolvesArticle(t *testing.T) {
	var sawSearch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			sawSearch = true
			_, _ = w.Write([]byte(`<a href="/p/9999004">Klimagerät</a>`))
		case r.URL.Path == "/api/stores-search":
			_, _ = w.Write([]byte(bauhausDirectoryJSON))
		case strings.Contains(r.URL.Path, "/availability-detail/"):
			// The resolved article (9999004) is queried for each store.
			if !strings.Contains(r.URL.Path, "9999004") {
				t.Errorf("availability path = %q, want article 9999004", r.URL.Path)
			}
			_, _ = w.Write([]byte(bauhausAvailabilityJSON("AVAILABLE")))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	unknown := domain.Product{ID: "p-search", Name: "Klimageraet", Model: ""}
	listings, _, err := newTestBauhaus(srv).Fetch(context.Background(), []domain.Product{unknown})
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
	if !strings.Contains(l.URL, "/p/9999004") {
		t.Errorf("listing URL = %q, want the resolved product page", l.URL)
	}
}

func TestBauhaus_StoreID_Deterministic(t *testing.T) {
	if a, b := bauhausStoreID("003"), bauhausStoreID("003"); a != b {
		t.Fatalf("store id not deterministic: %q != %q", a, b)
	}
	if bauhausStoreID("003") == bauhausStoreID("005") {
		t.Fatal("distinct stores share a store id")
	}
	// BAUHAUS store ids must differ from OBI's for the same store number.
	if bauhausStoreID("003") == obiStoreID("003") {
		t.Fatal("bauhaus and obi share a store id for the same store number")
	}
}
