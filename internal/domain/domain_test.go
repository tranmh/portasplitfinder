package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSourceType_Valid(t *testing.T) {
	cases := []struct {
		in   SourceType
		want bool
	}{
		{SourceOnline, true},
		{SourceStore, true},
		{"online", true},
		{"store", true},
		{"", false},
		{"Online", false},
		{"STORE", false},
		{" online", false},
		{"shop", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			if got := tc.in.Valid(); got != tc.want {
				t.Fatalf("SourceType(%q).Valid() = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	cases := map[LastStatus]string{StatusOK: "ok", StatusError: "error", StatusNever: "never"}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("status %q, want %q", got, want)
		}
	}
}

// B12: only confirmed and not unsubscribed subscriptions count as confirmed.
func TestSubscription_Confirmed(t *testing.T) {
	ts := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		confirmedAt *time.Time
		unsubbedAt  *time.Time
		want        bool
	}{
		{"pending", nil, nil, false},
		{"confirmed", &ts, nil, true},
		{"confirmed then unsubscribed", &ts, &ts, false},
		{"unsubscribed without confirm", nil, &ts, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Subscription{ConfirmedAt: tc.confirmedAt, UnsubbedAt: tc.unsubbedAt}
			if got := s.Confirmed(); got != tc.want {
				t.Fatalf("Confirmed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func marshalMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestSubscription_JSONHidesTokens(t *testing.T) {
	s := Subscription{
		ID: "id-1", Email: "a@b.de", PLZ: "73035", RadiusKm: 50,
		ConfirmToken: "secret-confirm", UnsubToken: "secret-unsub",
		CreatedAt: time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC),
	}
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "secret") {
		t.Fatalf("tokens leaked into JSON: %s", b)
	}
	m := marshalMap(t, s)
	for _, k := range []string{"id", "email", "plz", "radiusKm", "productId", "createdAt", "confirmedAt", "unsubbedAt"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %v", k, m)
		}
	}
	// B7: productId null means any product.
	if m["productId"] != nil || m["confirmedAt"] != nil || m["unsubbedAt"] != nil {
		t.Fatalf("unset pointers must encode as null: %v", m)
	}
}

// B23: null price means unknown; timestamps are RFC 3339 UTC.
func TestListing_JSONShape(t *testing.T) {
	checked := time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)
	cents := 49900
	cases := []struct {
		name      string
		listing   Listing
		wantPrice any
		wantStore any
	}{
		{"unknown price online", Listing{SourceKey: "amazon", ProductID: "p", Currency: "EUR", CheckedAt: checked}, nil, nil},
		{"priced store listing", Listing{SourceKey: "obi", ProductID: "p", StoreID: strPtr("s1"), PriceCents: &cents, Currency: "EUR", CheckedAt: checked}, float64(49900), "s1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := marshalMap(t, tc.listing)
			if m["priceCents"] != tc.wantPrice {
				t.Fatalf("priceCents = %#v, want %#v", m["priceCents"], tc.wantPrice)
			}
			if m["storeId"] != tc.wantStore {
				t.Fatalf("storeId = %#v, want %#v", m["storeId"], tc.wantStore)
			}
			if _, ok := m["quantity"]; !ok {
				t.Fatal("quantity key must be present")
			}
			if m["currency"] != "EUR" {
				t.Fatalf("currency = %v", m["currency"])
			}
			if m["checkedAt"] != "2026-11-07T09:30:00Z" {
				t.Fatalf("checkedAt = %v, want RFC 3339 UTC", m["checkedAt"])
			}
		})
	}
}

func TestSourceAndProduct_JSONKeys(t *testing.T) {
	src := marshalMap(t, Source{Key: "obi", Name: "OBI", Type: SourceStore, BaseURL: "https://obi.de", LastStatus: StatusNever})
	for _, k := range []string{"key", "name", "type", "baseUrl", "lastCheckedAt", "lastStatus", "lastError"} {
		if _, ok := src[k]; !ok {
			t.Errorf("source missing key %q", k)
		}
	}
	if src["type"] != "store" || src["lastStatus"] != "never" || src["lastCheckedAt"] != nil || src["lastError"] != nil {
		t.Fatalf("unexpected source JSON: %v", src)
	}

	prod := marshalMap(t, Product{ID: "p1", Name: "Portasplit", Model: "PS-1"})
	if _, ok := prod["imageUrl"]; !ok || prod["imageUrl"] != nil {
		t.Fatalf("imageUrl must be present and null: %v", prod)
	}

	st := marshalMap(t, Store{ID: "s1", SourceKey: "obi", Name: "OBI Kirchheim", PLZ: "73230", City: "Kirchheim", Lat: 48.6, Lon: 9.4})
	for _, k := range []string{"id", "sourceKey", "name", "street", "plz", "city", "lat", "lon"} {
		if _, ok := st[k]; !ok {
			t.Errorf("store missing key %q", k)
		}
	}
}

func strPtr(s string) *string { return &s }
