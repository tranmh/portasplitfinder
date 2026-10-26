package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/tranmh/portasplitfinder/internal/geo"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// StoreSearchResponse is the body of GET /api/stores.
type StoreSearchResponse struct {
	Stores   []StoreAvailability `json:"stores"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

// handleStoreSearch serves GET /api/stores: physical stores whose name or city
// matches q, with their current listings. With a plz the stores carry their
// distance and are ordered nearest first.
func (s *Server) handleStoreSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	term := strings.TrimSpace(q.Get("q"))

	var center *Center
	if plz := q.Get("plz"); plz != "" {
		if !validPLZ(plz) {
			writeError(w, http.StatusBadRequest, codeValidation, "plz must be 5 digits", fieldPtr("plz"))
			return
		}
		pt, ok := geo.Lookup(plz)
		if !ok {
			writeError(w, http.StatusBadRequest, codeValidation, "unknown plz", fieldPtr("plz"))
			return
		}
		center = &Center{PLZ: plz, Lat: pt.Lat, Lon: pt.Lon}
	}

	page, ok := intParam(q.Get("page"), 1, 1, 1<<20)
	if !ok {
		writeError(w, http.StatusBadRequest, codeValidation, "page must be a positive integer", fieldPtr("page"))
		return
	}
	pageSize, ok := intParam(q.Get("pageSize"), defaultPageSize, 1, maxPageSize)
	if !ok {
		writeError(w, http.StatusBadRequest, codeValidation, "pageSize must be an integer in 1..100", fieldPtr("pageSize"))
		return
	}

	found, err := s.store.SearchStores(r.Context(), term)
	if err != nil {
		s.internal(w, err)
		return
	}

	results := make([]StoreAvailability, 0, len(found))
	for _, st := range found {
		results = append(results, storeEntry(st, nil, center, ""))
	}
	if center != nil {
		sortStoreEntries(results)
	}

	start := (page - 1) * pageSize
	if start > len(results) {
		start = len(results)
	}
	end := start + pageSize
	if end > len(results) {
		end = len(results)
	}
	pageItems := results[start:end]

	for i := range pageItems {
		snaps, err := s.store.LatestSnapshotsForStore(r.Context(), pageItems[i].Store.ID)
		if err != nil {
			s.internal(w, err)
			return
		}
		pageItems[i].Listings = storeEntry(pageItems[i].Store, snaps, center, "").Listings
	}

	writeJSON(w, http.StatusOK, StoreSearchResponse{
		Stores:   pageItems,
		Total:    len(results),
		Page:     page,
		PageSize: pageSize,
	})
}

// intParam parses an optional integer query parameter in [lo, hi]; an empty
// value yields def.
func intParam(v string, def, lo, hi int) (int, bool) {
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, false
	}
	return n, true
}
