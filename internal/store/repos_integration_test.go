//go:build integration

package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// openSchemaDB connects to the run's Postgres (AQAA_PG_URL), creates a fresh
// schema for this test, migrates it and returns a *DB bound to it. The schema
// is dropped on cleanup, so tests never touch the app's own tables.
func openSchemaDB(t *testing.T) *DB {
	t.Helper()
	base := os.Getenv("AQAA_PG_URL")
	if base == "" {
		t.Skip("AQAA_PG_URL not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("aqaa_store_%d", time.Now().UnixNano())

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), base)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		_, _ = c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	dsn := u.String()

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func intp(i int) *int       { return &i }
func strp(s string) *string { return &s }

func TestIntegrationSubscriptionRepoLifecycle(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()
	pid, _ := seedBasics(t, db)

	t.Run("create with product and without product", func(t *testing.T) {
		idAny, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "any@b.de", PLZ: "10115", RadiusKm: 50, ConfirmToken: "c-any", UnsubToken: "u-any"})
		if err != nil || idAny == "" {
			t.Fatalf("create any: %q %v", idAny, err)
		}
		idP, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "p@b.de", PLZ: "80331", RadiusKm: 500, ProductID: &pid, ConfirmToken: "c-p", UnsubToken: "u-p"})
		if err != nil || idP == "" || idP == idAny {
			t.Fatalf("create p: %q %v", idP, err)
		}
		s, ok, err := db.Subscriptions.GetByConfirmToken(ctx, "c-any")
		if err != nil || !ok {
			t.Fatalf("get any: %v %v", ok, err)
		}
		if s.ID != idAny || s.ProductID != nil || s.ConfirmedAt != nil || s.UnsubbedAt != nil || s.RadiusKm != 50 {
			t.Fatalf("unexpected any sub: %+v", s)
		}
		s, ok, err = db.Subscriptions.GetByUnsubToken(ctx, "u-p")
		if err != nil || !ok || s.ProductID == nil || *s.ProductID != pid || s.PLZ != "80331" || s.RadiusKm != 500 {
			t.Fatalf("get p: %v %v %+v", ok, err, s)
		}
	})

	t.Run("invalid product uuid is an error", func(t *testing.T) {
		_, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "x@b.de", PLZ: "10115", RadiusKm: 50, ProductID: strp("not-a-uuid"), ConfirmToken: "c-x", UnsubToken: "u-x"})
		if err == nil {
			t.Fatal("expected error for non-uuid product id")
		}
		if _, ok, _ := db.Subscriptions.GetByConfirmToken(ctx, "c-x"); ok {
			t.Fatal("failed create must not leave a row behind")
		}
	})

	t.Run("unknown product uuid is an error", func(t *testing.T) {
		_, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "y@b.de", PLZ: "10115", RadiusKm: 50, ProductID: strp("99999999-9999-9999-9999-999999999999"), ConfirmToken: "c-y", UnsubToken: "u-y"})
		if err == nil {
			t.Fatal("expected FK error for unknown product id")
		}
	})

	t.Run("unknown tokens give ok=false", func(t *testing.T) {
		email, ok, err := db.Subscriptions.ConfirmByToken(ctx, "nope")
		if err != nil || ok || email != "" {
			t.Fatalf("confirm unknown: %q %v %v", email, ok, err)
		}
		ok, err = db.Subscriptions.UnsubByToken(ctx, "nope")
		if err != nil || ok {
			t.Fatalf("unsub unknown: %v %v", ok, err)
		}
		// The confirm token must not work as an unsub token and vice versa.
		if ok, _ := db.Subscriptions.UnsubByToken(ctx, "c-any"); ok {
			t.Fatal("confirm token accepted as unsub token")
		}
		if _, ok, _ := db.Subscriptions.ConfirmByToken(ctx, "u-any"); ok {
			t.Fatal("unsub token accepted as confirm token")
		}
	})

	t.Run("confirm is idempotent and keeps the first time", func(t *testing.T) {
		email, ok, err := db.Subscriptions.ConfirmByToken(ctx, "c-any")
		if err != nil || !ok || email != "any@b.de" {
			t.Fatalf("confirm: %q %v %v", email, ok, err)
		}
		first, _, _ := db.Subscriptions.GetByConfirmToken(ctx, "c-any")
		if first.ConfirmedAt == nil {
			t.Fatal("confirmed_at not set")
		}
		email, ok, err = db.Subscriptions.ConfirmByToken(ctx, "c-any")
		if err != nil || !ok || email != "any@b.de" {
			t.Fatalf("second confirm: %q %v %v", email, ok, err)
		}
		second, _, _ := db.Subscriptions.GetByConfirmToken(ctx, "c-any")
		if second.ConfirmedAt == nil || !second.ConfirmedAt.Equal(*first.ConfirmedAt) {
			t.Fatalf("confirmed_at changed: %v -> %v", first.ConfirmedAt, second.ConfirmedAt)
		}
	})

	t.Run("list confirmed ordered by created_at and excludes unsubscribed", func(t *testing.T) {
		if _, ok, err := db.Subscriptions.ConfirmByToken(ctx, "c-p"); err != nil || !ok {
			t.Fatalf("confirm p: %v %v", ok, err)
		}
		list, err := db.Subscriptions.ListConfirmed(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || list[0].Email != "any@b.de" || list[1].Email != "p@b.de" {
			t.Fatalf("list confirmed: %+v", list)
		}
		for _, s := range list {
			if !s.Confirmed() || s.ConfirmToken == "" || s.UnsubToken == "" {
				t.Fatalf("bad listed sub: %+v", s)
			}
		}

		ok, err := db.Subscriptions.UnsubByToken(ctx, "u-any")
		if err != nil || !ok {
			t.Fatalf("unsub: %v %v", ok, err)
		}
		first, _, _ := db.Subscriptions.GetByUnsubToken(ctx, "u-any")
		ok, err = db.Subscriptions.UnsubByToken(ctx, "u-any")
		if err != nil || !ok {
			t.Fatalf("second unsub: %v %v", ok, err)
		}
		second, _, _ := db.Subscriptions.GetByUnsubToken(ctx, "u-any")
		if first.UnsubbedAt == nil || second.UnsubbedAt == nil || !second.UnsubbedAt.Equal(*first.UnsubbedAt) {
			t.Fatalf("unsubbed_at not stable: %v -> %v", first.UnsubbedAt, second.UnsubbedAt)
		}

		list, err = db.Subscriptions.ListConfirmed(ctx)
		if err != nil || len(list) != 1 || list[0].Email != "p@b.de" {
			t.Fatalf("list after unsub: %v %+v", err, list)
		}

		// Confirming an unsubscribed subscription again does not resurrect it.
		if _, ok, err := db.Subscriptions.ConfirmByToken(ctx, "c-any"); err != nil || !ok {
			t.Fatalf("reconfirm: %v %v", ok, err)
		}
		list, _ = db.Subscriptions.ListConfirmed(ctx)
		if len(list) != 1 {
			t.Fatalf("unsubscribed sub reappeared: %+v", list)
		}
	})

	t.Run("unconfirmed subscriptions are not listed", func(t *testing.T) {
		if _, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "late@b.de", PLZ: "10115", RadiusKm: 1, ConfirmToken: "c-late", UnsubToken: "u-late"}); err != nil {
			t.Fatal(err)
		}
		list, _ := db.Subscriptions.ListConfirmed(ctx)
		for _, s := range list {
			if s.Email == "late@b.de" {
				t.Fatal("unconfirmed subscription listed")
			}
		}
	})
}

