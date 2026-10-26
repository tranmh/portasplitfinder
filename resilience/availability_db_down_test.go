package resilience_test

import (
	"net/http"
	"testing"
	"time"
)

// With the database down, GET /api/availability answers a clear error and
// answers normally again once the database is back.
func TestIntegrationResilienceAvailabilityDBDown(t *testing.T) {
	base := setup(t)
	const seconds = 10
	limit := seconds * time.Second

	injectFault(t, base, "down", "postgres", seconds)
	r := call(http.MethodGet, "/api/availability?plz=73035&radius=50", "", limit)
	assertClearError(t, r, "GET /api/availability with postgres down")

	clearFaults(t, base)
	waitRecovered(t, http.MethodGet, "/api/availability?plz=73035&radius=50", "", http.StatusOK, limit)
}
