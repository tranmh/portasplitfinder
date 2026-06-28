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

// TestOBI_Live performs a best-effort real fetch of the Midea PortaSplit page.
// It runs only when OBI_LIVE=1 so it never affects normal CI runs. OBI is
// bot-protected, so the test soft-skips (t.Skip) when blocked and otherwise
// asserts a sensible parsed listing.
func TestOBI_Live(t *testing.T) {
	if os.Getenv("OBI_LIVE") != "1" {
		t.Skip("set OBI_LIVE=1 to run the live OBI fetch")
	}

	s := newOBISource()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	listings, _, err := s.Fetch(ctx, []domain.Product{mideaProduct})
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			t.Skipf("OBI blocked the live request (expected): %v", err)
		}
		t.Fatalf("live fetch failed for a non-block reason: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("got %d listings, want 1", len(listings))
	}
	l := listings[0]
	if l.SourceKey != "obi" || l.ProductID != mideaProduct.ID || l.StoreID != nil {
		t.Fatalf("unexpected listing shape: %+v", l)
	}
	if l.Currency == "" {
		t.Errorf("empty currency in live listing: %+v", l)
	}
	t.Logf("live OBI listing: inStock=%v priceCents=%v url=%s", l.InStock, l.PriceCents, l.URL)
}
