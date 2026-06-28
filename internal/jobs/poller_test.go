package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
)

// fakeStore implements jobs.Store in-memory.
type fakeStore struct {
	prev          map[string]domain.AvailabilitySnapshot // key -> latest BEFORE this poll
	inserted      []domain.AvailabilitySnapshot
	subs          []domain.Subscription
	stores        map[string]domain.Store
	products      map[string]string
	notifications map[string]bool // subID|dedupeKey
	statusUpdates int
	productList   []domain.Product
}

func key(src, store, prod string) string { return src + "|" + store + "|" + prod }

func newFakeStore() *fakeStore {
	return &fakeStore{
		prev:          map[string]domain.AvailabilitySnapshot{},
		stores:        map[string]domain.Store{},
		products:      map[string]string{},
		notifications: map[string]bool{},
	}
}

func (f *fakeStore) ListProducts(context.Context) ([]domain.Product, error) {
	return f.productList, nil
}
func (f *fakeStore) UpsertSource(_ context.Context, _ domain.Source) error {
	return nil
}

func (f *fakeStore) UpsertStore(_ context.Context, s domain.Store) (string, error) {
	f.stores[s.ID] = s
	return s.ID, nil
}
func (f *fakeStore) InsertSnapshot(_ context.Context, snap domain.AvailabilitySnapshot) error {
	f.inserted = append(f.inserted, snap)
	return nil
}
func (f *fakeStore) LatestForKey(_ context.Context, src, store, prod string) (domain.AvailabilitySnapshot, bool, error) {
	s, ok := f.prev[key(src, store, prod)]
	return s, ok, nil
}
func (f *fakeStore) UpdateSourceStatus(_ context.Context, _ string, _ time.Time, _ domain.LastStatus, _ *string) error {
	f.statusUpdates++
	return nil
}
func (f *fakeStore) ListConfirmedSubscriptions(context.Context) ([]domain.Subscription, error) {
	return f.subs, nil
}
func (f *fakeStore) GetStore(_ context.Context, id string) (domain.Store, bool, error) {
	s, ok := f.stores[id]
	return s, ok, nil
}
func (f *fakeStore) GetProductName(_ context.Context, id string) (string, bool, error) {
	n, ok := f.products[id]
	return n, ok, nil
}
func (f *fakeStore) RecordNotification(_ context.Context, subID, _, _, _, dedupe string) (bool, error) {
	k := subID + "|" + dedupe
	if f.notifications[k] {
		return false, nil
	}
	f.notifications[k] = true
	return true, nil
}

func confirmedSub(id, email, plz string, radius int, productID *string) domain.Subscription {
	now := time.Now()
	return domain.Subscription{ID: id, Email: email, PLZ: plz, RadiusKm: radius, ProductID: productID, UnsubToken: "u-" + id, ConfirmedAt: &now}
}

