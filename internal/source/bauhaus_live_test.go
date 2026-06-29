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

// TestBauhaus_Live performs a best-effort real fetch of per-market PortaSplit
// availability across all German BAUHAUS Fachcentren. It runs only when
// BAUHAUS_LIVE=1 so it never affects normal CI runs. BAUHAUS is behind
// Cloudflare, so the test soft-skips (t.Skip) when blocked and otherwise asserts
// a sensible store-keyed result.
func TestBauhaus_Live(t *testing.T) {
	if os.Getenv("BAUHAUS_LIVE") != "1" {
		t.Skip("set BAUHAUS_LIVE=1 to run the live BAUHAUS fetch")
	}

	s := newBauhausSource()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	listings, stores, err := s.Fetch(ctx, []domain.Product{mideaProduct})
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			t.Skipf("BAUHAUS blocked the live request (expected, Cloudflare): %v", err)
		}
		t.Fatalf("live fetch failed for a non-block reason: %v", err)
	}
	if len(stores) == 0 {
		t.Fatal("got no BAUHAUS markets from the live directory")
	}
	if len(listings) == 0 {
		t.Fatal("got no per-market listings")
	}

	inStock := 0
	for _, l := range listings {
		if l.SourceKey != "bauhaus" || l.ProductID != mideaProduct.ID || l.StoreID == nil {
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
	t.Logf("live BAUHAUS: %d markets, %d listings, %d in stock", len(stores), len(listings), inStock)
}
