package geo

import (
	"math"
	"testing"
)

func TestLookup_Unknown(t *testing.T) {
	for _, plz := range []string{"", "   ", "abcde", "1011", "101150", "00000", "10115x"} {
		t.Run(plz, func(t *testing.T) {
			p, ok := Lookup(plz)
			if ok {
				t.Fatalf("Lookup(%q) found %+v, want not found", plz, p)
			}
			if p != (Point{}) {
				t.Errorf("Lookup(%q) returned non-zero point %+v for a miss", plz, p)
			}
		})
	}
}

func TestLookup_KnownCentroidsInGermany(t *testing.T) {
	for _, plz := range []string{"10115", "80331", "73035", "01067", "20095", "50667"} {
		t.Run(plz, func(t *testing.T) {
			p, ok := Lookup(plz)
			if !ok {
				t.Fatalf("Lookup(%q) not found", plz)
			}
			// Bounding box of Germany.
			if p.Lat < 47 || p.Lat > 55.1 || p.Lon < 5.8 || p.Lon > 15.1 {
				t.Errorf("Lookup(%q) = %+v, outside Germany", plz, p)
			}
		})
	}
}

func TestDistanceKm_KnownValues(t *testing.T) {
	oneDegree := earthRadiusKm * math.Pi / 180 // ~111.195 km
	cases := []struct {
		name string
		a, b Point
		want float64
		tol  float64
	}{
		{"one degree of longitude on the equator", Point{0, 0}, Point{0, 1}, oneDegree, 1e-6},
		{"one degree of latitude", Point{0, 0}, Point{1, 0}, oneDegree, 1e-6},
		{"antipodal on the equator", Point{0, 0}, Point{0, 180}, math.Pi * earthRadiusKm, 1e-6},
		{"pole to pole", Point{90, 0}, Point{-90, 0}, math.Pi * earthRadiusKm, 1e-6},
		{"across the antimeridian", Point{0, 179.5}, Point{0, -179.5}, oneDegree, 1e-6},
		{"Berlin Mitte to Hamburg Altstadt", Point{52.5200, 13.4050}, Point{53.5511, 9.9937}, 255.3, 2},
		{"Esslingen to Stuttgart", Point{48.7406, 9.3108}, Point{48.7758, 9.1829}, 10.1, 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DistanceKm(c.a, c.b)
			if math.Abs(got-c.want) > c.tol {
				t.Errorf("DistanceKm = %.4f, want %.4f ± %g", got, c.want, c.tol)
			}
		})
	}
}

func TestWithinRadius_BoundaryInclusive(t *testing.T) {
	a := Point{Lat: 48.7406, Lon: 9.3108}
	b := Point{Lat: 48.7758, Lon: 9.1829}
	d := DistanceKm(a, b)
	cases := []struct {
		name   string
		radius float64
		want   bool
	}{
		{"exactly the distance", d, true},
		{"just above", math.Nextafter(d, math.Inf(1)), true},
		{"just below", math.Nextafter(d, 0), false},
		{"zero radius", 0, false},
		{"negative radius", -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WithinRadius(a, b, c.radius); got != c.want {
				t.Errorf("WithinRadius(r=%v, d=%v) = %v, want %v", c.radius, d, got, c.want)
			}
		})
	}
	if !WithinRadius(a, a, 0) {
		t.Error("a point is within radius 0 of itself")
	}
}
