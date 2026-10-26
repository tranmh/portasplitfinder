package geo

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func samePoint(a, b Point) bool {
	eq := func(x, y float64) bool { return x == y || (math.IsNaN(x) && math.IsNaN(y)) }
	return eq(a.Lat, b.Lat) && eq(a.Lon, b.Lon)
}

// FuzzParseCSV checks that parseCSV never panics and that a parsed table,
// written back as CSV, parses to the same table.
func FuzzParseCSV(f *testing.F) {
	f.Add([]byte("plz,lat,lon\n10115,52.53,13.38\n80331,48.13,11.57\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n\n  \n"))
	f.Add([]byte("10115,52.53\n"))
	f.Add([]byte("10115,abc,13.38\n"))
	f.Add([]byte("plz,lat,lon\r\n 01067 , 51.05 , 13.73 \r\n"))
	f.Add([]byte("x,NaN,+Inf\nx,1e308,-0\n"))
	f.Add([]byte("\xff,1,2\n"))
	f.Add([]byte(strings.Repeat("9", 70000) + ",1,2\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := parseCSV(data)
		if err != nil {
			return
		}
		var b strings.Builder
		b.WriteString("plz,lat,lon\n")
		for plz, p := range m {
			b.WriteString(plz + "," + strconv.FormatFloat(p.Lat, 'g', -1, 64) + "," + strconv.FormatFloat(p.Lon, 'g', -1, 64) + "\n")
		}
		m2, err := parseCSV([]byte(b.String()))
		if err != nil {
			t.Fatalf("re-parse of serialised table from %q failed: %v", data, err)
		}
		if len(m2) != len(m) {
			t.Fatalf("input %q: %d entries, after round trip %d", data, len(m), len(m2))
		}
		for plz, p := range m {
			if p2, ok := m2[plz]; !ok || !samePoint(p, p2) {
				t.Errorf("input %q: entry %q = %v, after round trip %v (found %v)", data, plz, p, p2, ok)
			}
		}
	})
}

// FuzzDistanceKm checks that for valid coordinates the haversine distance is a
// number in [0, half the Earth's circumference] and symmetric.
func FuzzDistanceKm(f *testing.F) {
	f.Add(52.52, 13.405, 48.137, 11.575)
	f.Add(0.0, 0.0, 0.0, 0.0)
	f.Add(90.0, 0.0, -90.0, 0.0)
	f.Add(0.0, -180.0, 0.0, 180.0)
	f.Add(0.0, 0.0, 0.0, 180.0)
	f.Add(45.0, 10.0, -45.0, -170.0)
	f.Add(1e-300, 0.0, -1e-300, 5e-324)
	f.Fuzz(func(t *testing.T, lat1, lon1, lat2, lon2 float64) {
		for _, v := range []float64{lat1, lat2} {
			if !(v >= -90 && v <= 90) {
				return
			}
		}
		for _, v := range []float64{lon1, lon2} {
			if !(v >= -180 && v <= 180) {
				return
			}
		}
		a, b := Point{lat1, lon1}, Point{lat2, lon2}
		d := DistanceKm(a, b)
		maxD := math.Pi*earthRadiusKm + 1e-6
		if math.IsNaN(d) || d < 0 || d > maxD {
			t.Fatalf("DistanceKm(%v, %v) = %v, want within [0, %v]", a, b, d, maxD)
		}
		if back := DistanceKm(b, a); math.Abs(back-d) > 1e-6 {
			t.Errorf("DistanceKm not symmetric: (%v, %v) = %v, reversed = %v", a, b, d, back)
		}
	})
}
