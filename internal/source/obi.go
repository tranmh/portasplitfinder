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

// obiSource reports per-market (store-level) availability of the tracked
// products across all German OBI markets.
//
// OBI's webshop does not ship the tracked products to a home address
// ("Lieferung nach Hause: derzeit nicht möglich") — availability is offered only
// as reserve & collect in physical markets. So OBI is a store source: Fetch
// returns one domain.Store per OBI market (with lat/lon) and one listing per
// market keyed by that store, producing real map pins.
//
// It uses two undocumented JSON endpoints that obi.de's own frontend calls:
//
//   - store directory (all German markets, with coordinates):
//     GET /api/disc/store/locator/country/DE
//   - per-market stock for an article:
//     GET /api/disc/article-service-proxy/store-specific-articles/v2/DE/de/{storeNumber}?articleNumbers={art}
//     → [{"storeSpecificArticle":{"price":799.99,"stock":0}, "currency":"EUR", ...}]
//
// OBI is bot-protected: the shared httpx Client presents a browser User-Agent.
// Fetch degrades gracefully — a per-market availability failure skips that
// market's listing, and a source-level error is returned only if the directory
// itself fails or every market lookup fails.
type obiSource struct {
	client  *Client
	baseURL string
}

// obiStoreNamespace seeds deterministic (UUIDv5) store ids derived from a
// market's OBI store number, so a market keeps the same store id across polls
// and the in-Fetch listing can reference its store before the poller upserts it.
var obiStoreNamespace = uuid.MustParse("6f0b9f7e-9b1a-5c2d-8e3f-0a1b2c3d4e5f")

// obiAvailabilityWorkers bounds concurrent per-market availability requests.
const obiAvailabilityWorkers = 8

// newOBISource builds an obiSource with default browser-like client and base
// URL. Tests construct obiSource directly to point at an httptest.Server.
func newOBISource() *obiSource {
	return &obiSource{
		client:  NewClient(WithUnblocker(unblockerTemplate)),
		baseURL: "https://www.obi.de",
	}
}

func (s *obiSource) base() string {
	if s.baseURL == "" {
		return "https://www.obi.de"
	}
	return strings.TrimRight(s.baseURL, "/")
}

func (s *obiSource) httpClient() *Client {
	if s.client == nil {
		s.client = NewClient()
	}
	return s.client
}

// Key implements Source.
func (s *obiSource) Key() string { return "obi" }

// Meta implements Source. OBI is a physical store chain: per-market reserve &
// collect availability renders as map pins.
func (s *obiSource) Meta() domain.Source {
	return domain.Source{
		Key:        "obi",
		Name:       "OBI",
		Type:       domain.SourceStore,
		BaseURL:    "https://www.obi.de",
		LastStatus: domain.StatusNever,
	}
}

// knownObiArticles maps a recognised product to its OBI article number. Extend
// this small table as more products gain a verified OBI mapping. The Midea
// PortaSplit (our primary tracked product) is article 8620890.
var knownObiArticles = []struct {
	// match reports whether a product is this known item.
	match     func(p domain.Product) bool
	articleNo string
	slug      string
}{
	{
		match: func(p domain.Product) bool {
			name := strings.ToLower(p.Name + " " + p.Model)
			return strings.Contains(name, "midea") &&
				(strings.Contains(name, "portasplit") || strings.Contains(name, "porta split"))
		},
		articleNo: "8620890",
		slug:      "midea-mobile-split-klimaanlage-portasplit",
	},
}

