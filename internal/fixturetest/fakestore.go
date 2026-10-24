package fixturetest

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// FakeStore is an in-memory implementation of httpapi.DataStore and
// jobs.Store. Set Err to make every method fail.
type FakeStore struct {
	mu            sync.Mutex
	Err           error
	PingErr       error
	Products      []domain.Product
	Sources       map[string]domain.Source
	Stores        map[string]domain.Store
	Snapshots     []domain.AvailabilitySnapshot
	Subs          map[string]domain.Subscription
	Notifications map[string]bool // subscriptionID + "|" + dedupeKey
	Now           func() time.Time
	nextID        int
}

// NewFakeStore returns an empty store whose clock is fixed at Epoch.
func NewFakeStore() *FakeStore {
	return &FakeStore{Sources: map[string]domain.Source{}, Stores: map[string]domain.Store{},
		Subs: map[string]domain.Subscription{}, Notifications: map[string]bool{}, Now: NewClock().Now}
}

// NewSeededFakeStore mirrors testdata/seed.sql.
func NewSeededFakeStore() *FakeStore {
	f := NewFakeStore()
	f.Products = []domain.Product{Product(), Product(func(p *domain.Product) {
		p.ID, p.Name, p.Model = OtherProductID, "Midea PortaSplit XL", "PortaSplit 14000"
	})}
	f.Sources[SourceKeyStore] = Source()
	f.Sources[SourceKeyWeb] = Source(func(s *domain.Source) {
		s.Key, s.Name, s.Type, s.BaseURL = SourceKeyWeb, "Mock Online", domain.SourceOnline, "https://online.example"
	})
	f.Stores[StoreID] = Store()
	f.Snapshots = []domain.AvailabilitySnapshot{
		Snapshot(func(s *domain.AvailabilitySnapshot) { s.InStock, s.Quantity, s.CheckedAt = false, Ptr(0), Epoch.Add(-27 * time.Hour) }),
		Snapshot(),
		Snapshot(func(s *domain.AvailabilitySnapshot) {
			s.SourceKey, s.StoreID, s.Quantity, s.PriceCents, s.URL = SourceKeyWeb, nil, nil, Ptr(61900), "https://online.example/p"
		}),
	}
	sub := Subscription(Confirmed)
	f.Subs[sub.ID] = sub
	return f
}

func (f *FakeStore) id() string {
	f.nextID++
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", f.nextID)
}

func (f *FakeStore) Ping(context.Context) error {
	if f.PingErr != nil {
		return f.PingErr
	}
	return f.Err
}

func (f *FakeStore) ListProducts(context.Context) ([]domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Product(nil), f.Products...), f.Err
}

func (f *FakeStore) ListSources(context.Context) ([]domain.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Source{}
	for _, s := range f.Sources {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, f.Err
}

func (f *FakeStore) ListStores(context.Context) ([]domain.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Store{}
	for _, s := range f.Stores {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, f.Err
}

func snapKey(s domain.AvailabilitySnapshot) string {
	st := ""
	if s.StoreID != nil {
		st = *s.StoreID
	}
	return s.SourceKey + "|" + st + "|" + s.ProductID
}

// LatestSnapshots returns the newest snapshot per (source, store, product).
func (f *FakeStore) LatestSnapshots(context.Context) ([]domain.AvailabilitySnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	latest := map[string]domain.AvailabilitySnapshot{}
	for _, s := range f.Snapshots {
		if cur, ok := latest[snapKey(s)]; !ok || s.CheckedAt.After(cur.CheckedAt) {
			latest[snapKey(s)] = s
		}
	}
	out := []domain.AvailabilitySnapshot{}
	for _, s := range latest {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return snapKey(out[i]) < snapKey(out[j]) })
	return out, f.Err
}

func (f *FakeStore) CreateSubscription(_ context.Context, s domain.Subscription) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return "", f.Err
	}
	s.ID, s.CreatedAt = f.id(), f.Now()
	f.Subs[s.ID] = s
	return s.ID, nil
}

// ConfirmSubscription is idempotent: confirming twice keeps the first time.
func (f *FakeStore) ConfirmSubscription(_ context.Context, token string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, s := range f.Subs {
		if s.ConfirmToken == token && token != "" {
			if s.ConfirmedAt == nil {
				s.ConfirmedAt = Ptr(f.Now())
				f.Subs[id] = s
			}
			return s.Email, true, f.Err
		}
	}
	return "", false, f.Err
}

func (f *FakeStore) UnsubSubscription(_ context.Context, token string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, s := range f.Subs {
		if s.UnsubToken == token && token != "" {
			if s.UnsubbedAt == nil {
				s.UnsubbedAt = Ptr(f.Now())
				f.Subs[id] = s
			}
			return true, f.Err
		}
	}
	return false, f.Err
}

func (f *FakeStore) UpsertSource(_ context.Context, s domain.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Sources[s.Key]; !ok {
		f.Sources[s.Key] = s
	}
	return f.Err
}

// UpsertStore keys stores by (source, name), like the UNIQUE constraint.
func (f *FakeStore) UpsertStore(_ context.Context, s domain.Store) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, cur := range f.Stores {
		if cur.SourceKey == s.SourceKey && cur.Name == s.Name {
			s.ID = id
			f.Stores[id] = s
			return id, f.Err
		}
	}
	if s.ID == "" {
		s.ID = f.id()
	}
	f.Stores[s.ID] = s
	return s.ID, f.Err
}

func (f *FakeStore) InsertSnapshot(_ context.Context, s domain.AvailabilitySnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	if s.ID == "" {
		s.ID = f.id()
	}
	f.Snapshots = append(f.Snapshots, s)
	return nil
}

func (f *FakeStore) LatestForKey(ctx context.Context, sourceKey, storeID, productID string) (domain.AvailabilitySnapshot, bool, error) {
	all, err := f.LatestSnapshots(ctx)
	for _, s := range all {
		if snapKey(s) == sourceKey+"|"+storeID+"|"+productID {
			return s, true, err
		}
	}
	return domain.AvailabilitySnapshot{}, false, err
}

func (f *FakeStore) UpdateSourceStatus(_ context.Context, key string, checkedAt time.Time, status domain.LastStatus, lastErr *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.Sources[key]
	s.Key, s.LastCheckedAt, s.LastStatus, s.LastError = key, Ptr(checkedAt), status, lastErr
	f.Sources[key] = s
	return f.Err
}

func (f *FakeStore) ListConfirmedSubscriptions(context.Context) ([]domain.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Subscription{}
	for _, s := range f.Subs {
		if s.Confirmed() {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, f.Err
}

func (f *FakeStore) GetStore(_ context.Context, id string) (domain.Store, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.Stores[id]
	return s, ok, f.Err
}

func (f *FakeStore) GetProductName(_ context.Context, id string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.Products {
		if p.ID == id {
			return p.Name, true, f.Err
		}
	}
	return "", false, f.Err
}

// RecordNotification claims (sub, dedupeKey) once; later calls return false.
func (f *FakeStore) RecordNotification(_ context.Context, subscriptionID, _, _, _, dedupeKey string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return false, f.Err
	}
	k := subscriptionID + "|" + dedupeKey
	if f.Notifications[k] {
		return false, nil
	}
	f.Notifications[k] = true
	return true, nil
}
