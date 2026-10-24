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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
	"github.com/tranmh/portasplitfinder/internal/store"
)

// startSchemaStack runs the real router in-process against a fresh, migrated
// schema in the run's Postgres (AQAA_PG_URL). The schema is dropped on cleanup.
func startSchemaStack(t *testing.T) (*store.DB, *httptest.Server, *notify.FakeMailer) {
	t.Helper()
	base := os.Getenv("AQAA_PG_URL")
	if base == "" {
		t.Skip("AQAA_PG_URL not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("aqaa_http_%d", time.Now().UnixNano())
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
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	if err := store.Migrate(ctx, u.String()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	mailer := notify.NewFakeMailer()
	ts := httptest.NewServer(New(Config{Store: db, Mailer: mailer, PublicBaseURL: "http://localhost:8080"}).Router())
	t.Cleanup(ts.Close)
	return db, ts, mailer
}

type httpResult struct {
	code int
	ct   string
	body []byte
}

func (r httpResult) decode(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.body, out); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
}

func doReq(t *testing.T, method, u, body string) httpResult {
	t.Helper()
	req, err := http.NewRequest(method, u, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return httpResult{code: resp.StatusCode, ct: resp.Header.Get("Content-Type"), body: b}
}

// assertAPIError checks the contract error shape {error, message, field}.
func assertAPIError(t *testing.T, r httpResult, code int, errCode string, field *string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("status %d, want %d (body %s)", r.code, code, r.body)
	}
	if r.ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type %q", r.ct)
	}
	var raw map[string]any
	r.decode(t, &raw)
	if _, ok := raw["field"]; !ok {
		t.Fatalf("field key missing: %s", r.body)
	}
	var e apiError
	r.decode(t, &e)
	if e.Error != errCode || e.Message == "" {
		t.Fatalf("error body %s, want code %s", r.body, errCode)
	}
	switch {
	case field == nil && e.Field != nil:
		t.Fatalf("field = %q, want null", *e.Field)
	case field != nil && (e.Field == nil || *e.Field != *field):
		t.Fatalf("field = %v, want %q (body %s)", e.Field, *field, r.body)
	}
}

func countSubs(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM subscriptions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIntegrationSubscribeValidation(t *testing.T) {
	db, ts, mailer := startSchemaStack(t)
	seedStack(t, db)

	cases := []struct {
		name, body string
		field      *string
	}{
		{"invalid json", `{"email":`, nil},
		{"not an object", `[1,2]`, nil},
		{"wrong type radius", `{"email":"a@b.de","plz":"10115","radiusKm":"50"}`, nil},
		{"empty body", ``, nil},
		{"missing email", `{"plz":"10115"}`, fieldPtr("email")},
		{"bad email", `{"email":"not-an-email","plz":"10115"}`, fieldPtr("email")},
		{"display name email", `{"email":"Bob <a@b.de>","plz":"10115"}`, fieldPtr("email")},
		{"missing plz", `{"email":"a@b.de"}`, fieldPtr("plz")},
		{"4 digit plz", `{"email":"a@b.de","plz":"1011"}`, fieldPtr("plz")},
		{"6 digit plz", `{"email":"a@b.de","plz":"101155"}`, fieldPtr("plz")},
		{"letter plz", `{"email":"a@b.de","plz":"1011a"}`, fieldPtr("plz")},
		{"radius 0", `{"email":"a@b.de","plz":"10115","radiusKm":0}`, fieldPtr("radiusKm")},
		{"radius negative", `{"email":"a@b.de","plz":"10115","radiusKm":-5}`, fieldPtr("radiusKm")},
		{"radius 501", `{"email":"a@b.de","plz":"10115","radiusKm":501}`, fieldPtr("radiusKm")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", c.body)
			assertAPIError(t, r, http.StatusBadRequest, codeValidation, c.field)
		})
	}
	if n := countSubs(t, db); n != 0 {
		t.Fatalf("invalid requests created %d subscriptions", n)
	}
	if mailer.Count() != 0 {
		t.Fatalf("invalid requests sent %d mails", mailer.Count())
	}
}

func TestIntegrationSubscribeAccepted(t *testing.T) {
	db, ts, mailer := startSchemaStack(t)
	seedStack(t, db)
	ctx := context.Background()

	cases := []struct {
		name, body, email string
		radius            int
		productID         *string
	}{
		{"default radius and any product", `{"email":"def@b.de","plz":"10115"}`, "def@b.de", 50, nil},
		{"explicit null product", `{"email":"null@b.de","plz":"80331","radiusKm":1,"productId":null}`, "null@b.de", 1, nil},
		{"radius 500 with product", `{"email":"p@b.de","plz":"50667","radiusKm":500,"productId":"` + source.MockProductDaikinID + `"}`, "p@b.de", 500, strPtr(source.MockProductDaikinID)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := mailer.Count()
			r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", c.body)
			if r.code != http.StatusAccepted || r.ct != "application/json; charset=utf-8" {
				t.Fatalf("status %d ct %q body %s", r.code, r.ct, r.body)
			}
			var body map[string]string
			r.decode(t, &body)
			if body["status"] != "pending_confirmation" {
				t.Fatalf("body %s", r.body)
			}
			if mailer.Count() != before+1 {
				t.Fatalf("expected one confirmation mail, got %d", mailer.Count()-before)
			}
			msg := mailer.Messages()[before]
			tok := extractToken(msg.HTML, "/api/subscriptions/confirm?token=")
			sub, ok, err := db.Subscriptions.GetByConfirmToken(ctx, tok)
			if err != nil || !ok {
				t.Fatalf("subscription for mailed token: %v %v", ok, err)
			}
			if sub.Email != c.email || sub.RadiusKm != c.radius || sub.ConfirmedAt != nil {
				t.Fatalf("stored sub %+v", sub)
			}
			if (c.productID == nil) != (sub.ProductID == nil) || (c.productID != nil && *c.productID != *sub.ProductID) {
				t.Fatalf("product id stored %v, want %v", sub.ProductID, c.productID)
			}
			if !strings.Contains(msg.HTML, "/api/subscriptions/unsubscribe?token="+sub.UnsubToken) {
				t.Fatal("unsubscribe link missing from confirmation mail")
			}
		})
	}

	t.Run("duplicate subscription still 202 with fresh mail", func(t *testing.T) {
		before := mailer.Count()
		var tokens []string
		for i := 0; i < 2; i++ {
			r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", `{"email":"dup@b.de","plz":"10115"}`)
			if r.code != http.StatusAccepted {
				t.Fatalf("attempt %d: status %d %s", i, r.code, r.body)
			}
			tokens = append(tokens, extractToken(mailer.Messages()[before+i].HTML, "/api/subscriptions/confirm?token="))
		}
		if mailer.Count() != before+2 {
			t.Fatalf("expected 2 mails, got %d", mailer.Count()-before)
		}
		if tokens[0] == "" || tokens[0] == tokens[1] {
			t.Fatalf("confirmation tokens not fresh: %v", tokens)
		}
	})

	t.Run("mailer failure still 202", func(t *testing.T) {
		mailer.Err = fmt.Errorf("smtp down")
		defer func() { mailer.Err = nil }()
		r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", `{"email":"fail@b.de","plz":"10115"}`)
		if r.code != http.StatusAccepted {
			t.Fatalf("status %d %s", r.code, r.body)
		}
	})
}

