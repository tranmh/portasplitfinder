//go:build integration

package store

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// openSchemaDB connects to the run's Postgres (AQAA_PG_URL), creates a fresh
// schema named after the test, migrates it and returns a *DB bound to it. The
// schema is dropped on cleanup. Skips when AQAA_PG_URL is unset.
func openSchemaDB(t *testing.T) *DB {
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

	admin, err := Open(ctx, base)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Pool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := admin.Pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse AQAA_PG_URL: %v", err)
	}
	qs := u.Query()
	qs.Set("search_path", schema)
	u.RawQuery = qs.Encode()
	dsn := u.String()

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		c, err := Open(context.Background(), base)
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = c.Pool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})
	return db
}

func mustSource(t *testing.T, db *DB, key string, typ domain.SourceType) {
	t.Helper()
	if err := db.Sources.Upsert(context.Background(), domain.Source{Key: key, Name: key, Type: typ, BaseURL: "http://" + key}); err != nil {
		t.Fatalf("source %s: %v", key, err)
	}
}

func mustStore(t *testing.T, db *DB, s domain.Store) string {
	t.Helper()
	id, err := db.Stores.Upsert(context.Background(), s)
	if err != nil {
		t.Fatalf("store %s: %v", s.Name, err)
	}
	return id
}

func storeNames(ss []domain.Store) []string {
	out := []string{}
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}

// seedSearchStores inserts a fixed set of stores with plain ASCII names and
// cities so the expected city, name order is collation independent.
func seedSearchStores(t *testing.T, db *DB) {
	t.Helper()
	mustSource(t, db, "chain", domain.SourceStore)
	for _, s := range []domain.Store{
		{Name: "Alpha Markt", City: "Berlin", PLZ: "10115"},
		{Name: "Beta Haus", City: "Aachen", PLZ: "52062"},
		{Name: "Gamma BERLINER Laden", City: "Hamburg", PLZ: "20095"},
		{Name: "Delta", City: "Berlin", PLZ: "10117"},
		{Name: "100% Baumarkt", City: "Koeln", PLZ: "50667"},
		{Name: "O'Brien Shop", City: "Dublin", PLZ: "12345"},
		{Name: "Under_Score", City: "Zwickau", PLZ: "08056"},
	} {
		s.SourceKey = "chain"
		s.Lat, s.Lon = 50, 10
		mustStore(t, db, s)
	}
}

func TestIntegrationStoreRepoSearch(t *testing.T) {
	db := openSchemaDB(t)
	seedSearchStores(t, db)
	ctx := context.Background()

	all := []string{"Beta Haus", "Alpha Markt", "Delta", "O'Brien Shop", "Gamma BERLINER Laden", "100% Baumarkt", "Under_Score"}
	berlin := []string{"Alpha Markt", "Delta", "Gamma BERLINER Laden"}

	tests := []struct {
		name string
		q    string
		want []string
	}{
		{"empty q returns every store ordered by city, name", "", all},
		{"lower case matches city and name", "berlin", berlin},
		{"upper case matches city and name", "BERLIN", berlin},
		{"mixed case substring of a name", "hAuS", []string{"Beta Haus"}},
		{"substring of a city", "ach", []string{"Beta Haus"}},
		{"no match", "nowhere", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := db.Stores.Search(ctx, tc.q)
			if err != nil {
				t.Fatalf("search %q: %v", tc.q, err)
			}
			if names := storeNames(got); strings.Join(names, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("search %q = %v, want %v", tc.q, names, tc.want)
			}
		})
	}

	// B2: q is literal data; quotes and LIKE metacharacters must neither error
	// nor widen the match.
	literal := []struct {
		name string
		q    string
		want []string
	}{
		{"single quote", "'", []string{"O'Brien Shop"}},
		{"name with quote", "o'brien", []string{"O'Brien Shop"}},
		{"percent is literal", "%", []string{"100% Baumarkt"}},
		{"underscore is literal", "_", []string{"Under_Score"}},
		{"injection attempt", "' OR 1=1 OR name ILIKE '", []string{}},
	}
	for _, tc := range literal {
		t.Run("literal "+tc.name, func(t *testing.T) {
			got, err := db.Stores.Search(ctx, tc.q)
			if err != nil {
				t.Fatalf("search %q: %v", tc.q, err)
			}
			if names := storeNames(got); strings.Join(names, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("search %q = %v, want %v", tc.q, names, tc.want)
			}
		})
	}

	t.Run("stores table intact after hostile q", func(t *testing.T) {
		_, _ = db.Stores.Search(ctx, "x%'; DROP TABLE stores; --")
		got, err := db.Stores.List(ctx)
		if err != nil {
			t.Fatalf("list after hostile search: %v", err)
		}
		if len(got) != len(all) {
			t.Fatalf("stores after hostile search = %d, want %d", len(got), len(all))
		}
	})
}

