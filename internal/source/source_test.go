package source

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestBuild_Enabled(t *testing.T) {
	srcs, err := Build([]string{"mock", "obi", "amazon"})
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 3 {
		t.Fatalf("got %d sources, want 3", len(srcs))
	}
	keys := []string{srcs[0].Key(), srcs[1].Key(), srcs[2].Key()}
	if !reflect.DeepEqual(keys, []string{"mock", "obi", "amazon"}) {
		t.Fatalf("order not preserved: %v", keys)
	}
}

func TestBuild_MockObiBauhaus(t *testing.T) {
	srcs, err := Build([]string{"mock", "obi", "bauhaus"})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{srcs[0].Key(), srcs[1].Key(), srcs[2].Key()}
	if !reflect.DeepEqual(keys, []string{"mock", "obi", "bauhaus"}) {
		t.Fatalf("order not preserved: %v", keys)
	}
}

func TestBuild_Unknown(t *testing.T) {
	if _, err := Build([]string{"nope"}); err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestKnown(t *testing.T) {
	for _, k := range []string{"mock", "obi", "bauhaus", "ebay", "amazon", "guenstiger", "idealo"} {
		if !Known(k) {
			t.Errorf("expected %q to be known", k)
		}
	}
	if Known("xxx") {
		t.Error("xxx should not be known")
	}
}

func TestStubsReturnNotImplemented(t *testing.T) {
	// obi is now a real adapter (tested in obi_test.go); the remaining stubs
	// still return ErrNotImplemented.
	for _, k := range []string{"ebay", "amazon", "guenstiger", "idealo"} {
		srcs, _ := Build([]string{k})
		_, _, err := srcs[0].Fetch(context.Background(), nil)
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("source %q: got err %v, want ErrNotImplemented", k, err)
		}
	}
}

func TestMockDeterministic(t *testing.T) {
	a := NewMockWithTransition(false)
	b := NewMockWithTransition(false)
	la, sa, err := a.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	lb, sb, _ := b.Fetch(context.Background(), nil)
	if !reflect.DeepEqual(la, lb) {
		t.Fatal("mock listings are not deterministic")
	}
	if !reflect.DeepEqual(sa, sb) {
		t.Fatal("mock stores are not deterministic")
	}
	if len(sa) != 5 {
		t.Fatalf("got %d stores, want 5", len(sa))
	}
}

func TestMockTransitionToggle(t *testing.T) {
	off := NewMockWithTransition(false)
	on := NewMockWithTransition(true)

	findMunich := func(s *MockSource) bool {
		listings, _, _ := s.Fetch(context.Background(), nil)
		for _, l := range listings {
			if l.ProductID == MockProductMideaID && l.StoreID != nil && *l.StoreID == mockStoreMunichID {
				return l.InStock
			}
		}
		t.Fatal("munich/midea listing not found")
		return false
	}

	if findMunich(off) {
		t.Fatal("with toggle off, munich should be out of stock")
	}
	if !findMunich(on) {
		t.Fatal("with toggle on, munich should be in stock")
	}
}

func TestMockSeedCatalogue(t *testing.T) {
	m := NewMock()
	if len(m.Products()) != 2 {
		t.Fatalf("want 2 products, got %d", len(m.Products()))
	}
	// mock store source + 4 online retailers = 5 sources.
	if len(m.SeedSources()) != 5 {
		t.Fatalf("want 5 seed sources, got %d", len(m.SeedSources()))
	}
}
