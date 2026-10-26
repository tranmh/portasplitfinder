package httpapi

import (
	"sort"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
)

// Center is the optional geographic center of an availability query.
type Center struct {
	PLZ string  `json:"plz"`
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// StoreAvailability groups a physical store with its listings and distance.
type StoreAvailability struct {
	Store      domain.Store     `json:"store"`
	DistanceKm *float64         `json:"distanceKm,omitempty"`
	Listings   []domain.Listing `json:"listings"`
}

// OnlineAvailability groups an online source with its listings.
type OnlineAvailability struct {
	Source   domain.Source    `json:"source"`
	Listings []domain.Listing `json:"listings"`
}

// AvailabilityResponse is the body of GET /api/availability.
type AvailabilityResponse struct {
	Center      *Center              `json:"center"`
	RadiusKm    int                  `json:"radiusKm"`
	Stores      []StoreAvailability  `json:"stores"`
	Online      []OnlineAvailability `json:"online"`
	GeneratedAt time.Time            `json:"generatedAt"`
}

// availabilityInputs is everything the assembler needs, supplied by the handler
// (from the DB) or by tests (fakes).
type availabilityInputs struct {
	snapshots []domain.AvailabilitySnapshot
	stores    []domain.Store
	sources   []domain.Source
}

// assembleAvailability builds the response from latest snapshots, splitting
// physical-store vs online listings, computing distances, and applying the PLZ
// radius filter.
//
//   - center is nil when no plz was given (distances are null, no radius filter).
//   - productID, when non-empty, restricts to that product.
func assembleAvailability(in availabilityInputs, center *Center, radiusKm int, productID string) AvailabilityResponse {
	sourceByKey := make(map[string]domain.Source, len(in.sources))
	for _, s := range in.sources {
		sourceByKey[s.Key] = s
	}

	// Physical stores: one entry per store that has at least one listing.
	var stores []StoreAvailability
	for _, st := range in.stores {
		entry := storeEntry(st, in.snapshots, center, productID)
		if len(entry.Listings) == 0 {
			continue
		}
		if entry.DistanceKm != nil && radiusKm > 0 && *entry.DistanceKm > float64(radiusKm) {
			continue // outside radius
		}
		stores = append(stores, entry)
	}
	sortStoreEntries(stores)

	// Online listings are keyed by source.
	onlineListings := map[string][]domain.Listing{}
	for _, snap := range in.snapshots {
		if productID != "" && snap.ProductID != productID {
			continue
		}
		if snap.StoreID == nil || *snap.StoreID == "" {
			onlineListings[snap.SourceKey] = append(onlineListings[snap.SourceKey], snapshotToListing(snap))
		}
	}

	// Build online availability entries, sorted by source key for stability.
	var online []OnlineAvailability
	var onlineKeys []string
	for k := range onlineListings {
		onlineKeys = append(onlineKeys, k)
	}
	sort.Strings(onlineKeys)
	for _, k := range onlineKeys {
		listings := onlineListings[k]
		sortListings(listings)
		src, ok := sourceByKey[k]
		if !ok {
			src = domain.Source{Key: k, Name: k, Type: domain.SourceOnline}
		}
		online = append(online, OnlineAvailability{Source: src, Listings: listings})
	}

	return AvailabilityResponse{
		Center:      center,
		RadiusKm:    radiusKm,
		Stores:      stores,
		Online:      online,
		GeneratedAt: time.Now().UTC(),
	}
}

// storeEntry builds the availability entry for one store from the given
// snapshots: its listings (optionally restricted to productID, in-stock first)
// and, when center is set, its distance from the center.
func storeEntry(st domain.Store, snaps []domain.AvailabilitySnapshot, center *Center, productID string) StoreAvailability {
	listings := []domain.Listing{}
	for _, snap := range snaps {
		if snap.StoreID == nil || *snap.StoreID != st.ID {
			continue
		}
		if productID != "" && snap.ProductID != productID {
			continue
		}
		listings = append(listings, snapshotToListing(snap))
	}
	sortListings(listings)

	var dist *float64
	if center != nil {
		d := geo.DistanceKm(geo.Point{Lat: center.Lat, Lon: center.Lon}, geo.Point{Lat: st.Lat, Lon: st.Lon})
		dist = &d
	}
	return StoreAvailability{Store: st, DistanceKm: dist, Listings: listings}
}

// sortStoreEntries orders stores nearest first when distances are known, else
// by name.
func sortStoreEntries(stores []StoreAvailability) {
	sort.SliceStable(stores, func(i, j int) bool {
		a, b := stores[i], stores[j]
		switch {
		case a.DistanceKm != nil && b.DistanceKm != nil:
			if *a.DistanceKm != *b.DistanceKm {
				return *a.DistanceKm < *b.DistanceKm
			}
			return a.Store.Name < b.Store.Name
		case a.DistanceKm != nil:
			return true
		case b.DistanceKm != nil:
			return false
		default:
			return a.Store.Name < b.Store.Name
		}
	})
}

// sortListings puts in-stock first, then by price ascending (nil price last),
// then by source key for determinism.
func sortListings(ls []domain.Listing) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i], ls[j]
		if a.InStock != b.InStock {
			return a.InStock // in-stock first
		}
		ap, bp := a.PriceCents, b.PriceCents
		switch {
		case ap != nil && bp != nil:
			if *ap != *bp {
				return *ap < *bp
			}
		case ap != nil:
			return true
		case bp != nil:
			return false
		}
		return a.SourceKey < b.SourceKey
	})
}

func snapshotToListing(s domain.AvailabilitySnapshot) domain.Listing {
	return domain.Listing{
		SourceKey:  s.SourceKey,
		ProductID:  s.ProductID,
		StoreID:    s.StoreID,
		InStock:    s.InStock,
		Quantity:   s.Quantity,
		PriceCents: s.PriceCents,
		Currency:   s.Currency,
		URL:        s.URL,
		CheckedAt:  s.CheckedAt,
	}
}
