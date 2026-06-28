//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
	"github.com/tranmh/portasplitfinder/internal/store"
)

func startStack(t *testing.T) (*store.DB, *httptest.Server, *notify.FakeMailer, func()) {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run integration tests")
	}
	ctx := context.Background()
	c, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("portasplit"),
		postgres.WithUsername("portasplit"),
		postgres.WithPassword("portasplit"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	mailer := notify.NewFakeMailer()
	srv := New(Config{Store: db, Mailer: mailer, PublicBaseURL: "http://localhost:8080"})
	ts := httptest.NewServer(srv.Router())
	cleanup := func() { ts.Close(); db.Close(); _ = c.Terminate(ctx) }
	return db, ts, mailer, cleanup
}

func seedStack(t *testing.T, db *store.DB) {
	t.Helper()
	ctx := context.Background()
	m := source.NewMock()
	if err := db.Seed(ctx, m.Products(), m.SeedSources(), m.Stores()); err != nil {
		t.Fatal(err)
	}
	// Run one baseline poll (no mailer needed via API DB) to populate snapshots.
	// We write snapshots directly here for determinism.
	ip := func(i int) *int { return &i }
	storeBerlin := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	storeMunich := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	pid := source.MockProductMideaID
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &storeBerlin, ProductID: pid, InStock: true, Quantity: ip(4), PriceCents: ip(49900), Currency: "EUR", URL: "u-berlin"}))
	must(db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &storeMunich, ProductID: pid, InStock: true, Quantity: ip(3), PriceCents: ip(50900), Currency: "EUR", URL: "u-munich"}))
	must(db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "amazon", ProductID: pid, InStock: true, PriceCents: ip(47900), Currency: "EUR", URL: "u-amazon"}))
}

func getJSON(t *testing.T, url string, out any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestIntegration_Health(t *testing.T) {
	_, ts, _, cleanup := startStack(t)
	defer cleanup()
	if code := getJSON(t, ts.URL+"/api/health", nil); code != 200 {
		t.Fatalf("health status %d", code)
	}
}

func TestIntegration_AvailabilityRadiusFilter(t *testing.T) {
	db, ts, _, cleanup := startStack(t)
	defer cleanup()
	seedStack(t, db)

	// Centered on Frankfurt (60311): with a small radius no seeded store is in
	// range (nearest seeded store is Munich ~304km). 0 stores expected, but
	// online listings still returned.
	var resp AvailabilityResponse
	if code := getJSON(t, ts.URL+"/api/availability?plz=60311&radius=50", &resp); code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(resp.Stores) != 0 {
		t.Fatalf("expected 0 stores within 50km of Frankfurt, got %d", len(resp.Stores))
	}
	if len(resp.Online) == 0 {
		t.Fatal("expected online listings")
	}

	// radius 350 -> only Munich (~304km). Berlin (~423km) excluded.
	resp = AvailabilityResponse{}
	if code := getJSON(t, ts.URL+"/api/availability?plz=60311&radius=350", &resp); code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(resp.Stores) != 1 || resp.Stores[0].Store.City != "München" {
		t.Fatalf("radius 350 should give only Munich, got %+v", resp.Stores)
	}

	// radius 450 -> Munich + Berlin, Munich (nearer) first.
	resp = AvailabilityResponse{}
	if code := getJSON(t, ts.URL+"/api/availability?plz=60311&radius=450", &resp); code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(resp.Stores) != 2 {
		t.Fatalf("expected 2 stores within 450km, got %d", len(resp.Stores))
	}
	if resp.Stores[0].Store.City != "München" {
		t.Fatalf("nearest-first broken: first = %s", resp.Stores[0].Store.City)
	}
}

func TestIntegration_SubscriptionFlow(t *testing.T) {
	db, ts, mailer, cleanup := startStack(t)
	defer cleanup()
	seedStack(t, db)
	ctx := context.Background()

	body := strings.NewReader(`{"email":"a@b.de","plz":"10115","radiusKm":50}`)
	resp, err := http.Post(ts.URL+"/api/subscriptions", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 202 {
		t.Fatalf("subscribe status %d", resp.StatusCode)
	}
	resp.Body.Close()
	if mailer.Count() != 1 {
		t.Fatalf("expected 1 confirm mail, got %d", mailer.Count())
	}

	// Fetch the confirm token from the DB (via subscriptions repo).
	subs, err := db.Subscriptions.ListConfirmed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Fatalf("should be 0 confirmed before confirm, got %d", len(subs))
	}

	// Extract token from the confirm email content.
	msg := mailer.Messages()[0]
	token := extractToken(msg.HTML, "/api/subscriptions/confirm?token=")
	if token == "" {
		t.Fatal("could not extract confirm token from email")
	}

	var cresp map[string]string
	if code := getJSON(t, ts.URL+"/api/subscriptions/confirm?token="+token, &cresp); code != 200 {
		t.Fatalf("confirm status %d", code)
	}
	if cresp["status"] != "confirmed" {
		t.Fatalf("confirm body %v", cresp)
	}

	subs, _ = db.Subscriptions.ListConfirmed(ctx)
	if len(subs) != 1 {
		t.Fatalf("expected 1 confirmed sub, got %d", len(subs))
	}

	// Unsubscribe via token in the same email.
	unsub := extractToken(msg.HTML, "/api/subscriptions/unsubscribe?token=")
	if unsub == "" {
		t.Fatal("could not extract unsub token")
	}
	var uresp map[string]string
	if code := getJSON(t, ts.URL+"/api/subscriptions/unsubscribe?token="+unsub, &uresp); code != 200 {
		t.Fatalf("unsub status %d", code)
	}
	subs, _ = db.Subscriptions.ListConfirmed(ctx)
	if len(subs) != 0 {
		t.Fatalf("expected 0 confirmed after unsub, got %d", len(subs))
	}
}

// extractToken pulls the token query value following the given path prefix.
func extractToken(html, prefix string) string {
	i := strings.Index(html, prefix)
	if i < 0 {
		return ""
	}
	rest := html[i+len(prefix):]
	end := strings.IndexAny(rest, `"'< `)
	if end < 0 {
		return rest
	}
	return rest[:end]
}
