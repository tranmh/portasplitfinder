package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
)

// bauhausSource reports per-market (store-level) availability of the tracked
// products across all German BAUHAUS Fachcentren.
//
// Like OBI, BAUHAUS offers the tracked products as reserve & collect in physical
// markets, so it is a store source: Fetch returns one domain.Store per BAUHAUS
// Fachcentrum (with lat/lon) and one listing per market keyed by that store,
// producing real map pins.
//
// It uses two undocumented JSON endpoints that bauhaus.info's own frontend calls:
//
//   - store directory (all German Fachcentren, with coordinates):
//     GET /api/stores-search
//     → {"stores":[{"storeId":"520","name":"...","geolocation":{"latitude":...,"longitude":...},"address":{...}}]}
//     (a bare top-level array is also accepted defensively)
//   - per-store availability for a product:
//     GET /api/product-availability/availability-detail/{sku}?storeId={storeId}&searchRadius=100
//     → {"availabilityMap":{"reservation":{"state":"AVAILABLE","price":799.99}, "online-order":{...}}}
//
// BAUHAUS is behind Cloudflare Bot Management and returns HTTP 403 to plain Go
// HTTP clients even with a browser User-Agent. The shared httpx Client maps
// 403/404/429 → ErrBlocked, and Fetch degrades gracefully exactly like OBI: a
// per-store availability failure skips that market's listing, and a source-level
// error is returned only if the directory itself fails or every store lookup
// fails. Live data flows once a Cloudflare bypass is added (deferred).
type bauhausSource struct {
	client  *Client
	baseURL string
}

// bauhausStoreNamespace seeds deterministic (UUIDv5) store ids derived from a
// BAUHAUS store number, so a market keeps the same store id across polls and the
// in-Fetch listing can reference its store before the poller upserts it. It is a
// distinct namespace from OBI's so the two chains never collide on a store id.
var bauhausStoreNamespace = uuid.MustParse("89ed741d-e70e-4c32-b344-3e86db759afe")

// bauhausAvailabilityWorkers bounds concurrent per-store availability requests.
// Kept low (2): BAUHAUS is fetched via a web-unblocker that rotates IPs, so high
// concurrency buys nothing and is impolite; the radius filter keeps the store
// count small anyway.
const bauhausAvailabilityWorkers = 2

// bauhausAnchorPLZ / bauhausRadiusKm scope BAUHAUS to Fachcentren within
// bauhausRadiusKm of the anchor postal code, keeping the per-poll request count
// (and thus web-unblocker cost) small. Markets outside the radius are skipped.
const (
	bauhausAnchorPLZ = "73035"
	bauhausRadiusKm  = 50.0
)

// bauhausInStockStates is the set of reservation states (case-insensitive) that
// count as in stock for reserve & collect.
var bauhausInStockStates = map[string]bool{
	"available":        true,
	"back_in_stock":    true,
	"ready_for_pickup": true,
}

// newBauhausSource builds a bauhausSource with default browser-like client and
// base URL. Tests construct bauhausSource directly to point at an
// httptest.Server.
func newBauhausSource() *bauhausSource {
	return &bauhausSource{
		client:  NewClient(WithUnblocker(unblockerTemplate)),
		baseURL: "https://www.bauhaus.info",
	}
}

func (s *bauhausSource) base() string {
	if s.baseURL == "" {
		return "https://www.bauhaus.info"
	}
	return strings.TrimRight(s.baseURL, "/")
}

func (s *bauhausSource) httpClient() *Client {
	if s.client == nil {
		s.client = NewClient()
	}
	return s.client
}

// Key implements Source.
func (s *bauhausSource) Key() string { return "bauhaus" }

// Meta implements Source. BAUHAUS is a physical store chain: per-market reserve &
// collect availability renders as map pins.
func (s *bauhausSource) Meta() domain.Source {
	return domain.Source{
		Key:        "bauhaus",
		Name:       "BAUHAUS",
		Type:       domain.SourceStore,
		BaseURL:    "https://www.bauhaus.info",
		LastStatus: domain.StatusNever,
	}
}

// knownBauhausArticles maps a recognised product to its BAUHAUS SKU. Entries are
// tried in order, so the more specific matcher (the Cool 8.000 BTU variant) is
// listed first; the 12.000 BTU PortaSplit is the default for the generic match.
var knownBauhausArticles = []struct {
	// match reports whether a product is this known item.
	match func(p domain.Product) bool
	sku   string
}{
	{
		// PortaSplit Cool 8.000 BTU — more specific, so listed first.
		match: func(p domain.Product) bool {
			name := strings.ToLower(p.Name + " " + p.Model)
			isPorta := strings.Contains(name, "midea") &&
				(strings.Contains(name, "portasplit") || strings.Contains(name, "porta split"))
			return isPorta &&
				(strings.Contains(name, "8000") || strings.Contains(name, "8.000") || strings.Contains(name, "cool"))
		},
		sku: "33946696",
	},
	{
		// PortaSplit (12.000 BTU) — the primary tracked product, default mapping.
		match: func(p domain.Product) bool {
			name := strings.ToLower(p.Name + " " + p.Model)
			return strings.Contains(name, "midea") &&
				(strings.Contains(name, "portasplit") || strings.Contains(name, "porta split"))
		},
		sku: "31934233",
	},
}