func TestIntegrationSnapshotRepoLatest(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()
	pid, sid := seedBasics(t, db)
	pid2 := "22222222-2222-2222-2222-222222222222"
	if err := db.Products.Upsert(ctx, domain.Product{ID: pid2, Name: "Daikin", Model: "DPS"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "obi", Name: "OBI", Type: domain.SourceStore, BaseURL: "http://o"}); err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ins := func(s domain.AvailabilitySnapshot) {
		t.Helper()
		if err := db.Snapshots.Insert(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	// mock/sid/pid: the newest row is inserted first, so ordering must use
	// checked_at and not insertion order.
	ins(domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid, InStock: true, Quantity: intp(3), PriceCents: intp(300), Currency: "EUR", URL: "mock-new", CheckedAt: t0.Add(2 * time.Hour)})
	ins(domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid, InStock: false, PriceCents: intp(100), Currency: "EUR", URL: "mock-old", CheckedAt: t0})
	ins(domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid, InStock: false, Currency: "EUR", URL: "mock-mid", CheckedAt: t0.Add(time.Hour)})
	// Same store and product, different source: its own key.
	ins(domain.AvailabilitySnapshot{SourceKey: "obi", StoreID: &sid, ProductID: pid, InStock: false, Currency: "EUR", URL: "obi-old", CheckedAt: t0})
	ins(domain.AvailabilitySnapshot{SourceKey: "obi", StoreID: &sid, ProductID: pid, InStock: true, Currency: "EUR", URL: "obi-new", CheckedAt: t0.Add(time.Minute)})
	// Same store and source, other product.
	ins(domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &sid, ProductID: pid2, InStock: true, Currency: "EUR", URL: "mock-p2", CheckedAt: t0})
	// Online (no store) listings, two rows.
	ins(domain.AvailabilitySnapshot{SourceKey: "amazon", ProductID: pid, InStock: true, Currency: "EUR", URL: "amazon-old", CheckedAt: t0})
	ins(domain.AvailabilitySnapshot{SourceKey: "amazon", ProductID: pid, InStock: false, Currency: "EUR", URL: "amazon-new", CheckedAt: t0.Add(time.Hour)})

	latest, err := db.Snapshots.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.AvailabilitySnapshot{}
	for _, s := range latest {
		key := s.SourceKey + "|" + deref(s.StoreID) + "|" + s.ProductID
		if _, dup := got[key]; dup {
			t.Fatalf("duplicate key %s in latest", key)
		}
		got[key] = s
	}
	want := map[string]string{
		"mock|" + sid + "|" + pid:  "mock-new",
		"obi|" + sid + "|" + pid:   "obi-new",
		"mock|" + sid + "|" + pid2: "mock-p2",
		"amazon||" + pid:           "amazon-new",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d keys, got %d: %+v", len(want), len(got), latest)
	}
	for k, u := range want {
		s, ok := got[k]
		if !ok || s.URL != u {
			t.Fatalf("key %s: want %s, got %+v", k, u, s)
		}
	}
	newest := got["mock|"+sid+"|"+pid]
	if !newest.InStock || newest.Quantity == nil || *newest.Quantity != 3 || newest.PriceCents == nil || *newest.PriceCents != 300 || newest.Currency != "EUR" {
		t.Fatalf("fields not round-tripped: %+v", newest)
	}
	if !newest.CheckedAt.Equal(t0.Add(2 * time.Hour)) {
		t.Fatalf("checked_at round-trip: %v", newest.CheckedAt)
	}
	if newest.ID == "" {
		t.Fatal("snapshot id empty")
	}
	if a := got["amazon||"+pid]; a.StoreID != nil || a.Quantity != nil || a.PriceCents != nil {
		t.Fatalf("online nulls not preserved: %+v", a)
	}

	t.Run("latest for key", func(t *testing.T) {
		cases := []struct {
			source, store, product, url string
			ok                          bool
		}{
			{"mock", sid, pid, "mock-new", true},
			{"obi", sid, pid, "obi-new", true},
			{"amazon", "", pid, "amazon-new", true},
			{"amazon", "", pid2, "", false},
			{"obi", sid, pid2, "", false},
			{"mock", "", pid, "", false},
		}
		for _, c := range cases {
			s, ok, err := db.Snapshots.LatestForKey(ctx, c.source, c.store, c.product)
			if err != nil || ok != c.ok || s.URL != c.url {
				t.Errorf("%s/%s/%s: got %v %v %+v", c.source, c.store, c.product, ok, err, s)
			}
		}
	})

	t.Run("zero checked_at defaults to now and becomes latest", func(t *testing.T) {
		ins(domain.AvailabilitySnapshot{SourceKey: "obi", StoreID: &sid, ProductID: pid, InStock: false, Currency: "EUR", URL: "obi-now"})
		s, ok, err := db.Snapshots.LatestForKey(ctx, "obi", sid, pid)
		if err != nil || !ok || s.URL != "obi-now" || s.CheckedAt.IsZero() {
			t.Fatalf("got %v %v %+v", ok, err, s)
		}
	})

	t.Run("empty table gives no rows", func(t *testing.T) {
		empty := openSchemaDB(t)
		l, err := empty.Snapshots.Latest(ctx)
		if err != nil || len(l) != 0 {
			t.Fatalf("got %v %+v", err, l)
		}
	})
}

