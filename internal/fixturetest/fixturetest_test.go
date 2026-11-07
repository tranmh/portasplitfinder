package fixturetest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/fixturetest"
	"github.com/tranmh/portasplitfinder/internal/httpapi"
	"github.com/tranmh/portasplitfinder/internal/notify"
)

var _ httpapi.DataStore = (*fixturetest.FakeStore)(nil)

func TestFakeStore_SubscriptionLifecycle(t *testing.T) {
	ctx := context.Background()
	fs := fixturetest.NewFakeStore()
	sub := fixturetest.Subscription()

	id, err := fs.CreateSubscription(ctx, sub)
	if err != nil || id != "sub-1" {
		t.Fatalf("create: id=%q err=%v", id, err)
	}
	fs.Clock.Advance(time.Minute)

	tests := []struct {
		name   string
		run    func() (bool, error)
		wantOK bool
	}{
		{"confirm unknown", func() (bool, error) { _, ok, err := fs.ConfirmSubscription(ctx, "nope"); return ok, err }, false},
		{"confirm", func() (bool, error) { _, ok, err := fs.ConfirmSubscription(ctx, sub.ConfirmToken); return ok, err }, true},
		{"confirm again", func() (bool, error) { _, ok, err := fs.ConfirmSubscription(ctx, sub.ConfirmToken); return ok, err }, true},
		{"unsub", func() (bool, error) { return fs.UnsubSubscription(ctx, sub.UnsubToken) }, true},
		{"confirm after unsub", func() (bool, error) { _, ok, err := fs.ConfirmSubscription(ctx, sub.ConfirmToken); return ok, err }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := tc.run()
			if err != nil || ok != tc.wantOK {
				t.Fatalf("ok=%v err=%v, want ok=%v", ok, err, tc.wantOK)
			}
		})
	}

	got := fs.Subscriptions()[0]
	if got.Confirmed() {
		t.Fatalf("unsubscribed row must not count as confirmed: %+v", got)
	}
	if want := fixturetest.Now.Add(time.Minute); !got.ConfirmedAt.Equal(want) || !got.UnsubbedAt.Equal(want) {
		t.Fatalf("timestamps = %v / %v, want %v", got.ConfirmedAt, got.UnsubbedAt, want)
	}
}

func TestFakeStore_DrivesHTTPAPI(t *testing.T) {
	srv := httpapi.New(httpapi.Config{Store: fixturetest.NewFakeStore(), Mailer: notify.NewFakeMailer(), PublicBaseURL: "http://localhost"})
	for _, path := range []string{"/api/health", "/api/products", "/api/sources", "/api/availability?plz=" + fixturetest.PLZGoeppingen} {
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}