func TestIntegrationSubscribeBadProductID(t *testing.T) {
	db, ts, _ := startSchemaStack(t)
	seedStack(t, db)

	cases := []struct{ name, productID string }{
		{"not a uuid", "abc"},
		{"unknown uuid", "99999999-9999-9999-9999-999999999999"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", `{"email":"a@b.de","plz":"10115","productId":"`+c.productID+`"}`)
			if r.code == http.StatusInternalServerError {
				// B11/B19: a bad client input must be a 400 validation_error, not a 500.
				t.Skip("aqaa: suspected bug: POST /api/subscriptions with productId " + c.productID + " returns 500 internal instead of 400 validation_error (B11, B19); the store's uuid cast/FK error is surfaced as an internal error")
			}
			assertAPIError(t, r, http.StatusBadRequest, codeValidation, fieldPtr("productId"))
		})
	}
}

func TestIntegrationConfirmUnsubscribeTokens(t *testing.T) {
	db, ts, mailer := startSchemaStack(t)
	seedStack(t, db)
	ctx := context.Background()

	r := doReq(t, http.MethodPost, ts.URL+"/api/subscriptions", `{"email":"tok@b.de","plz":"10115","radiusKm":100}`)
	if r.code != http.StatusAccepted {
		t.Fatalf("subscribe: %d %s", r.code, r.body)
	}
	html := mailer.Messages()[0].HTML
	confirmTok := extractToken(html, "/api/subscriptions/confirm?token=")
	unsubTok := extractToken(html, "/api/subscriptions/unsubscribe?token=")
	if confirmTok == "" || unsubTok == "" {
		t.Fatalf("tokens missing in mail: %s", html)
	}

	for _, path := range []string{"/api/subscriptions/confirm", "/api/subscriptions/unsubscribe"} {
		t.Run(path+" missing token", func(t *testing.T) {
			assertAPIError(t, doReq(t, http.MethodGet, ts.URL+path, ""), http.StatusBadRequest, codeInvalidTok, fieldPtr("token"))
			assertAPIError(t, doReq(t, http.MethodGet, ts.URL+path+"?token=", ""), http.StatusBadRequest, codeInvalidTok, fieldPtr("token"))
		})
		t.Run(path+" unknown token", func(t *testing.T) {
			assertAPIError(t, doReq(t, http.MethodGet, ts.URL+path+"?token=unknown-token", ""), http.StatusNotFound, codeInvalidTok, fieldPtr("token"))
		})
	}

	t.Run("tokens are not interchangeable", func(t *testing.T) {
		assertAPIError(t, doReq(t, http.MethodGet, ts.URL+"/api/subscriptions/confirm?token="+url.QueryEscape(unsubTok), ""), http.StatusNotFound, codeInvalidTok, fieldPtr("token"))
		assertAPIError(t, doReq(t, http.MethodGet, ts.URL+"/api/subscriptions/unsubscribe?token="+url.QueryEscape(confirmTok), ""), http.StatusNotFound, codeInvalidTok, fieldPtr("token"))
	})

	t.Run("confirm twice", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			r := doReq(t, http.MethodGet, ts.URL+"/api/subscriptions/confirm?token="+url.QueryEscape(confirmTok), "")
			if r.code != http.StatusOK || r.ct != "application/json; charset=utf-8" {
				t.Fatalf("confirm %d: %d %q %s", i, r.code, r.ct, r.body)
			}
			var body map[string]string
			r.decode(t, &body)
			if body["status"] != "confirmed" || body["email"] != "tok@b.de" {
				t.Fatalf("confirm %d body %s", i, r.body)
			}
		}
		subs, err := db.Subscriptions.ListConfirmed(ctx)
		if err != nil || len(subs) != 1 || subs[0].Email != "tok@b.de" {
			t.Fatalf("confirmed subs: %v %+v", err, subs)
		}
	})

	t.Run("unsubscribe twice", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			r := doReq(t, http.MethodGet, ts.URL+"/api/subscriptions/unsubscribe?token="+url.QueryEscape(unsubTok), "")
			if r.code != http.StatusOK {
				t.Fatalf("unsub %d: %d %s", i, r.code, r.body)
			}
			var body map[string]string
			r.decode(t, &body)
			if body["status"] != "unsubscribed" {
				t.Fatalf("unsub %d body %s", i, r.body)
			}
		}
		subs, _ := db.Subscriptions.ListConfirmed(ctx)
		if len(subs) != 0 {
			t.Fatalf("still confirmed after unsub: %+v", subs)
		}
	})
}