func TestPollOnce_TransitionFiresEmail(t *testing.T) {
	fs := newFakeStore()
	// The mock Munich/Midea listing was previously out of stock.
	fs.prev[key("mock", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", source.MockProductMideaID)] =
		domain.AvailabilitySnapshot{InStock: false}
	// Pre-seed the Munich store so radius matching can find it.
	fs.stores["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"] = domain.Store{
		ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Name: "OBI München", City: "München", Lat: 48.1372, Lon: 11.5755,
	}
	fs.products[source.MockProductMideaID] = "Midea Portasplit"
	// A Munich subscriber (small radius covers the Munich store).
	fs.subs = []domain.Subscription{confirmedSub("s1", "muc@x.de", "80331", 25, nil)}

	mailer := notify.NewFakeMailer()
	p := New(Config{
		Sources: []source.Source{source.NewMockWithTransition(true)}, // toggle ON
		Store:   fs,
		Mailer:  mailer,
	})

	sent, err := p.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sent == 0 {
		t.Fatal("expected at least one availability email from the Munich transition")
	}
	if len(fs.inserted) == 0 {
		t.Fatal("no snapshots written")
	}
	// Munich subscriber should have received a mail mentioning München.
	found := false
	for _, m := range mailer.Messages() {
		if m.To == "muc@x.de" {
			found = true
		}
	}
	if !found {
		t.Fatal("Munich subscriber did not get an email")
	}
}

func TestPollOnce_NoTransitionNoEmail(t *testing.T) {
	fs := newFakeStore()
	// Munich was ALREADY in stock -> no transition.
	fs.prev[key("mock", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", source.MockProductMideaID)] =
		domain.AvailabilitySnapshot{InStock: true}
	fs.subs = []domain.Subscription{confirmedSub("s1", "muc@x.de", "80331", 25, nil)}

	mailer := notify.NewFakeMailer()
	p := New(Config{Sources: []source.Source{source.NewMockWithTransition(true)}, Store: fs, Mailer: mailer})
	_, err := p.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// No prior snapshot for the other in-stock listings either -> first-seen is
	// not a transition. So no Munich mail.
	for _, m := range mailer.Messages() {
		if m.To == "muc@x.de" {
			// Munich could still get mail from online listings (everyone). Ensure
			// none reference Munich store transition. Since online listings had no
			// prev snapshot, they are first-seen, not transitions -> no mail.
			t.Fatalf("unexpected email to Munich subscriber: %q", m.Subject)
		}
	}
}

func TestNotify_RadiusMiss(t *testing.T) {
	fs := newFakeStore()
	fs.stores["st1"] = domain.Store{ID: "st1", Name: "OBI Berlin", City: "Berlin", Lat: 52.53, Lon: 13.38}
	fs.products["p1"] = "Prod"
	// Subscriber in Munich, tiny radius -> Berlin store is a miss.
	fs.subs = []domain.Subscription{confirmedSub("s1", "muc@x.de", "80331", 10, nil)}

	mailer := notify.NewFakeMailer()
	p := New(Config{Store: fs, Mailer: mailer})

	tr := []transition{{listing: domain.Listing{SourceKey: "mock", StoreID: sptr("st1"), ProductID: "p1", InStock: true, Currency: "EUR"}}}
	sent, err := p.notify(context.Background(), tr)
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 mails (radius miss), got %d", sent)
	}
}

func TestNotify_OnlineEveryone(t *testing.T) {
	fs := newFakeStore()
	fs.products["p1"] = "Prod"
	fs.subs = []domain.Subscription{
		confirmedSub("s1", "a@x.de", "10115", 10, nil),
		confirmedSub("s2", "b@x.de", "80331", 10, nil),
	}
	mailer := notify.NewFakeMailer()
	p := New(Config{Store: fs, Mailer: mailer})

	tr := []transition{{listing: domain.Listing{SourceKey: "amazon", StoreID: nil, ProductID: "p1", InStock: true, Currency: "EUR"}}}
	sent, err := p.notify(context.Background(), tr)
	if err != nil {
		t.Fatal(err)
	}
	if sent != 2 {
		t.Fatalf("online listing should notify everyone (2), got %d", sent)
	}
}

func TestNotify_ProductFilter(t *testing.T) {
	fs := newFakeStore()
	fs.products["p1"] = "Prod1"
	p2 := "p2"
	fs.subs = []domain.Subscription{confirmedSub("s1", "a@x.de", "10115", 50, &p2)} // only wants p2
	mailer := notify.NewFakeMailer()
	p := New(Config{Store: fs, Mailer: mailer})

	tr := []transition{{listing: domain.Listing{SourceKey: "amazon", ProductID: "p1", InStock: true, Currency: "EUR"}}}
	sent, _ := p.notify(context.Background(), tr)
	if sent != 0 {
		t.Fatalf("product filter should exclude (0), got %d", sent)
	}
}

func TestNotify_Dedupe(t *testing.T) {
	fs := newFakeStore()
	fs.products["p1"] = "Prod"
	fs.subs = []domain.Subscription{confirmedSub("s1", "a@x.de", "10115", 10, nil)}
	mailer := notify.NewFakeMailer()
	p := New(Config{Store: fs, Mailer: mailer})

	tr := []transition{{listing: domain.Listing{SourceKey: "amazon", ProductID: "p1", InStock: true, Currency: "EUR"}}}
	if sent, _ := p.notify(context.Background(), tr); sent != 1 {
		t.Fatalf("first notify sent %d, want 1", sent)
	}
	// Same transition again same day -> deduped.
	if sent, _ := p.notify(context.Background(), tr); sent != 0 {
		t.Fatalf("dedupe failed: second notify sent %d, want 0", sent)
	}
}

func sptr(s string) *string { return &s }