// Fetch implements Source. It resolves each product to an OBI article number,
// fetches the German market directory once, then queries per-market stock for
// every (product, market) pair and emits a store-keyed listing per market.
func (s *obiSource) Fetch(ctx context.Context, products []domain.Product) ([]domain.Listing, []domain.Store, error) {
	// Resolve the tracked products that have an OBI article number; skip the rest.
	type tracked struct {
		product   domain.Product
		articleNo string
		pageURL   string
	}
	var items []tracked
	for _, p := range products {
		art, pageURL, err := s.resolveArticle(ctx, p)
		if err != nil {
			continue
		}
		items = append(items, tracked{product: p, articleNo: art, pageURL: pageURL})
	}
	if len(items) == 0 {
		// No tracked product maps to OBI: nothing to do, but not an error.
		return nil, nil, nil
	}

	markets, err := s.fetchMarkets(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("obi: market directory: %w", err)
	}

	// Build the placeable markets (those we can show on the map) and their stores.
	type placed struct {
		storeNo string
		storeID string
	}
	var stores []domain.Store
	var places []placed
	for _, m := range markets {
		if !m.IsActive || m.StoreNumber == "" {
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
		id := obiStoreID(m.StoreNumber)
		street := m.street()
		stores = append(stores, domain.Store{
			ID:        id,
			SourceKey: "obi",
			Name:      "OBI " + m.Name,
			Street:    &street,
			PLZ:       m.zip(),
			City:      m.city(),
			Lat:       lat,
			Lon:       lon,
		})
		places = append(places, placed{storeNo: m.StoreNumber, storeID: id})
	}
	if len(places) == 0 {
		return nil, nil, errors.New("obi: market directory returned no placeable markets")
	}

	// Query per-market stock for every (product, market) pair concurrently.
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
	sem := make(chan struct{}, obiAvailabilityWorkers)
	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			l, err := s.fetchMarketListing(ctx, j.item.product, j.item.articleNo, j.item.pageURL, j.place.storeNo, j.place.storeID)
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

	// Only a total failure (every market lookup failed) is a source-level error.
	if failures == len(jobs) {
		return nil, nil, fmt.Errorf("obi: all %d market lookup(s) failed", len(jobs))
	}
	return listings, stores, nil
}

// fetchMarketListing queries one market's stock for one article and builds a
// store-keyed listing.
func (s *obiSource) fetchMarketListing(ctx context.Context, p domain.Product, articleNo, pageURL, storeNo, storeID string) (domain.Listing, error) {
	body, err := s.httpClient().GetHTML(ctx, s.availabilityURL(storeNo, articleNo))
	if err != nil {
		return domain.Listing{}, fmt.Errorf("obi: availability store %s: %w", storeNo, err)
	}

	var arts []obiArticle
	if err := json.Unmarshal(body, &arts); err != nil {
		return domain.Listing{}, fmt.Errorf("obi: decode availability store %s: %w", storeNo, err)
	}

	var match *obiArticle
	for i := range arts {
		if arts[i].ArticleNumber == articleNo {
			match = &arts[i]
			break
		}
	}
	// A missing article (not carried at this market) counts as out of stock.
	storeIDCopy := storeID
	listing := domain.Listing{
		SourceKey: "obi",
		ProductID: p.ID,
		StoreID:   &storeIDCopy,
		Currency:  "EUR",
		URL:       pageURL,
	}
	if match != nil {
		if ssa := match.StoreSpecific; ssa != nil {
			qty := ssa.Stock
			listing.InStock = ssa.Stock > 0
			listing.Quantity = &qty
			if ssa.Price > 0 {
				cents := int(math.Round(ssa.Price * 100))
				listing.PriceCents = &cents
			}
		}
		if c := strings.ToUpper(strings.TrimSpace(match.Currency.IsoCode)); c != "" {
			listing.Currency = c
		}
	}
	return listing, nil
}

// resolveArticle returns the OBI article number and product page URL for p: the
// known mapping if it matches, otherwise the first product link on the search
// results page.
func (s *obiSource) resolveArticle(ctx context.Context, p domain.Product) (articleNo, pageURL string, err error) {
	for _, k := range knownObiArticles {
		if k.match(p) {
			return k.articleNo, fmt.Sprintf("%s/p/%s/%s", s.base(), k.articleNo, k.slug), nil
		}
	}
	return s.searchArticle(ctx, p)
}

// productLinkRe matches an OBI product path: /p/<digits>/<slug>; group 1 is the
// article number.
var productLinkRe = regexp.MustCompile(`/p/(\d+)/[^"'?#\s]+`)

// searchArticle fetches the search results page for p.Name and returns the
// article number and product URL of the first product link found.
func (s *obiSource) searchArticle(ctx context.Context, p domain.Product) (string, string, error) {
	searchURL := fmt.Sprintf("%s/search/%s/", s.base(), url.PathEscape(p.Name))
	html, err := s.httpClient().GetHTML(ctx, searchURL)
	if err != nil {
		return "", "", fmt.Errorf("search %q: %w", p.Name, err)
	}
	m := productLinkRe.FindStringSubmatch(string(html))
	if m == nil {
		return "", "", fmt.Errorf("search %q: no product link found", p.Name)
	}
	return m[1], s.base() + m[0], nil
}

// fetchMarkets fetches and decodes the German OBI market directory.
func (s *obiSource) fetchMarkets(ctx context.Context) ([]obiMarket, error) {
	body, err := s.httpClient().GetHTML(ctx, s.base()+"/api/disc/store/locator/country/DE")
	if err != nil {
		return nil, err
	}
	var resp obiStoreLocator
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode directory: %w", err)
	}
	return resp.Stores, nil
}

