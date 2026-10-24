package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/internal/notify"
)

func TestCORS_OriginFollowsWebOrigin(t *testing.T) {
	cases := []struct {
		name      string
		webOrigin string
		want      string
	}{
		{"empty falls back to wildcard", "", "*"},
		{"configured origin", "http://localhost:5173", "http://localhost:5173"},
		{"production origin", "https://portasplit.example.de", "https://portasplit.example.de"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(Config{Store: newFakeStore(), Mailer: notify.NewFakeMailer(), WebOrigin: c.webOrigin})
			rr := do(s, http.MethodGet, "/api/health", nil)
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != c.want {
				t.Errorf("Allow-Origin = %q, want %q", got, c.want)
			}
			methods := rr.Header().Get("Access-Control-Allow-Methods")
			for _, m := range []string{"GET", "POST", "OPTIONS"} {
				if !strings.Contains(methods, m) {
					t.Errorf("Allow-Methods = %q, missing %s", methods, m)
				}
			}
			if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
				t.Errorf("Allow-Headers = %q, want Content-Type", got)
			}
		})
	}
}

func TestCORS_PreflightReturns204WithoutReachingHandler(t *testing.T) {
	for _, path := range []string{"/api/subscriptions", "/api/availability", "/api/health"} {
		t.Run(path, func(t *testing.T) {
			fs := newFakeStore()
			fs.pingErr = errors.New("must not be called")
			s, mailer := newTestServer(fs)
			s.webOrigin = "http://localhost:5173"
			rr := do(s, http.MethodOptions, path, nil)
			if rr.Code != http.StatusNoContent {
				t.Fatalf("status %d, want 204", rr.Code)
			}
			if rr.Body.Len() != 0 {
				t.Errorf("preflight body = %q, want empty", rr.Body.String())
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
				t.Errorf("Allow-Origin = %q", got)
			}
			if fs.created != 0 || mailer.Count() != 0 {
				t.Errorf("preflight reached the handler: created=%d mails=%d", fs.created, mailer.Count())
			}
		})
	}
}

func TestHealth_Body(t *testing.T) {
	cases := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantState  string
		wantDB     string
	}{
		{"db ok", nil, http.StatusOK, "ok", "ok"},
		{"db down", errors.New("connection refused"), http.StatusServiceUnavailable, "degraded", "error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fs := newFakeStore()
			fs.pingErr = c.pingErr
			s, _ := newTestServer(fs)
			rr := do(s, http.MethodGet, "/api/health", nil)
			if rr.Code != c.wantStatus {
				t.Fatalf("status %d, want %d", rr.Code, c.wantStatus)
			}
			if ct := rr.Header().Get("Content-Type"); ct != jsonContentType {
				t.Errorf("Content-Type = %q, want %q", ct, jsonContentType)
			}
			m := decodeObject(t, rr)
			if m["status"] != c.wantState {
				t.Errorf("status = %v, want %q", m["status"], c.wantState)
			}
			if m["db"] != c.wantDB {
				t.Errorf("db = %v, want %q", m["db"], c.wantDB)
			}
			ts, _ := m["time"].(string)
			parsed, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				t.Fatalf("time %q is not RFC 3339: %v", ts, err)
			}
			if !strings.HasSuffix(ts, "Z") || parsed.Location() != time.UTC {
				t.Errorf("time %q is not UTC", ts)
			}
		})
	}
}