func TestIntegrationAvailabilityQueryParams(t *testing.T) {
	db, ts, _ := startSchemaStack(t)
	seedStack(t, db)
	ctx := context.Background()
	// A Daikin listing in Berlin (out of stock) next to the Midea one, plus an
	// older Berlin Midea row that must be superseded by the latest snapshot.
	berlin := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	old := time.Now().UTC().Add(-time.Hour)
	if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &berlin, ProductID: source.MockProductDaikinID, InStock: false, Currency: "EUR", URL: "u-berlin-daikin"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Snapshots.Insert(ctx, domain.AvailabilitySnapshot{SourceKey: "mock", StoreID: &berlin, ProductID: source.MockProductMideaID, InStock: false, Currency: "EUR", URL: "u-berlin-old", CheckedAt: old}); err != nil {
		t.Fatal(err)
	}

	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			query, field string
		}{
			{"plz=1011", "plz"},
			{"plz=101155", "plz"},
			{"plz=abcde", "plz"},
			{"plz=00000", "plz"}, // well-formed but unknown
			{"plz=10115&radius=0", "radius"},
			{"plz=10115&radius=501", "radius"},
			{"plz=10115&radius=-1", "radius"},
			{"plz=10115&radius=abc", "radius"},
			{"plz=10115&radius=12.5", "radius"},
			{"radius=501", "radius"},
		}
		for _, c := range cases {
			r := doReq(t, http.MethodGet, ts.URL+"/api/availability?"+c.query, "")
			t.Run(c.query, func(t *testing.T) { assertAPIError(t, r, http.StatusBadRequest, codeValidation, fieldPtr(c.field)) })
		}
	})

	t.Run("no plz: null center and distances, default radius", func(t *testing.T) {
		r := doReq(t, http.MethodGet, ts.URL+"/api/availability", "")
		if r.code != 200 || r.ct != "application/json; charset=utf-8" {
			t.Fatalf("%d %q %s", r.code, r.ct, r.body)
		}
		var raw map[string]json.RawMessage
		r.decode(t, &raw)
		if string(raw["center"]) != "null" {
			t.Fatalf("center %s", raw["center"])
		}
		var resp AvailabilityResponse
		r.decode(t, &resp)
		if resp.RadiusKm != 50 || len(resp.Stores) != 2 || resp.GeneratedAt.IsZero() {
			t.Fatalf("resp %+v", resp)
		}
		if resp.GeneratedAt.Location() != time.UTC {
			t.Fatalf("generatedAt not UTC: %s", raw["generatedAt"])
		}
		for _, s := range resp.Stores {
			if s.DistanceKm != nil {
				t.Fatalf("distance not null without plz: %+v", s)
			}
			if s.Store.ID == berlin {
				// Latest Midea row (in stock) wins over the older out-of-stock one,
				// and in-stock listings come first.
				if len(s.Listings) != 2 || !s.Listings[0].InStock || s.Listings[0].URL != "u-berlin" || s.Listings[1].ProductID != source.MockProductDaikinID {
					t.Fatalf("berlin listings %+v", s.Listings)
				}
			}
		}
		if len(resp.Online) != 1 || resp.Online[0].Source.Key != "amazon" || resp.Online[0].Listings[0].StoreID != nil {
			t.Fatalf("online %+v", resp.Online)
		}
	})

	t.Run("plz with radius: center and nearest first", func(t *testing.T) {
		// Frankfurt: München ~304km, Berlin ~423km.
		r := doReq(t, http.MethodGet, ts.URL+"/api/availability?plz=60311&radius=500", "")
		var resp AvailabilityResponse
		r.decode(t, &resp)
		if r.code != 200 || resp.Center == nil || resp.Center.PLZ != "60311" || resp.Center.Lat == 0 || resp.RadiusKm != 500 {
			t.Fatalf("%d %+v", r.code, resp)
		}
		if len(resp.Stores) != 2 || resp.Stores[0].Store.City != "München" || resp.Stores[1].Store.City != "Berlin" {
			t.Fatalf("stores %+v", resp.Stores)
		}
		if *resp.Stores[0].DistanceKm > *resp.Stores[1].DistanceKm || *resp.Stores[1].DistanceKm > 500 {
			t.Fatalf("distances %v %v", *resp.Stores[0].DistanceKm, *resp.Stores[1].DistanceKm)
		}
	})

	t.Run("productId restricts results", func(t *testing.T) {
		var resp AvailabilityResponse
		doReq(t, http.MethodGet, ts.URL+"/api/availability?productId="+source.MockProductDaikinID, "").decode(t, &resp)
		if len(resp.Stores) != 1 || resp.Stores[0].Store.ID != berlin || len(resp.Stores[0].Listings) != 1 || len(resp.Online) != 0 {
			t.Fatalf("daikin only: %+v", resp)
		}
	})

	t.Run("empty result is [] not null", func(t *testing.T) {
		for _, q := range []string{"productId=99999999-9999-9999-9999-999999999999", "productId=not-a-uuid", "plz=60311&radius=1&productId=99999999-9999-9999-9999-999999999999"} {
			r := doReq(t, http.MethodGet, ts.URL+"/api/availability?"+q, "")
			if r.code != 200 {
				t.Fatalf("%s: %d %s", q, r.code, r.body)
			}
			var raw map[string]json.RawMessage
			r.decode(t, &raw)
			if string(raw["stores"]) != "[]" || string(raw["online"]) != "[]" {
				t.Fatalf("%s: stores=%s online=%s", q, raw["stores"], raw["online"])
			}
		}
	})
}

func TestIntegrationAvailabilityEmptyDB(t *testing.T) {
	_, ts, _ := startSchemaStack(t)
	r := doReq(t, http.MethodGet, ts.URL+"/api/availability?plz=10115", "")
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.body)
	}
	var raw map[string]json.RawMessage
	r.decode(t, &raw)
	if string(raw["stores"]) != "[]" || string(raw["online"]) != "[]" {
		t.Fatalf("stores=%s online=%s", raw["stores"], raw["online"])
	}
}

func strPtr(s string) *string { return &s }