func TestIntegrationSnapshotLatestForStore(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()

	p1 := "11111111-1111-1111-1111-111111111111"
	p2 := "22222222-2222-2222-2222-222222222222"
	for _, p := range []domain.Product{{ID: p1, Name: "P1", Model: "M1"}, {ID: p2, Name: "P2", Model: "M2"}} {
		if err := db.Products.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	mustSource(t, db, "s1", domain.SourceStore)
	mustSource(t, db, "s2", domain.SourceStore)
	mustSource(t, db, "web", domain.SourceOnline)
	x := mustStore(t, db, domain.Store{SourceKey: "s1", Name: "X", PLZ: "10115", City: "Berlin", Lat: 52.5, Lon: 13.4})
	y := mustStore(t, db, domain.Store{SourceKey: "s1", Name: "Y", PLZ: "10115", City: "Berlin", Lat: 52.5, Lon: 13.4})
	empty := mustStore(t, db, domain.Store{SourceKey: "s2", Name: "Empty", PLZ: "10115", City: "Berlin", Lat: 52.5, Lon: 13.4})

	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ip := func(i int) *int { return &i }
	ins := func(src string, store *string, pid, u string, inStock bool, at time.Time) {
		t.Helper()
		if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{
			SourceKey: src, StoreID: store, ProductID: pid, InStock: inStock,
			PriceCents: ip(100), Currency: "EUR", URL: u, CheckedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Inserted out of chronological order on purpose.
	ins("s1", &x, p1, "x-s1-p1-old", false, t0)
	ins("s1", &x, p1, "x-s1-p1-new", true, t0.Add(2*time.Hour))
	ins("s1", &x, p1, "x-s1-p1-mid", false, t0.Add(1*time.Hour))
	ins("s2", &x, p1, "x-s2-p1", true, t0)
	ins("s1", &x, p2, "x-s1-p2-new", false, t0.Add(3*time.Hour))
	ins("s1", &x, p2, "x-s1-p2-old", true, t0.Add(-time.Hour))
	ins("s1", &y, p1, "y-s1-p1", true, t0.Add(5*time.Hour))
	ins("web", nil, p1, "online-p1", true, t0.Add(6*time.Hour))

	urls := func(ss []domain.AvailabilitySnapshot) []string {
		out := []string{}
		for _, s := range ss {
			out = append(out, s.URL)
		}
		sort.Strings(out)
		return out
	}

	t.Run("latest per source and product for the store", func(t *testing.T) {
		got, err := db.Snapshots.LatestForStore(ctx, x)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"x-s1-p1-new", "x-s1-p2-new", "x-s2-p1"}
		if strings.Join(urls(got), "|") != strings.Join(want, "|") {
			t.Fatalf("latest for store X = %v, want %v", urls(got), want)
		}
		for _, s := range got {
			if s.StoreID == nil || *s.StoreID != x {
				t.Fatalf("snapshot from another store: %+v", s)
			}
			if s.URL == "x-s1-p1-new" && (!s.InStock || !s.CheckedAt.Equal(t0.Add(2*time.Hour))) {
				t.Fatalf("latest s1/p1 fields wrong: %+v", s)
			}
		}
	})

	t.Run("other store sees only its own", func(t *testing.T) {
		got, err := db.Snapshots.LatestForStore(ctx, y)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(urls(got), "|") != "y-s1-p1" {
			t.Fatalf("latest for store Y = %v", urls(got))
		}
	})

	t.Run("store without snapshots", func(t *testing.T) {
		got, err := db.Snapshots.LatestForStore(ctx, empty)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("expected no snapshots, got %+v", got)
		}
	})

	t.Run("unknown store id", func(t *testing.T) {
		got, err := db.Snapshots.LatestForStore(ctx, "99999999-9999-9999-9999-999999999999")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("expected no snapshots, got %+v", got)
		}
	})

	t.Run("adapter delegates to the repositories", func(t *testing.T) {
		viaAdapter, err := db.LatestSnapshotsForStore(ctx, x)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(urls(viaAdapter), "|") != "x-s1-p1-new|x-s1-p2-new|x-s2-p1" {
			t.Fatalf("adapter LatestSnapshotsForStore = %v", urls(viaAdapter))
		}
	})
}

func TestIntegrationAdapterSearchStores(t *testing.T) {
	db := openSchemaDB(t)
	seedSearchStores(t, db)
	ctx := context.Background()

	for _, q := range []string{"", "berlin", "Haus", "nowhere"} {
		t.Run("q="+q, func(t *testing.T) {
			viaAdapter, err := db.SearchStores(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			viaRepo, err := db.Stores.Search(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(storeNames(viaAdapter), "|") != strings.Join(storeNames(viaRepo), "|") {
				t.Fatalf("adapter %v != repo %v", storeNames(viaAdapter), storeNames(viaRepo))
			}
		})
	}
	t.Run("returns full store rows", func(t *testing.T) {
		got, err := db.SearchStores(ctx, "delta")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID == "" || got[0].SourceKey != "chain" || got[0].PLZ != "10117" || got[0].City != "Berlin" {
			t.Fatalf("unexpected rows: %+v", got)
		}
	})
}
