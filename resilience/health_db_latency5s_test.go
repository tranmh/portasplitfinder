package resilience_test

import (
	"net/http"
	"testing"
	"time"
)

// With +5 s on every database answer, GET /api/health still answers within
// the fault's duration (slow or degraded, but no hang), and answers 200 once
// the fault is off.
func TestIntegrationResilienceHealthDBLatency5s(t *testing.T) {
	base := setup(t)
	const seconds = 15
	limit := seconds * time.Second

	injectFault(t, base, "latency-5s", "postgres", seconds)
	r := call(http.MethodGet, "/api/health", "", limit)
	if r.err != nil {
		t.Fatalf("GET /api/health with +5s postgres latency did not answer within the fault (%ds): %v", seconds, r.err)
	}
	if r.status != http.StatusOK && r.status != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/health: want 200 or 503, got %d: %s", r.status, r.body)
	}

	clearFaults(t, base)
	waitRecovered(t, http.MethodGet, "/api/health", "", http.StatusOK, limit)
}
