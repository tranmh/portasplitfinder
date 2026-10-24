package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// healthServer serves /api/health with the given status on 127.0.0.1 and points
// API_PORT at it.
func healthServer(t *testing.T, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("API_PORT", strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port))
}

func TestRunHealthcheck_StatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{http.StatusOK, 0},
		{http.StatusNoContent, 1},
		{http.StatusNotFound, 1},
		{http.StatusInternalServerError, 1},
		{http.StatusServiceUnavailable, 1},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			healthServer(t, c.status)
			if got := runHealthcheck(); got != c.want {
				t.Errorf("runHealthcheck() with %d = %d, want %d", c.status, got, c.want)
			}
		})
	}
}

func TestRunHealthcheck_ServerDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	t.Setenv("API_PORT", strconv.Itoa(port))
	if got := runHealthcheck(); got != 1 {
		t.Errorf("runHealthcheck() with nothing listening = %d, want 1", got)
	}
}

func TestGetenvDefault(t *testing.T) {
	const key = "AQAA_TEST_GETENV_DEFAULT"
	cases := []struct {
		name, val, def, want string
	}{
		{"empty uses default", "", "fallback", "fallback"},
		{"set value wins", "value", "fallback", "value"},
		{"whitespace is a value", " ", "fallback", " "},
		{"literal false", "false", "true", "false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(key, c.val)
			if got := getenvDefault(key, c.def); got != c.want {
				t.Errorf("getenvDefault = %q, want %q", got, c.want)
			}
		})
	}
	if got := getenvDefault("AQAA_TEST_SURELY_UNSET_VARIABLE", "d"); got != "d" {
		t.Errorf("unset variable: got %q, want d", got)
	}
}
