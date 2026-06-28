package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/tranmh/portasplitfinder/internal/domain"
)

// obiSource reports obi.de webshop (online) availability for the tracked
// products. It resolves each product to an OBI product page, parses the
// schema.org Product JSON-LD embedded in the page, and emits one online listing
// (StoreID=nil) per product. Per-market store-level stock is deferred.
//
// OBI is bot-protected: server-side fetches by non-browser clients are commonly
// rejected (403/404). The httpx client presents a browser User-Agent, and Fetch
// degrades gracefully when a product is blocked or unparseable: it skips that
// product and only returns a source-level error if *every* product failed.
type obiSource struct {
	client  *Client
	baseURL string
}

// newOBISource builds an obiSource with default browser-like client and base
// URL. Tests construct obiSource directly to point at an httptest.Server.
func newOBISource() *obiSource {
	return &obiSource{
		client:  NewClient(),
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

// Meta implements Source. OBI v1 reports nationwide webshop availability, so the
// type is online (the availability endpoint renders StoreID=nil listings in the
// online panel keyed by source).
func (s *obiSource) Meta() domain.Source {
	return domain.Source{
		Key:        "obi",
		Name:       "OBI",
		Type:       domain.SourceOnline,
		BaseURL:    "https://www.obi.de",
		LastStatus: domain.StatusNever,
	}
}

// knownArticleNos maps a recognised product to its OBI article number. Extend
// this small table as more products gain a verified OBI mapping. The Midea
// PortaSplit (our primary tracked product) is article 8620890.
var knownObiArticles = []struct {
	// match reports whether a product is this known item.
	match    func(p domain.Product) bool
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

// Fetch implements Source. For each product it resolves an OBI product URL
// (known mapping first, then search fallback), fetches and parses the page, and
// emits one online listing. Returns no stores. If every product fails, returns a
// non-nil error so the poller records the source as errored.
func (s *obiSource) Fetch(ctx context.Context, products []domain.Product) ([]domain.Listing, []domain.Store, error) {
	var listings []domain.Listing
	var firstErr error
	attempted := 0
	failed := 0

	for _, p := range products {
		attempted++
		l, err := s.fetchProduct(ctx, p)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listings = append(listings, l)
	}

	// If we had products but every one failed, surface a source-level error.
	if attempted > 0 && failed == attempted {
		return nil, nil, fmt.Errorf("obi: all %d product(s) failed; first error: %w", attempted, firstErr)
	}
	return listings, nil, nil
}

// fetchProduct resolves, fetches, and parses one product.
func (s *obiSource) fetchProduct(ctx context.Context, p domain.Product) (domain.Listing, error) {
	pageURL, err := s.resolveURL(ctx, p)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("obi: resolve %q: %w", p.Name, err)
	}

	html, err := s.httpClient().GetHTML(ctx, pageURL)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("obi: fetch %s: %w", pageURL, err)
	}

	prod, err := parseProductJSONLD(html)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("obi: parse %s: %w", pageURL, err)
	}

	listing := domain.Listing{
		SourceKey: "obi",
		ProductID: p.ID,
		StoreID:   nil,
		InStock:   prod.inStock,
		Currency:  prod.currency,
		URL:       pageURL,
	}
	if prod.priceCents != nil {
		listing.PriceCents = prod.priceCents
	}
	return listing, nil
}

// resolveURL returns the OBI product page URL for p: the known mapping if it
// matches, otherwise the first product link found on the search results page.
func (s *obiSource) resolveURL(ctx context.Context, p domain.Product) (string, error) {
	for _, k := range knownObiArticles {
		if k.match(p) {
			return fmt.Sprintf("%s/p/%s/%s", s.base(), k.articleNo, k.slug), nil
		}
	}
	return s.searchURL(ctx, p)
}

// productLinkRe matches an OBI product path: /p/<digits>/<slug>.
var productLinkRe = regexp.MustCompile(`/p/(\d+)/[^"'?#\s]+`)

