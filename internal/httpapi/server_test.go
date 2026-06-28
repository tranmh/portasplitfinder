package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/notify"
)

// fakeStore implements DataStore in-memory for handler tests.
type fakeStore struct {
	pingErr  error
	products []domain.Product
	sources  []domain.Source
	stores   []domain.Store
	snaps    []domain.AvailabilitySnapshot
	subs     map[string]domain.Subscription // by confirm token
	created  int
}

func newFakeStore() *fakeStore { return &fakeStore{subs: map[string]domain.Subscription{}} }

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }
func (f *fakeStore) ListProducts(context.Context) ([]domain.Product, error) {
	return f.products, nil
}
func (f *fakeStore) ListSources(context.Context) ([]domain.Source, error) { return f.sources, nil }
func (f *fakeStore) ListStores(context.Context) ([]domain.Store, error)   { return f.stores, nil }
func (f *fakeStore) LatestSnapshots(context.Context) ([]domain.AvailabilitySnapshot, error) {
	return f.snaps, nil
}
func (f *fakeStore) CreateSubscription(_ context.Context, s domain.Subscription) (string, error) {
	f.created++
	s.ID = "sub-1"
	f.subs[s.ConfirmToken] = s
	return s.ID, nil
}
func (f *fakeStore) ConfirmSubscription(_ context.Context, token string) (string, bool, error) {
	s, ok := f.subs[token]
	if !ok {
		return "", false, nil
	}
	return s.Email, true, nil
}
func (f *fakeStore) UnsubSubscription(_ context.Context, token string) (bool, error) {
	for _, s := range f.subs {
		if s.UnsubToken == token {
			return true, nil
		}
	}
	return false, nil
}

func newTestServer(fs *fakeStore) (*Server, *notify.FakeMailer) {
	m := notify.NewFakeMailer()
	return New(Config{Store: fs, Mailer: m, PublicBaseURL: "http://localhost:8080"}), m
}

func TestHealth_OK(t *testing.T) {
	s, _ := newTestServer(newFakeStore())
	rr := do(s, http.MethodGet, "/api/health", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestHealth_Degraded(t *testing.T) {
	fs := newFakeStore()
	fs.pingErr = errors.New("down")
	s, _ := newTestServer(fs)
	rr := do(s, http.MethodGet, "/api/health", nil)
	if rr.Code != 503 {
		t.Fatalf("status %d, want 503", rr.Code)
	}
}

func TestSubscribe_Validation(t *testing.T) {
	s, _ := newTestServer(newFakeStore())
	cases := []string{
		`{"email":"bad","plz":"10115"}`,
		`{"email":"a@b.de","plz":"123"}`,
		`{"email":"a@b.de","plz":"10115","radiusKm":0}`,
		`{"email":"a@b.de","plz":"10115","radiusKm":999}`,
	}
	for _, body := range cases {
		rr := do(s, http.MethodPost, "/api/subscriptions", []byte(body))
		if rr.Code != 400 {
			t.Errorf("body %s: status %d, want 400", body, rr.Code)
		}
		var e apiError
		_ = json.Unmarshal(rr.Body.Bytes(), &e)
		if e.Error != codeValidation {
			t.Errorf("body %s: error code %q", body, e.Error)
		}
	}
}

func TestSubscribe_Confirm_Unsub_Flow(t *testing.T) {
	fs := newFakeStore()
	s, mailer := newTestServer(fs)

	rr := do(s, http.MethodPost, "/api/subscriptions", []byte(`{"email":"a@b.de","plz":"10115","radiusKm":50}`))
	if rr.Code != 202 {
		t.Fatalf("subscribe status %d, want 202", rr.Code)
	}
	if fs.created != 1 {
		t.Fatalf("created %d subs", fs.created)
	}
	if mailer.Count() != 1 {
		t.Fatalf("expected 1 confirm mail, got %d", mailer.Count())
	}

	// Grab the created token.
	var token, unsub string
	for ct, sub := range fs.subs {
		token = ct
		unsub = sub.UnsubToken
	}

	rr = do(s, http.MethodGet, "/api/subscriptions/confirm?token="+token, nil)
	if rr.Code != 200 {
		t.Fatalf("confirm status %d", rr.Code)
	}
	var cresp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &cresp)
	if cresp["status"] != "confirmed" || cresp["email"] != "a@b.de" {
		t.Fatalf("confirm body %v", cresp)
	}

	rr = do(s, http.MethodGet, "/api/subscriptions/confirm?token=nope", nil)
	if rr.Code != 404 {
		t.Fatalf("bad token confirm status %d, want 404", rr.Code)
	}

	rr = do(s, http.MethodGet, "/api/subscriptions/unsubscribe?token="+unsub, nil)
	if rr.Code != 200 {
		t.Fatalf("unsub status %d", rr.Code)
	}
}

func TestAvailability_BadPLZ(t *testing.T) {
	s, _ := newTestServer(newFakeStore())
	rr := do(s, http.MethodGet, "/api/availability?plz=abc", nil)
	if rr.Code != 400 {
		t.Fatalf("status %d, want 400", rr.Code)
	}
}

func TestAvailability_BadRadius(t *testing.T) {
	s, _ := newTestServer(newFakeStore())
	rr := do(s, http.MethodGet, "/api/availability?plz=10115&radius=9999", nil)
	if rr.Code != 400 {
		t.Fatalf("status %d, want 400", rr.Code)
	}
}

func TestAvailability_OK(t *testing.T) {
	fs := newFakeStore()
	fs.stores = []domain.Store{{ID: "s1", Name: "OBI Berlin", PLZ: "10115", City: "Berlin", Lat: 52.53, Lon: 13.38}}
	fs.sources = []domain.Source{{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline}}
	fs.snaps = []domain.AvailabilitySnapshot{
		{SourceKey: "mock", StoreID: sp("s1"), ProductID: "p1", InStock: true, Currency: "EUR"},
		{SourceKey: "amazon", ProductID: "p1", InStock: true, Currency: "EUR"},
	}
	s, _ := newTestServer(fs)
	rr := do(s, http.MethodGet, "/api/availability?plz=10115&radius=50", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp AvailabilityResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Center == nil || resp.Center.PLZ != "10115" {
		t.Errorf("center wrong: %+v", resp.Center)
	}
	if len(resp.Stores) != 1 || len(resp.Online) != 1 {
		t.Errorf("stores=%d online=%d", len(resp.Stores), len(resp.Online))
	}
}

func do(s *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, r)
	return rr
}
