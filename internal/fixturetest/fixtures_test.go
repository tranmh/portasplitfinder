package fixturetest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/fixturetest"
	"github.com/tranmh/portasplitfinder/internal/httpapi"
	"github.com/tranmh/portasplitfinder/internal/jobs"
)

var (
	_ httpapi.DataStore = (*fixturetest.FakeStore)(nil)
	_ jobs.Store        = (*fixturetest.FakeStore)(nil)
)

func TestFakeStoreSeeded(t *testing.T) {
	ctx := context.Background()
	f := fixturetest.NewSeededFakeStore()

	snaps, err := f.LatestSnapshots(ctx)
	if err != nil || len(snaps) != 2 {
		t.Fatalf("LatestSnapshots = %d, %v; want 2 rows", len(snaps), err)
	}
	if s, ok, _ := f.LatestForKey(ctx, fixturetest.SourceKeyStore, fixturetest.StoreID, fixturetest.ProductID); !ok || !s.InStock {
		t.Fatalf("LatestForKey = %+v, %v; want newest in-stock row", s, ok)
	}

	email, ok, err := f.ConfirmSubscription(ctx, "confirm-token")
	if err != nil || !ok || email != "user@example.com" {
		t.Fatalf("ConfirmSubscription = %q, %v, %v", email, ok, err)
	}
	if ok, _ := f.UnsubSubscription(ctx, "unsub-token"); !ok {
		t.Fatal("UnsubSubscription: want ok")
	}
	if subs, _ := f.ListConfirmedSubscriptions(ctx); len(subs) != 0 {
		t.Fatalf("ListConfirmedSubscriptions = %d; want 0 after unsub", len(subs))
	}
	if first, _ := f.RecordNotification(ctx, "s", "", "", "", "k"); !first {
		t.Fatal("first RecordNotification: want inserted")
	}
	if again, _ := f.RecordNotification(ctx, "s", "", "", "", "k"); again {
		t.Fatal("second RecordNotification: want deduped")
	}
	if !strings.Contains(fixturetest.SeedSQL, fixturetest.SubscriptionID) {
		t.Fatal("SeedSQL not embedded")
	}
}
