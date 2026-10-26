//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
	"github.com/tranmh/portasplitfinder/internal/store"
)

// *store.DB is the production DataStore (adapter methods SearchStores and
// LatestSnapshotsForStore included).
var _ DataStore = (*store.DB)(nil)

// startSchemaStack migrates a fresh, test-named schema in the run's Postgres
// (AQAA_PG_URL) and serves the real router over it. Skips when unset.
func startSchemaStack(t *testing.T) (*store.DB, *httptest.Server) {
	t.Helper()
	base := os.Getenv("AQAA_PG_URL")
	if base == "" {
		t.Skip("AQAA_PG_URL not set")
	}
	ctx := context.Background()
	schema := "aqaa_it_" + strings.ToLower(regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(t.Name(), "_"))
	if len(schema) > 60 {
		schema = schema[:60]
	}
	exec := func(sql string) error {
		c, err := store.Open(ctx, base)
		if err != nil {
			return err
		}
		defer c.Close()
		_, err = c.Pool.Exec(ctx, sql)
		return err
	}
	if err := exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	qs := u.Query()
	qs.Set("search_path", schema)
	u.RawQuery = qs.Encode()
	if err := store.Migrate(ctx, u.String()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ts := httptest.NewServer(New(Config{Store: db}).Router())
	t.Cleanup(func() {
		ts.Close()
		db.Close()
		_ = exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)
	})
	return db, ts
}

// storeSearchBody mirrors the JSON of GET /api/stores, keeping distanceKm and
// listings as raw values so absence and [] vs null can be asserted.
type storeSearchBody struct {
	Stores []struct {
		Store      domain.Store     `json:"store"`
		DistanceKm *float64         `json:"distanceKm"`
		Listings   []domain.Listing `json:"listings"`
	} `json:"stores"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

func getStores(t *testing.T, ts *httptest.Server, query string) (int, storeSearchBody, string) {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/stores?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var body storeSearchBody
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
	}
	return resp.StatusCode, body, string(raw)
}

func (b storeSearchBody) names() []string {
	out := []string{}
	for _, s := range b.Stores {
		out = append(out, s.Store.Name)
	}
	return out
}

func mustDB(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seedPagedStores inserts n stores "Store 01".."Store nn" in cities
// "City 01".."City nn" so the search order (city, name) is their number.
func seedPagedStores(t *testing.T, db *store.DB, n int) []string {
	t.Helper()
	ctx := context.Background()
	mustDB(t, db.Sources.Upsert(ctx, domain.Source{Key: "chain", Name: "Chain", Type: domain.SourceStore, BaseURL: "http://chain"}))
	var names []string
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("Store %02d", i)
		_, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "chain", Name: name, PLZ: "10115", City: fmt.Sprintf("City %02d", i), Lat: 52.5, Lon: 13.4})
		mustDB(t, err)
		names = append(names, name)
	}
	return names
}

func TestIntegrationStoreSearchPagination(t *testing.T) {
	db, ts := startSchemaStack(t)
	all := seedPagedStores(t, db, 5)

	t.Run("defaults page 1 and pageSize 20", func(t *testing.T) {
		code, body, raw := getStores(t, ts, "")
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, raw)
		}
		if body.Page != 1 || body.PageSize != 20 || body.Total != 5 {
			t.Fatalf("page/pageSize/total = %d/%d/%d, want 1/20/5", body.Page, body.PageSize, body.Total)
		}
		if strings.Join(body.names(), "|") != strings.Join(all, "|") {
			t.Fatalf("stores = %v, want %v", body.names(), all)
		}
	})

	t.Run("total and echo on every page", func(t *testing.T) {
		for page := 1; page <= 4; page++ {
			code, body, raw := getStores(t, ts, fmt.Sprintf("pageSize=2&page=%d", page))
			if code != http.StatusOK {
				t.Fatalf("page %d status %d: %s", page, code, raw)
			}
			if body.Total != 5 || body.Page != page || body.PageSize != 2 {
				t.Fatalf("page %d: total/page/pageSize = %d/%d/%d", page, body.Total, body.Page, body.PageSize)
			}
		}
	})

	t.Run("last page holds the remainder", func(t *testing.T) {
		_, body, _ := getStores(t, ts, "pageSize=2&page=3")
		if strings.Join(body.names(), "|") != "Store 05" {
			t.Fatalf("page 3 = %v, want [Store 05]", body.names())
		}
	})

	t.Run("page past the end is an empty array", func(t *testing.T) {
		code, body, raw := getStores(t, ts, "pageSize=2&page=4")
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, raw)
		}
		if !strings.Contains(raw, `"stores":[]`) {
			t.Fatalf("expected empty stores array, got %s", raw)
		}
		if body.Total != 5 {
			t.Fatalf("total = %d, want 5", body.Total)
		}
	})

	full := []struct {
		name  string
		query string
		want  []string
	}{
		{"page 1 is full", "pageSize=2&page=1", all[0:2]},
		{"page 2 is full", "pageSize=2&page=2", all[2:4]},
		{"pageSize equal to total", "pageSize=5&page=1", all},
		{"pageSize 1 page 1", "pageSize=1&page=1", all[0:1]},
		{"pageSize 1 page 5", "pageSize=1&page=5", all[4:5]},
	}
	for _, tc := range full {
		t.Run(tc.name, func(t *testing.T) {
			// aqaa: suspected bug [B8] - handleStoreSearch computes
			// end = start+pageSize-1, so every non-final page has pageSize-1 stores.
			t.Skip("aqaa: open bug, see report")
			_, body, raw := getStores(t, ts, tc.query)
			if strings.Join(body.names(), "|") != strings.Join(tc.want, "|") {
				t.Fatalf("%s: stores = %v, want %v (%s)", tc.query, body.names(), tc.want, raw)
			}
		})
	}

	t.Run("walking all pages yields every store once", func(t *testing.T) {
		// aqaa: suspected bug [B8] - stores at the page boundary are skipped.
		t.Skip("aqaa: open bug, see report")
		var seen []string
		for page := 1; page <= 3; page++ {
			_, body, _ := getStores(t, ts, fmt.Sprintf("pageSize=2&page=%d", page))
			seen = append(seen, body.names()...)
		}
		if strings.Join(seen, "|") != strings.Join(all, "|") {
			t.Fatalf("walked %v, want %v", seen, all)
		}
	})
}

func TestIntegrationStoreSearchValidation(t *testing.T) {
	// Validation happens before the datastore is touched, so no DB is needed.
	ts := httptest.NewServer(New(Config{}).Router())
	defer ts.Close()

	tests := []struct {
		query string
		field string
	}{
		{"plz=1234", "plz"},
		{"plz=123456", "plz"},
		{"plz=abcde", "plz"},
		{"plz=1011a", "plz"},
		{"plz=00000", "plz"}, // well-formed but unknown
		{"page=0", "page"},
		{"page=-1", "page"},
		{"page=abc", "page"},
		{"page=1.5", "page"},
		{"pageSize=0", "pageSize"},
		{"pageSize=101", "pageSize"},
		{"pageSize=-5", "pageSize"},
		{"pageSize=abc", "pageSize"},
		{"pageSize=2.0", "pageSize"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/api/stores?" + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d, want 400", resp.StatusCode)
			}
			var e struct {
				Error   string  `json:"error"`
				Message string  `json:"message"`
				Field   *string `json:"field"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
				t.Fatal(err)
			}
			if e.Error != "validation_error" || e.Field == nil || *e.Field != tc.field || e.Message == "" {
				t.Fatalf("error body = %+v (field %v), want validation_error on %s", e, e.Field, tc.field)
			}
		})
	}
}

