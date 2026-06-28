package geo

import (
	"math"
	"testing"
)

func TestDistanceKm_BerlinMunich(t *testing.T) {
	berlin, ok := Lookup("10115")
	if !ok {
		t.Fatal("Berlin 10115 not found")
	}
	munich, ok := Lookup("80331")
	if !ok {
		t.Fatal("Munich 80331 not found")
	}
	d := DistanceKm(berlin, munich)
	// Straight-line Berlin <-> Munich is ~504 km.
	if math.Abs(d-504) > 15 {
		t.Fatalf("Berlin<->Munich = %.1f km, want ~504", d)
	}
}

func TestDistanceKm_Symmetric(t *testing.T) {
	a := Point{Lat: 52.52, Lon: 13.40}
	b := Point{Lat: 48.14, Lon: 11.58}
	if math.Abs(DistanceKm(a, b)-DistanceKm(b, a)) > 1e-9 {
		t.Fatal("distance is not symmetric")
	}
}

func TestDistanceKm_Zero(t *testing.T) {
	p := Point{Lat: 50.0, Lon: 10.0}
	if d := DistanceKm(p, p); d > 1e-9 {
		t.Fatalf("distance to self = %.6f, want 0", d)
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("10115"); !ok {
		t.Fatal("expected to find 10115")
	}
	if _, ok := Lookup("00000"); ok {
		t.Fatal("did not expect to find 00000")
	}
	if _, ok := Lookup(" 10115 "); !ok {
		t.Fatal("expected trimming to find 10115")
	}
}

func TestWithinRadius(t *testing.T) {
	berlin, _ := Lookup("10115")
	munich, _ := Lookup("80331")
	if WithinRadius(berlin, munich, 100) {
		t.Fatal("Munich should not be within 100km of Berlin")
	}
	if !WithinRadius(berlin, munich, 600) {
		t.Fatal("Munich should be within 600km of Berlin")
	}
}

func TestCount(t *testing.T) {
	if Count() < 60 {
		t.Fatalf("expected >= 60 centroids, got %d", Count())
	}
}