// searchURL fetches the search results page for p.Name and returns the first
// product link found.
func (s *obiSource) searchURL(ctx context.Context, p domain.Product) (string, error) {
	term := url.PathEscape(p.Name)
	searchURL := fmt.Sprintf("%s/search/%s/", s.base(), term)

	html, err := s.httpClient().GetHTML(ctx, searchURL)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return "", fmt.Errorf("search: parse html: %w", err)
	}

	var found string
	doc.Find(`a[href]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		href, _ := sel.Attr("href")
		if m := productLinkRe.FindString(href); m != "" {
			found = m
			return false
		}
		return true
	})

	// Fallback: scan raw HTML if no anchor matched (some links are JS-rendered).
	if found == "" {
		if m := productLinkRe.FindString(string(html)); m != "" {
			found = m
		}
	}

	if found == "" {
		return "", errors.New("search: no product link found")
	}
	if strings.HasPrefix(found, "http") {
		return found, nil
	}
	return s.base() + found, nil
}

// --- JSON-LD parsing ---

// parsedProduct holds the fields we extract from a schema.org Product offer.
type parsedProduct struct {
	inStock    bool
	priceCents *int
	currency   string
}

// ldOffer mirrors the schema.org Offer / AggregateOffer fields we read. price
// and lowPrice are decoded leniently (they may be a number or a string).
type ldOffer struct {
	Type         string      `json:"@type"`
	Price        json.Number `json:"price"`
	LowPrice     json.Number `json:"lowPrice"`
	PriceCurrency string     `json:"priceCurrency"`
	Availability string      `json:"availability"`
}

// ldNode is a single JSON-LD node; offers is decoded separately because it can
// be an object or an array.
type ldNode struct {
	Type   json.RawMessage `json:"@type"`
	Graph  []ldNode        `json:"@graph"`
	Offers json.RawMessage `json:"offers"`
}

// typeContains reports whether the node's @type (string or array) contains want.
func (n ldNode) typeContains(want string) bool {
	if len(n.Type) == 0 {
		return false
	}
	var single string
	if err := json.Unmarshal(n.Type, &single); err == nil {
		return single == want
	}
	var many []string
	if err := json.Unmarshal(n.Type, &many); err == nil {
		for _, t := range many {
			if t == want {
				return true
			}
		}
	}
	return false
}

// parseProductJSONLD extracts the first schema.org Product offer from any
// application/ld+json block in the page. It handles JSON-LD that is a single
// object, an array of objects, or wraps nodes in @graph; offers that are an
// Offer, an AggregateOffer, or an array of offers; and price that is a number or
// string (using lowPrice when price is absent).
func parseProductJSONLD(html []byte) (parsedProduct, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return parsedProduct{}, fmt.Errorf("parse html: %w", err)
	}

	var found bool
	var result parsedProduct
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		nodes := decodeLDNodes([]byte(sel.Text()))
		for _, n := range nodes {
			if !n.typeContains("Product") {
				continue
			}
			offer, ok := pickOffer(n.Offers)
			if !ok {
				continue
			}
			result = offerToProduct(offer)
			found = true
			return false
		}
		return true
	})

	if !found {
		return parsedProduct{}, errors.New("no schema.org Product offer found")
	}
	return result, nil
}

// decodeLDNodes returns all top-level nodes in a JSON-LD block, flattening
// arrays and @graph wrappers one level.
func decodeLDNodes(raw []byte) []ldNode {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil
	}

	var out []ldNode
	// Try a single object first.
	var single ldNode
	if err := json.Unmarshal(raw, &single); err == nil {
		if len(single.Graph) > 0 {
			out = append(out, single.Graph...)
		}
		out = append(out, single)
		return out
	}
	// Try an array of objects.
	var arr []ldNode
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, n := range arr {
			if len(n.Graph) > 0 {
				out = append(out, n.Graph...)
			}
			out = append(out, n)
		}
		return out
	}
	return nil
}

// pickOffer decodes an offers value that may be a single Offer/AggregateOffer or
// an array of them, returning the first usable one.
func pickOffer(raw json.RawMessage) (ldOffer, bool) {
	if len(raw) == 0 {
		return ldOffer{}, false
	}
	var single ldOffer
	if err := json.Unmarshal(raw, &single); err == nil && (single.Type != "" || single.Price != "" || single.LowPrice != "" || single.Availability != "") {
		return single, true
	}
	var arr []ldOffer
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0], true
	}
	return ldOffer{}, false
}

// offerToProduct converts a decoded offer to our parsedProduct.
func offerToProduct(o ldOffer) parsedProduct {
	cur := strings.ToUpper(strings.TrimSpace(o.PriceCurrency))
	if cur == "" {
		cur = "EUR"
	}
	p := parsedProduct{
		inStock:  availabilityInStock(o.Availability),
		currency: cur,
	}

	priceStr := o.Price.String()
	if priceStr == "" {
		priceStr = o.LowPrice.String()
	}
	if priceStr != "" {
		if f, err := strconv.ParseFloat(priceStr, 64); err == nil {
			cents := int(math.Round(f * 100))
			p.priceCents = &cents
		}
	}
	return p
}

// availabilityInStock interprets a schema.org availability URL. It treats the
// product as out-of-stock only for explicit unavailable states; everything else
// (InStock, InStoreOnly, OnlineOnly, PreOrder, LimitedAvailability, …) counts as
// available. The scheme (http/https) and trailing path vary, so we match on the
// final token.
func availabilityInStock(avail string) bool {
	a := strings.ToLower(strings.TrimSpace(avail))
	if a == "" {
		// No availability stated: assume not in stock to avoid false positives.
		return false
	}
	// Keep only the final path segment, e.g. ".../InStock" -> "instock".
	if i := strings.LastIndex(a, "/"); i >= 0 {
		a = a[i+1:]
	}
	switch a {
	case "outofstock", "soldout", "discontinued", "backorder":
		return false
	default:
		return true
	}
}
