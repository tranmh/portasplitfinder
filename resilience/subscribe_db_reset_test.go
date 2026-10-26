package resilience_test

import (
	"net/http"
	"testing"
	"time"
)

// With dropped database connections, POST /api/subscriptions is either
// accepted (202, stored) or answered with a clear error, never a hang or a
// dropped answer; once the fault is off it is accepted again.
func TestIntegrationResilienceSubscribeDBResetPeer(t *testing.T) {
	base := setup(t)
	const seconds = 10
	limit := seconds * time.Second
	body := `{"email":"resilience-reset@example.com","plz":"73035","radiusKm":50}`

	injectFault(t, base, "reset-peer", "postgres", seconds)
	r := call(http.MethodPost, "/api/subscriptions", body, limit)
	if !(r.err == nil && r.status == http.StatusAccepted) {
		assertClearError(t, r, "POST /api/subscriptions with postgres resetting connections")
	}

	clearFaults(t, base)
	waitRecovered(t, http.MethodPost, "/api/subscriptions", body, http.StatusAccepted, limit)
}