// Fetch implements Source. It resolves each product to a BAUHAUS SKU, fetches the
// German store directory once, then queries per-store stock for every (product,
// store) pair and emits a store-keyed listing per market.
func (s *bauhausSource) Fetch(ctx context.Context, products []domain.Product) ([]domain.Listing, []domain.Store, error) {
	// Resolve the tracked products that have a BAUHAUS SKU; skip the rest.
	type tracked struct {
		product domain.Product
		sku     string
		pageURL string
	}
	var items []tracked
	for _, p := range products {
		sku, pageURL, err := s.resolveArticle(ctx, p)
		if err != nil {
			continue
		}
		items = append(items, tracked{product: p, sku: sku, pageURL: pageURL})
	}
	if len(items) == 0 {
		// No tracked product maps to BAUHAUS: nothing to do, but not an error.
		return nil, nil, nil
	}

	markets, err := s.fetchMarkets(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("bauhaus: store directory: %w", err)
	}

	// Anchor for the radius filter: only Fachcentren within bauhausRadiusKm of
	// this point are kept. If the anchor PLZ can't be resolved, skip filtering.
	anchor, anchorOK := geo.Lookup(bauhausAnchorPLZ)

	// Build the placeable markets (those we can show on the map) and their stores.
	type placed struct {
		storeNo string
		storeID string
	}
	var stores []domain.Store
	var places []placed
	for _, m := range markets {
		if !m.active() || m.StoreID == "" {
			continue
		}
		lat, lon := m.lat(), m.lon()
		if lat == 0 && lon == 0 {
			pt, ok := geo.Lookup(m.zip())
			if !ok {
				// Cannot place the market on the map; skip it.
				continue
			}
			lat, lon = pt.Lat, pt.Lon
		}
		// Scope to the anchor radius (keeps per-poll request count small).
		if anchorOK && !geo.WithinRadius(anchor, geo.Point{Lat: lat, Lon: lon}, bauhausRadiusKm) {
			continue
		}
		id := bauhausStoreID(m.StoreID)
		street := m.street()
		stores = append(stores, domain.Store{
			ID:        id,
			SourceKey: "bauhaus",
			Name:      "BAUHAUS " + m.Name,
			Street:    &street,
			PLZ:       m.zip(),
			City:      m.city(),
			Lat:       lat,
			Lon:       lon,
		})
		places = append(places, placed{storeNo: m.StoreID, storeID: id})
	}
	if len(places) == 0 {
		return nil, nil, errors.New("bauhaus: store directory returned no placeable markets")
	}

	// Query per-store stock for every (product, market) pair concurrently.
	type job struct {
		item  tracked
		place placed
	}
	var jobs []job
	for _, it := range items {
		for _, pl := range places {
			jobs = append(jobs, job{item: it, place: pl})
		}
	}

	var (
		mu       sync.Mutex
		listings []domain.Listing
		failures int
	)
	sem := make(chan struct{}, bauhausAvailabilityWorkers)
	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			l, err := s.fetchMarketListing(ctx, j.item.product, j.item.sku, j.item.pageURL, j.place.storeNo, j.place.storeID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures++
				return
			}
			listings = append(listings, l)
		}()
	}
	wg.Wait()

	// Only a total failure (every store lookup failed) is a source-level error.
	if failures == len(jobs) {
		return nil, nil, fmt.Errorf("bauhaus: all %d store lookup(s) failed", len(jobs))
	}
	return listings, stores, nil
}

// fetchMarketListing queries one market's availability for one SKU and builds a
// store-keyed listing from the reservation branch of the availability map.
func (s *bauhausSource) fetchMarketListing(ctx context.Context, p domain.Product, sku, pageURL, storeNo, storeID string) (domain.Listing, error) {
	body, err := s.httpClient().GetHTML(ctx, s.availabilityURL(sku, storeNo))
	if err != nil {
		return domain.Listing{}, fmt.Errorf("bauhaus: availability store %s: %w", storeNo, err)
	}

	var resp bauhausAvailabilityResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return domain.Listing{}, fmt.Errorf("bauhaus: decode availability store %s: %w", storeNo, err)
	}

	storeIDCopy := storeID
	listing := domain.Listing{
		SourceKey: "bauhaus",
		ProductID: p.ID,
		StoreID:   &storeIDCopy,
		Currency:  "EUR",
		URL:       pageURL,
	}

	// A missing reservation branch (not offered for reserve & collect at this
	// market) counts as out of stock, not an error.
	if res, ok := resp.AvailabilityMap["reservation"]; ok {
		listing.InStock = bauhausInStockStates[strings.ToLower(strings.TrimSpace(res.State))]
		if res.Price > 0 {
			cents := int(math.Round(res.Price * 100))
			listing.PriceCents = &cents
		}
	}
	return listing, nil
}