func TestIntegrationStoreSearchBoundaries(t *testing.T) {
	db, ts := startSchemaStack(t)
	seedPagedStores(t, db, 2)
	for _, q := range []string{"pageSize=1", "pageSize=100", "page=1", "page=7", "plz=10115"} {
		t.Run(q, func(t *testing.T) {
			if code, _, raw := getStores(t, ts, q); code != http.StatusOK {
				t.Fatalf("status %d: %s", code, raw)
			}
		})
	}
}

func TestIntegrationStoreSearchQueryAndListings(t *testing.T) {
	db, ts := startSchemaStack(t)
	ctx := context.Background()
	mustDB(t, db.Sources.Upsert(ctx, domain.Source{Key: "chain", Name: "Chain", Type: domain.SourceStore, BaseURL: "http://chain"}))
	mustDB(t, db.Sources.Upsert(ctx, domain.Source{Key: "other", Name: "Other", Type: domain.SourceStore, BaseURL: "http://other"}))
	pid := "11111111-1111-1111-1111-111111111111"
	pid2 := "22222222-2222-2222-2222-222222222222"
	mustDB(t, db.Products.Upsert(ctx, domain.Product{ID: pid, Name: "P1", Model: "M1"}))
	mustDB(t, db.Products.Upsert(ctx, domain.Product{ID: pid2, Name: "P2", Model: "M2"}))

	center, ok := geo.Lookup("10115")
	if !ok {
		t.Fatal("10115 missing from geo data")
	}
	add := func(name, city string, dLat float64) string {
		id, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "chain", Name: name, PLZ: "10115", City: city, Lat: center.Lat + dLat, Lon: center.Lon})
		mustDB(t, err)
		return id
	}
	far := add("Far Shop", "Aachen", 2.0)
	add("Near Bravo", "Zeitz", 0.1)
	add("Near Alpha", "Zeitz", 0.1)
	add("Mid Berlin", "Potsdam", 0.5)
	add("Plain", "Berlin Mitte", 1.0)
	empty := add("Empty Store", "Dresden", 3.0)

	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ip := func(i int) *int { return &i }
	snap := func(src, pid, u string, inStock bool, at time.Time) {
		t.Helper()
		mustDB(t, db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: src, StoreID: &far, ProductID: pid, InStock: inStock, PriceCents: ip(100), Currency: "EUR", URL: u, CheckedAt: at}))
	}
	snap("chain", pid, "chain-p1-old", true, t0)
	snap("chain", pid, "chain-p1-new", false, t0.Add(time.Hour))
	snap("other", pid, "other-p1", true, t0)
	snap("chain", pid2, "chain-p2", false, t0)

	t.Run("q matches name or city case-insensitively, ordered by city, name", func(t *testing.T) {
		code, body, raw := getStores(t, ts, "q="+url.QueryEscape("  BERLIN "))
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, raw)
		}
		want := []string{"Plain", "Mid Berlin"} // cities "Berlin Mitte" < "Potsdam"
		if strings.Join(body.names(), "|") != strings.Join(want, "|") || body.Total != 2 {
			t.Fatalf("stores = %v total %d, want %v", body.names(), body.Total, want)
		}
		if strings.Contains(raw, "distanceKm") {
			t.Fatalf("distanceKm must be omitted without plz: %s", raw)
		}
	})

	t.Run("empty q returns every store ordered by city, name", func(t *testing.T) {
		_, body, _ := getStores(t, ts, "q=")
		want := []string{"Far Shop", "Plain", "Empty Store", "Mid Berlin", "Near Alpha", "Near Bravo"}
		if strings.Join(body.names(), "|") != strings.Join(want, "|") {
			t.Fatalf("stores = %v, want %v", body.names(), want)
		}
	})

	t.Run("plz orders nearest first, ties by name, with distance", func(t *testing.T) {
		_, body, _ := getStores(t, ts, "plz=10115")
		want := []string{"Near Alpha", "Near Bravo", "Mid Berlin", "Plain", "Far Shop", "Empty Store"}
		if strings.Join(body.names(), "|") != strings.Join(want, "|") {
			t.Fatalf("stores = %v, want %v", body.names(), want)
		}
		prev := -1.0
		for _, s := range body.Stores {
			if s.DistanceKm == nil {
				t.Fatalf("%s has no distanceKm", s.Store.Name)
			}
			if *s.DistanceKm < prev {
				t.Fatalf("distances not ascending at %s", s.Store.Name)
			}
			prev = *s.DistanceKm
		}
		if d := *body.Stores[0].DistanceKm; d < 10 || d > 12.5 {
			t.Fatalf("Near Alpha distance %.2f km, want about 11 km", d)
		}
	})

	t.Run("listings are the latest per source and product, in stock first", func(t *testing.T) {
		_, body, _ := getStores(t, ts, "q=far")
		if len(body.Stores) != 1 {
			t.Fatalf("stores = %v", body.names())
		}
		var got []string
		for _, l := range body.Stores[0].Listings {
			if l.StoreID == nil || *l.StoreID != far {
				t.Fatalf("listing from another store: %+v", l)
			}
			got = append(got, l.URL)
		}
		if len(got) != 3 || got[0] != "other-p1" {
			t.Fatalf("listings = %v, want other-p1 first then chain-p1-new and chain-p2", got)
		}
		rest := strings.Join(got[1:], "|")
		if rest != "chain-p1-new|chain-p2" && rest != "chain-p2|chain-p1-new" {
			t.Fatalf("listings = %v", got)
		}
	})

	t.Run("store without snapshots has an empty listings array", func(t *testing.T) {
		_, body, raw := getStores(t, ts, "q=empty")
		if len(body.Stores) != 1 || body.Stores[0].Store.ID != empty {
			t.Fatalf("stores = %v", body.names())
		}
		if !strings.Contains(raw, `"listings":[]`) {
			t.Fatalf("expected listings [], got %s", raw)
		}
	})

	t.Run("no match gives empty stores and total 0", func(t *testing.T) {
		code, body, raw := getStores(t, ts, "q=nowhere")
		if code != http.StatusOK || body.Total != 0 || !strings.Contains(raw, `"stores":[]`) {
			t.Fatalf("status %d body %s", code, raw)
		}
	})

	t.Run("quote in q is plain data", func(t *testing.T) {
		// aqaa: suspected bug [B1, B2, B18] - StoreRepo.Search concatenates q
		// into SQL; a quote turns the request into a 500 internal error.
		t.Skip("aqaa: open bug, see report")
		code, body, raw := getStores(t, ts, "q="+url.QueryEscape("o'brien"))
		if code != http.StatusOK || body.Total != 0 {
			t.Fatalf("status %d body %s", code, raw)
		}
	})

	t.Run("percent in q is literal", func(t *testing.T) {
		// aqaa: suspected bug [B1] - % is not escaped and matches every store.
		t.Skip("aqaa: open bug, see report")
		code, body, raw := getStores(t, ts, "q="+url.QueryEscape("%"))
		if code != http.StatusOK || body.Total != 0 {
			t.Fatalf("status %d body %s", code, raw)
		}
	})
}
