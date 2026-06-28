//go:build integration

package source

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// TestOBI_Live performs a best-effort real fetch of per-market PortaSplit
// availability across all German OBI markets. It runs only when OBI_LIVE=1 so it
// never affects normal CI runs. OBI is bot-protected, so the test soft-skips
// (t.Skip) when blocked and otherwise asserts a sensible store-keyed result.
func TestOBI_Live(t *testing.T) {
	if os.Getenv("OBI_LIVE") != "1" {
		t.Skip("set OBI_LIVE=1 to run the live OBI fetch")
	}

	s := newOBISource()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	listings, stores, err := s.Fetch(ctx, []domain.Product{mideaProduct})
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			t.Skipf("OBI blocked the live request (expected): %v", err)
		}
		t.Fatalf("live fetch failed for a non-block reason: %v", err)
	}
	if len(stores) == 0 {
		t.Fatal("got no OBI markets from the live directory")
	}
	if len(listings) == 0 {
		t.Fatal("got no per-market listings")
	}

	inStock := 0
	for _, l := range listings {
		if l.SourceKey != "obi" || l.ProductID != mideaProduct.ID || l.StoreID == nil {
			t.Fatalf("unexpected listing shape: %+v", l)
		}
		if l.InStock {
			inStock++
		}
	}
	for _, st := range stores {
		if st.Lat == 0 || st.Lon == 0 {
			t.Errorf("store %q has zero coordinates", st.Name)
		}
	}
	t.Logf("live OBI: %d markets, %d listings, %d in stock", len(stores), len(listings), inStock)
}