// resolveArticle returns the BAUHAUS SKU and product page URL for p: the known
// mapping if it matches, otherwise the first product link on the search results
// page.
func (s *bauhausSource) resolveArticle(ctx context.Context, p domain.Product) (sku, pageURL string, err error) {
	for _, k := range knownBauhausArticles {
		if k.match(p) {
			return k.sku, fmt.Sprintf("%s/p/%s", s.base(), k.sku), nil
		}
	}
	return s.searchArticle(ctx, p)
}

// bauhausProductLinkRe matches a BAUHAUS product path: /p/<digits> (no slug);
// group 1 is the SKU.
var bauhausProductLinkRe = regexp.MustCompile(`/p/(\d+)`)

// searchArticle fetches the search results page for p.Name and returns the SKU
// and product URL of the first product link found.
func (s *bauhausSource) searchArticle(ctx context.Context, p domain.Product) (string, string, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s", s.base(), url.QueryEscape(p.Name))
	html, err := s.httpClient().GetHTML(ctx, searchURL)
	if err != nil {
		return "", "", fmt.Errorf("search %q: %w", p.Name, err)
	}
	m := bauhausProductLinkRe.FindStringSubmatch(string(html))
	if m == nil {
		return "", "", fmt.Errorf("search %q: no product link found", p.Name)
	}
	return m[1], fmt.Sprintf("%s/p/%s", s.base(), m[1]), nil
}

// fetchMarkets fetches and decodes the German BAUHAUS store directory. It accepts
// both {"stores":[...]} and a bare top-level array.
func (s *bauhausSource) fetchMarkets(ctx context.Context) ([]bauhausStore, error) {
	body, err := s.httpClient().GetHTML(ctx, s.base()+"/api/stores-search")
	if err != nil {
		return nil, err
	}
	var resp bauhausStoreLocator
	if err := json.Unmarshal(body, &resp); err == nil && len(resp.Stores) > 0 {
		return resp.Stores, nil
	}
	// Fall back to a bare top-level array.
	var arr []bauhausStore
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	// Re-decode the wrapper to surface a meaningful decode error.
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode directory: %w", err)
	}
	return resp.Stores, nil
}

// availabilityURL builds the per-store availability endpoint for a SKU.
func (s *bauhausSource) availabilityURL(sku, storeNo string) string {
	return fmt.Sprintf("%s/api/product-availability/availability-detail/%s?storeId=%s&searchRadius=100",
		s.base(), url.PathEscape(sku), url.QueryEscape(storeNo))
}

// bauhausStoreID returns the deterministic store id for a BAUHAUS store number.
func bauhausStoreID(storeNo string) string {
	return uuid.NewSHA1(bauhausStoreNamespace, []byte("bauhaus:store:"+storeNo)).String()
}

// --- JSON shapes (BAUHAUS internal endpoints) ---

// bauhausStoreLocator is the /api/stores-search response wrapper.
type bauhausStoreLocator struct {
	Stores []bauhausStore `json:"stores"`
}

// bauhausStore is one BAUHAUS Fachcentrum. Address fields appear both at the top
// level and nested under "address"; accessors prefer the top-level value and
// fall back to the nested one. Coordinates live under "geolocation". A missing
// "active" flag is treated as active.
type bauhausStore struct {
	StoreID     string             `json:"storeId"`
	Name        string             `json:"name"`
	Street      string             `json:"street"`
	Zip         string             `json:"zip"`
	City        string             `json:"city"`
	Active      *bool              `json:"active"`
	Geolocation bauhausGeolocation `json:"geolocation"`
	Address     bauhausStoreAddr   `json:"address"`
}

type bauhausGeolocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type bauhausStoreAddr struct {
	Street string  `json:"street"`
	Zip    string  `json:"zip"`
	City   string  `json:"city"`
	Lat    float64 `json:"latitude"`
	Lon    float64 `json:"longitude"`
}

func (m bauhausStore) street() string { return firstNonEmpty(m.Street, m.Address.Street) }
func (m bauhausStore) zip() string    { return firstNonEmpty(m.Zip, m.Address.Zip) }
func (m bauhausStore) city() string   { return firstNonEmpty(m.City, m.Address.City) }
func (m bauhausStore) lat() float64   { return firstNonZero(m.Geolocation.Latitude, m.Address.Lat) }
func (m bauhausStore) lon() float64   { return firstNonZero(m.Geolocation.Longitude, m.Address.Lon) }

// active reports whether the market is active; a missing flag is treated as
// active.
func (m bauhausStore) active() bool { return m.Active == nil || *m.Active }

// bauhausAvailabilityResp is the availability-detail response: a map keyed by
// purchase option (e.g. "reservation", "online-order").
type bauhausAvailabilityResp struct {
	AvailabilityMap map[string]bauhausAvailabilityEntry `json:"availabilityMap"`
}

// bauhausAvailabilityEntry is one purchase-option branch. State carries the
// stock state; Price (euros) is optional.
type bauhausAvailabilityEntry struct {
	State string  `json:"state"`
	Price float64 `json:"price"`
}
