//go:build integration

package jobs

import (
	"context"
	"os"
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

func startPostgres(t *testing.T) (*store.DB, func()) {
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
	return db, func() { db.Close(); _ = c.Terminate(ctx) }
}

func TestIntegration_PollerOnce_TransitionAndMail(t *testing.T) {
	db, cleanup := startPostgres(t)
	defer cleanup()
	ctx := context.Background()

	// Seed catalogue from the mock so stores/products/sources exist.
	m := source.NewMock()
	if err := db.Seed(ctx, m.Products(), m.SeedSources(), m.Stores()); err != nil {
		t.Fatal(err)
	}

	mailer := notify.NewFakeMailer()

	// First poll with the transition OFF: establishes baseline snapshots (Munich
	// out of stock). No mails expected (everything is first-seen).
	pOff := New(Config{Sources: []source.Source{source.NewMockWithTransition(false)}, Store: db, Mailer: mailer, PublicBaseURL: "http://x"})
	if _, err := pOff.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if mailer.Count() != 0 {
		t.Fatalf("expected no mails on baseline poll, got %d", mailer.Count())
	}

	// A confirmed Munich subscriber.
	subID, err := db.Subscriptions.Create(ctx, domain.Subscription{Email: "muc@x.de", PLZ: "80331", RadiusKm: 25, ConfirmToken: "ct", UnsubToken: "ut"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.Subscriptions.ConfirmByToken(ctx, "ct"); err != nil || !ok {
		t.Fatalf("confirm: %v %v", err, ok)
	}
	_ = subID

	// Second poll with transition ON: Munich/Midea flips out->in stock.
	pOn := New(Config{Sources: []source.Source{source.NewMockWithTransition(true)}, Store: db, Mailer: mailer, PublicBaseURL: "http://x"})
	sent, err := pOn.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sent == 0 {
		t.Fatal("expected at least one availability mail from the transition")
	}

	// The Munich subscriber should have a mail.
	gotMuc := false
	for _, msg := range mailer.Messages() {
		if msg.To == "muc@x.de" {
			gotMuc = true
		}
	}
	if !gotMuc {
		t.Fatal("Munich subscriber received no mail")
	}

	// Latest snapshots should reflect Munich now in stock.
	latest, err := db.Snapshots.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) == 0 {
		t.Fatal("no latest snapshots")
	}

	// Re-poll with transition still ON -> no NEW transition -> dedupe/no new mail.
	before := mailer.Count()
	if _, err := pOn.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if mailer.Count() != before {
		t.Fatalf("expected no new mails on stable re-poll, before=%d after=%d", before, mailer.Count())
	}
}
