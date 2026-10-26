package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
func (f *fakeStore) SearchStores(_ context.Context, q string) ([]domain.Store, error) {
	var out []domain.Store
	for _, st := range f.stores {
		if q == "" || strings.Contains(strings.ToLower(st.Name+" "+st.City), strings.ToLower(q)) {
			out = append(out, st)
		}
	}
	return out, nil
}
func (f *fakeStore) LatestSnapshotsForStore(_ context.Context, storeID string) ([]domain.AvailabilitySnapshot, error) {
	var out []domain.AvailabilitySnapshot
	for _, sn := range f.snaps {
		if sn.StoreID != nil && *sn.StoreID == storeID {
			out = append(out, sn)
		}
	}
	return out, nil
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

// fixtureTime is a fixed clock for fixtures; tests must not use time.Now.
var fixtureTime = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

// storeFixture builds a store; mutate the result for variations.
func storeFixture(id, name, city, plz string, lat, lon float64) domain.Store {
	return domain.Store{ID: id, SourceKey: "obi", Name: name, PLZ: plz, City: city, Lat: lat, Lon: lon}
}

// snapFixture builds a snapshot. storeID "" means an online listing.
func snapFixture(id, productID, storeID string, inStock bool) domain.AvailabilitySnapshot {
	sn := domain.AvailabilitySnapshot{
		ID: id, SourceKey: "obi", ProductID: productID, InStock: inStock,
		Currency: "EUR", URL: "https://example.test/p/" + productID, CheckedAt: fixtureTime,
	}
	if storeID != "" {
		sn.StoreID = ptr(storeID)
	}
	return sn
}

// seededFakeStore returns a fakeStore with a small, deterministic catalogue
// around Esslingen (73035): three stores and listings for two products.
func seededFakeStore() *fakeStore {
	fs := newFakeStore()
	fs.products = []domain.Product{{ID: "p1", Name: "Porta Split 1", Model: "PS1"}, {ID: "p2", Name: "Porta Split 2", Model: "PS2"}}
	fs.stores = []domain.Store{
		storeFixture("s1", "OBI Esslingen", "Esslingen", "73730", 48.74, 9.31),
		storeFixture("s2", "OBI Stuttgart", "Stuttgart", "70173", 48.78, 9.18),
		storeFixture("s3", "OBI Berlin", "Berlin", "10115", 52.53, 13.38),
	}
	fs.snaps = []domain.AvailabilitySnapshot{
		snapFixture("a1", "p1", "s1", true),
		snapFixture("a2", "p2", "s1", false),
		snapFixture("a3", "p1", "s2", false),
		snapFixture("a4", "p1", "s3", true),
		snapFixture("a5", "p1", "", true),
	}
	return fs
}

func TestFixtures_SeededFakeStore(t *testing.T) {
	fs := seededFakeStore()
	ctx := context.Background()
	got, err := fs.SearchStores(ctx, "esslingen")
	if err != nil || len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("SearchStores = %v, %v", got, err)
	}
	snaps, err := fs.LatestSnapshotsForStore(ctx, "s1")
	if err != nil || len(snaps) != 2 {
		t.Fatalf("LatestSnapshotsForStore = %v, %v", snaps, err)
	}
	var _ DataStore = fs
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

func TestStoreSearch_ByCity(t *testing.T) {
	fs := newFakeStore()
	fs.stores = []domain.Store{
		{ID: "s1", Name: "OBI Berlin-Spandau", PLZ: "13597", City: "Berlin", Lat: 52.53, Lon: 13.20},
		{ID: "s2", Name: "OBI München", PLZ: "80331", City: "München", Lat: 48.14, Lon: 11.58},
	}
	fs.snaps = []domain.AvailabilitySnapshot{
		{SourceKey: "mock", StoreID: sp("s1"), ProductID: "p1", InStock: true, Currency: "EUR"},
	}
	s, _ := newTestServer(fs)
	rr := do(s, http.MethodGet, "/api/stores?q=berlin", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp StoreSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Stores) != 1 || resp.Stores[0].Store.ID != "s1" {
		t.Fatalf("unexpected result: %+v", resp)
	}
	if len(resp.Stores[0].Listings) != 1 {
		t.Errorf("listings = %d, want 1", len(resp.Stores[0].Listings))
	}
}

func TestStoreSearch_BadPageSize(t *testing.T) {
	s, _ := newTestServer(newFakeStore())
	rr := do(s, http.MethodGet, "/api/stores?pageSize=0", nil)
	if rr.Code != 400 {
		t.Fatalf("status %d, want 400", rr.Code)
	}
}

func TestIntParam(t *testing.T) {
	tests := []struct {
		name   string
		v      string
		def    int
		lo, hi int
		want   int
		wantOK bool
	}{
		{"empty yields default", "", 20, 1, 100, 20, true},
		{"default not range checked", "", 0, 1, 100, 0, true},
		{"lower bound inclusive", "1", 20, 1, 100, 1, true},
		{"upper bound inclusive", "100", 20, 1, 100, 100, true},
		{"inside range", "42", 20, 1, 100, 42, true},
		{"leading zeros", "007", 20, 1, 100, 7, true},
		{"explicit plus sign", "+5", 20, 1, 100, 5, true},
		{"below lower bound", "0", 20, 1, 100, 0, false},
		{"above upper bound", "101", 20, 1, 100, 0, false},
		{"negative", "-1", 20, 1, 100, 0, false},
		{"non numeric", "abc", 20, 1, 100, 0, false},
		{"decimal", "1.5", 20, 1, 100, 0, false},
		{"trailing garbage", "10x", 20, 1, 100, 0, false},
		{"surrounding spaces", " 10 ", 20, 1, 100, 0, false},
		{"whitespace only", " ", 20, 1, 100, 0, false},
		{"overflow", "99999999999999999999999", 1, 1, 1 << 20, 0, false},
		{"page upper bound", "1048576", 1, 1, 1 << 20, 1 << 20, true},
		{"page above upper bound", "1048577", 1, 1, 1 << 20, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := intParam(tt.v, tt.def, tt.lo, tt.hi)
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Fatalf("intParam(%q, %d, %d, %d) = (%d, %v), want (%d, %v)",
					tt.v, tt.def, tt.lo, tt.hi, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestStoreSearch_PageParamValidation(t *testing.T) {
	s, _ := newTestServer(seededFakeStore())
	cases := []struct {
		query string
		code  int
		field string
	}{
		{"page=0", 400, "page"},
		{"page=-1", 400, "page"},
		{"page=x", 400, "page"},
		{"pageSize=101", 400, "pageSize"},
		{"pageSize=1.5", 400, "pageSize"},
		{"page=1&pageSize=1", 200, ""},
		{"pageSize=100", 200, ""},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			rr := do(s, http.MethodGet, "/api/stores?"+c.query, nil)
			if rr.Code != c.code {
				t.Fatalf("status %d, want %d", rr.Code, c.code)
			}
			if c.code != 400 {
				return
			}
			var e apiError
			if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if e.Error != codeValidation || e.Field == nil || *e.Field != c.field {
				t.Fatalf("error = %+v, want validation_error on %q", e, c.field)
			}
		})
	}
}

func TestStoreSearch_Defaults(t *testing.T) {
	s, _ := newTestServer(seededFakeStore())
	rr := do(s, http.MethodGet, "/api/stores", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var resp StoreSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Page != 1 || resp.PageSize != defaultPageSize || resp.Total != 3 {
		t.Fatalf("page=%d pageSize=%d total=%d, want 1/%d/3", resp.Page, resp.PageSize, resp.Total, defaultPageSize)
	}
}

// TestStoreSearch_PageContainsPageSizeStores checks B8: page p holds
// results[(p-1)*pageSize .. p*pageSize), with no store skipped between pages.
func TestStoreSearch_PageContainsPageSizeStores(t *testing.T) {
	s, _ := newTestServer(seededFakeStore())
	var seen []string
	for _, page := range []string{"1", "2"} {
		rr := do(s, http.MethodGet, "/api/stores?pageSize=2&page="+page, nil)
		if rr.Code != 200 {
			t.Fatalf("page %s: status %d", page, rr.Code)
		}
		var resp StoreSearchResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Total != 3 {
			t.Fatalf("page %s: total %d, want 3", page, resp.Total)
		}
		want := 2
		if page == "2" {
			want = 1
		}
		if len(resp.Stores) != want {
			t.Fatalf("page %s: %d stores, want %d", page, len(resp.Stores), want)
		}
		for _, st := range resp.Stores {
			seen = append(seen, st.Store.ID)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("stores across pages = %v, want all 3", seen)
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
