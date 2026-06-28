// Package domain holds the shared data structures for PortaSplitFinder.
// These mirror the API contract field names exactly and import neither DB nor
// HTTP packages.
package domain

import "time"

// SourceType distinguishes online retailers from physical stores.
type SourceType string

const (
	// SourceOnline is a nationwide online retailer (amazon, ebay, ...).
	SourceOnline SourceType = "online"
	// SourceStore is a physical store chain with branches (obi, ...).
	SourceStore SourceType = "store"
)

// Valid reports whether t is a known SourceType.
func (t SourceType) Valid() bool {
	return t == SourceOnline || t == SourceStore
}

// LastStatus is the outcome of the most recent scrape of a source.
type LastStatus string

const (
	// StatusOK means the last scrape succeeded.
	StatusOK LastStatus = "ok"
	// StatusError means the last scrape failed.
	StatusError LastStatus = "error"
	// StatusNever means the source has not been scraped yet.
	StatusNever LastStatus = "never"
)

// Product is a tracked Portasplit variant.
type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Model    string  `json:"model"`
	ImageURL *string `json:"imageUrl"`
}

// Source is a retailer (online or physical-store chain).
type Source struct {
	Key           string     `json:"key"`
	Name          string     `json:"name"`
	Type          SourceType `json:"type"`
	BaseURL       string     `json:"baseUrl"`
	LastCheckedAt *time.Time `json:"lastCheckedAt"`
	LastStatus    LastStatus `json:"lastStatus"`
	LastError     *string    `json:"lastError"`
}

// Store is a physical branch belonging to a store-type source.
type Store struct {
	ID        string  `json:"id"`
	SourceKey string  `json:"sourceKey"`
	Name      string  `json:"name"`
	Street    *string `json:"street"`
	PLZ       string  `json:"plz"`
	City      string  `json:"city"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
}

// Listing is the availability of one product at one source (and store, if
// physical).
type Listing struct {
	SourceKey  string    `json:"sourceKey"`
	ProductID  string    `json:"productId"`
	StoreID    *string   `json:"storeId"`
	InStock    bool      `json:"inStock"`
	Quantity   *int      `json:"quantity"`
	PriceCents *int      `json:"priceCents"`
	Currency   string    `json:"currency"`
	URL        string    `json:"url"`
	CheckedAt  time.Time `json:"checkedAt"`
}

// Subscription is a user's request to be notified when a product becomes
// available near a PLZ. Tokens drive the double opt-in / unsubscribe flow.
type Subscription struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PLZ          string     `json:"plz"`
	RadiusKm     int        `json:"radiusKm"`
	ProductID    *string    `json:"productId"`
	ConfirmToken string     `json:"-"`
	UnsubToken   string     `json:"-"`
	CreatedAt    time.Time  `json:"createdAt"`
	ConfirmedAt  *time.Time `json:"confirmedAt"`
	UnsubbedAt   *time.Time `json:"unsubbedAt"`
}

// Confirmed reports whether the subscription has been confirmed and not
// unsubscribed.
func (s Subscription) Confirmed() bool {
	return s.ConfirmedAt != nil && s.UnsubbedAt == nil
}

// AvailabilitySnapshot is a point-in-time record of a listing, written by the
// poller. The latest snapshot per (source, store, product) is the current
// availability.
type AvailabilitySnapshot struct {
	ID         string    `json:"id"`
	SourceKey  string    `json:"sourceKey"`
	StoreID    *string   `json:"storeId"`
	ProductID  string    `json:"productId"`
	InStock    bool      `json:"inStock"`
	Quantity   *int      `json:"quantity"`
	PriceCents *int      `json:"priceCents"`
	Currency   string    `json:"currency"`
	URL        string    `json:"url"`
	CheckedAt  time.Time `json:"checkedAt"`
}
