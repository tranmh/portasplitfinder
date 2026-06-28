// Package geo provides German PLZ (postal code) centroid lookups and
// great-circle distance helpers.
//
// internal/geo/data/plz_centroids.csv holds one centroid per German postal
// code (~10,800 PLZ), derived from the GeoNames DE postal-code dataset by
// averaging the coordinates of all places sharing a PLZ.
package geo

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
)

//go:embed data/plz_centroids.csv
var centroidsCSV []byte

// earthRadiusKm is the mean Earth radius used by the haversine formula.
const earthRadiusKm = 6371.0088

// Point is a geographic coordinate.
type Point struct {
	Lat float64
	Lon float64
}

// centroids maps PLZ -> Point. Populated at package init from the embedded CSV.
var centroids map[string]Point

func init() {
	var err error
	centroids, err = parseCSV(centroidsCSV)
	if err != nil {
		panic(fmt.Sprintf("geo: failed to parse embedded centroids: %v", err))
	}
}

func parseCSV(data []byte) (map[string]Point, error) {
	m := make(map[string]Point)
	sc := bufio.NewScanner(bytes.NewReader(data))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if lineNo == 1 && strings.HasPrefix(line, "plz") {
			continue // header
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("geo: line %d: expected 3 columns, got %d", lineNo, len(parts))
		}
		plz := strings.TrimSpace(parts[0])
		lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("geo: line %d: bad lat: %w", lineNo, err)
		}
		lon, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		if err != nil {
			return nil, fmt.Errorf("geo: line %d: bad lon: %w", lineNo, err)
		}
		m[plz] = Point{Lat: lat, Lon: lon}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// Lookup returns the centroid for a PLZ and whether it was found.
func Lookup(plz string) (Point, bool) {
	p, ok := centroids[strings.TrimSpace(plz)]
	return p, ok
}

// DistanceKm returns the great-circle distance in kilometres between two points
// using the haversine formula.
func DistanceKm(a, b Point) float64 {
	lat1 := rad(a.Lat)
	lat2 := rad(b.Lat)
	dLat := rad(b.Lat - a.Lat)
	dLon := rad(b.Lon - a.Lon)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusKm * c
}

// WithinRadius reports whether b is within radiusKm of a.
func WithinRadius(a, b Point, radiusKm float64) bool {
	return DistanceKm(a, b) <= radiusKm
}

func rad(deg float64) float64 {
	return deg * math.Pi / 180
}

// Count returns the number of loaded PLZ centroids (useful for diagnostics).
func Count() int {
	return len(centroids)
}