func TestIntegrationStoreRepoUpsertList(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "mock", Name: "Mock", Type: domain.SourceStore, BaseURL: "http://x"}); err != nil {
		t.Fatal(err)
	}

	fixedID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	idB, err := db.Stores.Upsert(ctx, domain.Store{ID: fixedID, SourceKey: "mock", Name: "OBI Berlin", Street: strp("Invalidenstraße 1"), PLZ: "10115", City: "Berlin", Lat: 52.5323, Lon: 13.3846})
	if err != nil || idB != fixedID {
		t.Fatalf("upsert with fixed id: %q %v", idB, err)
	}
	idM, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "mock", Name: "OBI München", PLZ: "80331", City: "München", Lat: 48.1372, Lon: 11.5755})
	if err != nil || idM == "" || idM == idB {
		t.Fatalf("upsert generated id: %q %v", idM, err)
	}
	idA, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "mock", Name: "A-Markt Berlin", PLZ: "10117", City: "Berlin", Lat: -33.8688, Lon: -151.2093})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("coordinates and nullable street round-trip", func(t *testing.T) {
		s, ok, err := db.Stores.Get(ctx, idB)
		if err != nil || !ok {
			t.Fatalf("get: %v %v", ok, err)
		}
		if s.Lat != 52.5323 || s.Lon != 13.3846 || s.Street == nil || *s.Street != "Invalidenstraße 1" || s.SourceKey != "mock" || s.PLZ != "10115" {
			t.Fatalf("berlin: %+v", s)
		}
		s, _, _ = db.Stores.Get(ctx, idA)
		if s.Lat != -33.8688 || s.Lon != -151.2093 || s.Street != nil {
			t.Fatalf("negative coords / nil street: %+v", s)
		}
	})

	t.Run("list ordered by city then name", func(t *testing.T) {
		list, err := db.Stores.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, s := range list {
			names = append(names, s.Name)
		}
		want := []string{"A-Markt Berlin", "OBI Berlin", "OBI München"}
		if fmt.Sprint(names) != fmt.Sprint(want) {
			t.Fatalf("order: %v, want %v", names, want)
		}
	})

	t.Run("re-upsert by source and name keeps id and updates fields", func(t *testing.T) {
		id, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "mock", Name: "OBI München", Street: strp("Neu 2"), PLZ: "80333", City: "München", Lat: 48.15, Lon: 11.56})
		if err != nil || id != idM {
			t.Fatalf("id changed: %q -> %q (%v)", idM, id, err)
		}
		// A different supplied id does not replace the existing row's id.
		id, err = db.Stores.Upsert(ctx, domain.Store{ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", SourceKey: "mock", Name: "OBI Berlin", PLZ: "10115", City: "Berlin", Lat: 52.5323, Lon: 13.3846})
		if err != nil || id != idB {
			t.Fatalf("supplied id replaced existing: %q (%v)", id, err)
		}
		s, _, _ := db.Stores.Get(ctx, idM)
		if s.PLZ != "80333" || s.Lat != 48.15 || s.Lon != 11.56 || s.Street == nil || *s.Street != "Neu 2" {
			t.Fatalf("fields not updated: %+v", s)
		}
		list, _ := db.Stores.List(ctx)
		if len(list) != 3 {
			t.Fatalf("upsert duplicated rows: %d", len(list))
		}
	})

	t.Run("unknown store and unknown source", func(t *testing.T) {
		if _, ok, err := db.Stores.Get(ctx, "99999999-9999-9999-9999-999999999999"); err != nil || ok {
			t.Fatalf("get unknown: %v %v", ok, err)
		}
		if _, err := db.Stores.Upsert(ctx, domain.Store{SourceKey: "nosuch", Name: "X", PLZ: "10115", City: "B"}); err == nil {
			t.Fatal("expected FK error for unknown source")
		}
	})
}