// availabilityURL builds the per-market stock endpoint for an article.
func (s *obiSource) availabilityURL(storeNo, articleNo string) string {
	return fmt.Sprintf("%s/api/disc/article-service-proxy/store-specific-articles/v2/DE/de/%s?articleNumbers=%s",
		s.base(), url.PathEscape(storeNo), url.QueryEscape(articleNo))
}

// obiStoreID returns the deterministic store id for an OBI market number.
func obiStoreID(storeNo string) string {
	return uuid.NewSHA1(obiStoreNamespace, []byte("obi:store:"+storeNo)).String()
}

// --- JSON shapes (OBI internal endpoints) ---

// obiStoreLocator is the /store/locator/country/DE response.
type obiStoreLocator struct {
	Stores []obiMarket `json:"stores"`
}

// obiMarket is one OBI market. Address fields appear both at the top level and
// nested under "address"; accessors prefer the top-level value and fall back to
// the nested one.
type obiMarket struct {
	StoreNumber string        `json:"storeNumber"`
	Name        string        `json:"name"`
	Street      string        `json:"street"`
	Zip         string        `json:"zip"`
	City        string        `json:"city"`
	Lat         float64       `json:"lat"`
	Lon         float64       `json:"lon"`
	IsActive    bool          `json:"isActive"`
	Address     obiMarketAddr `json:"address"`
}

type obiMarketAddr struct {
	Street string  `json:"street"`
	Zip    string  `json:"zip"`
	City   string  `json:"city"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

func (m obiMarket) street() string { return firstNonEmpty(m.Street, m.Address.Street) }
func (m obiMarket) zip() string    { return firstNonEmpty(m.Zip, m.Address.Zip) }
func (m obiMarket) city() string   { return firstNonEmpty(m.City, m.Address.City) }
func (m obiMarket) lat() float64   { return firstNonZero(m.Lat, m.Address.Lat) }
func (m obiMarket) lon() float64   { return firstNonZero(m.Lon, m.Address.Lon) }

// obiArticle is one element of the store-specific-articles response.
type obiArticle struct {
	ArticleNumber string            `json:"articleNumber"`
	Currency      obiCurrency       `json:"currency"`
	StoreSpecific *obiStoreSpecific `json:"storeSpecificArticle"`
}

// obiCurrency is the article's currency, e.g. {"isoCode":"EUR","symbol":"€"}.
type obiCurrency struct {
	IsoCode string `json:"isoCode"`
	Symbol  string `json:"symbol"`
}

type obiStoreSpecific struct {
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...float64) float64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}
