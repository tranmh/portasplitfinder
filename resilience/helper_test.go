package resilience_test

// Shared helpers for the resilience tests: the fault switch (AQAA_FAULT_URL)
// and the api's URL (AQAA_APP_API_URL). This file defines no test.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// faultURL returns the fault switch's URL or skips the test when it is unset.
func faultURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("AQAA_FAULT_URL")
	if u == "" {
		t.Skip("AQAA_FAULT_URL not set")
	}
	return strings.TrimRight(u, "/")
}

func apiURL() string {
	if u := os.Getenv("AQAA_APP_API_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:8080"
}

// clearFaults turns every fault off.
func clearFaults(t *testing.T, base string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, base+"/faults", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("clear faults: %v", err)
		return
	}
	resp.Body.Close()
}

// setup skips without the fault switch and turns every fault off now and at
// the end of the test (also when it fails).
func setup(t *testing.T) string {
	t.Helper()
	base := faultURL(t)
	clearFaults(t, base)
	t.Cleanup(func() { clearFaults(t, base) })
	return base
}

// injectFault turns one fault on for the given seconds.
func injectFault(t *testing.T, base, scenario, proxy string, seconds int) {
	t.Helper()
	u := fmt.Sprintf("%s/faults/%s?proxy=%s&seconds=%d", base, scenario, proxy, seconds)
	resp, err := http.Post(u, "application/json", nil)
	if err != nil {
		t.Fatalf("inject %s: %v", scenario, err)
	}
	defer resp.Body.Close()
	var body struct {
		Error any `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode >= 300 || (body.Error != nil && body.Error != "") {
		t.Fatalf("inject %s: status %d, error %v", scenario, resp.StatusCode, body.Error)
	}
}

type result struct {
	status int
	body   string
	err    error
}

// call sends a request to the api with a client limited to limit (the fault's
// own duration): an answer that has not come when the fault ends has hung for
// the whole fault.
func call(method, path, body string, limit time.Duration) result {
	c := &http.Client{Timeout: limit}
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, apiURL()+path, r)
	if err != nil {
		return result{err: err}
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return result{err: err}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return result{status: resp.StatusCode, body: string(b)}
}

// assertClearError checks for an error status with a JSON error body.
func assertClearError(t *testing.T, r result, what string) {
	t.Helper()
	if r.err != nil {
		t.Fatalf("%s: no answer while the fault was on (hang or dropped connection): %v", what, r.err)
	}
	if r.status < 500 {
		t.Fatalf("%s: want an error status (5xx) while the database is faulty, got %d: %s", what, r.status, r.body)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil || len(m) == 0 {
		t.Fatalf("%s: want a JSON error body, got %q", what, r.body)
	}
}

// waitRecovered polls path until it answers wantStatus, for at most the
// fault's own duration after the fault is off.
func waitRecovered(t *testing.T, method, path, body string, wantStatus int, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	var last result
	for time.Now().Before(deadline) {
		last = call(method, path, body, limit)
		if last.err == nil && last.status == wantStatus {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s %s did not answer %d again after the fault was off: status %d, err %v, body %q",
		method, path, wantStatus, last.status, last.err, last.body)
}