func TestIntegrationSourceRepo(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()

	for _, s := range []domain.Source{
		{Key: "obi", Name: "OBI", Type: domain.SourceStore, BaseURL: "http://o"},
		{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline, BaseURL: "http://a"},
		{Key: "mock", Name: "Mock", Type: domain.SourceStore, BaseURL: "http://m", LastStatus: domain.StatusOK},
	} {
		if err := db.Sources.Upsert(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	list, err := db.Sources.List(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if list[0].Key != "amazon" || list[1].Key != "mock" || list[2].Key != "obi" {
		t.Fatalf("order by key: %+v", list)
	}
	if list[0].LastStatus != domain.StatusNever || list[0].LastCheckedAt != nil || list[0].LastError != nil || list[0].Type != domain.SourceOnline {
		t.Fatalf("defaults: %+v", list[0])
	}
	if list[1].LastStatus != domain.StatusOK {
		t.Fatalf("initial status honoured: %+v", list[1])
	}

	checked := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := db.Sources.UpdateStatus(ctx, "obi", checked, domain.StatusError, strp("timeout")); err != nil {
		t.Fatal(err)
	}
	s, ok, err := db.Sources.Get(ctx, "obi")
	if err != nil || !ok || s.LastStatus != domain.StatusError || s.LastError == nil || *s.LastError != "timeout" || s.LastCheckedAt == nil || !s.LastCheckedAt.Equal(checked) {
		t.Fatalf("after update status: %v %v %+v", ok, err, s)
	}

	// Re-seeding updates metadata but keeps the scrape status.
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "obi", Name: "OBI Neu", Type: domain.SourceOnline, BaseURL: "http://o2", LastStatus: domain.StatusNever}); err != nil {
		t.Fatal(err)
	}
	s, _, _ = db.Sources.Get(ctx, "obi")
	if s.Name != "OBI Neu" || s.Type != domain.SourceOnline || s.BaseURL != "http://o2" {
		t.Fatalf("metadata not updated: %+v", s)
	}
	if s.LastStatus != domain.StatusError || s.LastError == nil || *s.LastError != "timeout" || s.LastCheckedAt == nil {
		t.Fatalf("scrape status overwritten on upsert: %+v", s)
	}

	// Recovering clears the error.
	if err := db.Sources.UpdateStatus(ctx, "obi", checked.Add(time.Minute), domain.StatusOK, nil); err != nil {
		t.Fatal(err)
	}
	s, _, _ = db.Sources.Get(ctx, "obi")
	if s.LastStatus != domain.StatusOK || s.LastError != nil {
		t.Fatalf("recovery: %+v", s)
	}

	if _, ok, err := db.Sources.Get(ctx, "nosuch"); err != nil || ok {
		t.Fatalf("get unknown: %v %v", ok, err)
	}
	if err := db.Sources.Upsert(ctx, domain.Source{Key: "bad", Name: "Bad", Type: "shop", BaseURL: "x"}); err == nil {
		t.Fatal("expected check constraint error for invalid type")
	}
	if err := db.Sources.UpdateStatus(ctx, "obi", checked, "weird", nil); err == nil {
		t.Fatal("expected check constraint error for invalid status")
	}
}

func TestIntegrationNotificationRepoDedupe(t *testing.T) {
	db := openSchemaDB(t)
	ctx := context.Background()
	pid, sid := seedBasics(t, db)

	sub1, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "a@b.de", PLZ: "10115", RadiusKm: 50, ConfirmToken: "c1", UnsubToken: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	sub2, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "c@d.de", PLZ: "10115", RadiusKm: 50, ConfirmToken: "c2", UnsubToken: "u2"})
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name                   string
		sub, store, dedupe     string
		wantInserted           bool
	}{
		{"first claim", sub1, sid, "mock:" + sid + ":" + pid, true},
		{"same key again", sub1, sid, "mock:" + sid + ":" + pid, false},
		{"same key other store id still deduped", sub1, "", "mock:" + sid + ":" + pid, false},
		{"other subscription same key", sub2, sid, "mock:" + sid + ":" + pid, true},
		{"same subscription other key", sub1, sid, "mock:" + sid + ":" + pid + ":2", true},
		{"online listing (no store)", sub1, "", "amazon::" + pid, true},
		{"online listing again", sub1, "", "amazon::" + pid, false},
	}
	for _, st := range steps {
		ins, err := db.Notifications.Record(ctx, st.sub, "mock", st.store, pid, st.dedupe)
		if err != nil || ins != st.wantInserted {
			t.Fatalf("%s: inserted=%v err=%v, want inserted=%v", st.name, ins, err, st.wantInserted)
		}
	}

	var n int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("notification rows: %d %v", n, err)
	}

	exists := []struct {
		sub, dedupe string
		want        bool
	}{
		{sub1, "mock:" + sid + ":" + pid, true},
		{sub2, "mock:" + sid + ":" + pid, true},
		{sub2, "amazon::" + pid, false},
		{sub1, "never", false},
	}
	for _, e := range exists {
		got, err := db.Notifications.Exists(ctx, e.sub, e.dedupe)
		if err != nil || got != e.want {
			t.Fatalf("exists(%s,%s) = %v %v, want %v", e.sub, e.dedupe, got, err, e.want)
		}
	}

	if _, err := db.Notifications.Record(ctx, "99999999-9999-9999-9999-999999999999", "mock", sid, pid, "x"); err == nil {
		t.Fatal("expected FK error for unknown subscription")
	}

	// Deleting a subscription cascades to its notifications.
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, sub1); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Notifications.Exists(ctx, sub1, "mock:"+sid+":"+pid); ok {
		t.Fatal("notifications not cascaded on subscription delete")
	}
}
