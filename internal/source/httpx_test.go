package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(opts ...ClientOption) *Client {
	// Default to zero backoff so tests never sleep.
	base := []ClientOption{WithBaseBackoff(0)}
	return NewClient(append(base, opts...)...)
}

func TestClient_SendsBrowserHeaders(t *testing.T) {
	var gotUA, gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotLang = r.Header.Get("Accept-Language")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	if _, err := testClient().GetHTML(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if gotUA == "" || gotUA != defaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, defaultUserAgent)
	}
	if gotLang == "" || gotLang[:5] != "de-DE" {
		t.Errorf("Accept-Language = %q, want de-DE...", gotLang)
	}
}

func TestClient_RetryThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	body, err := testClient(WithMaxAttempts(3)).GetHTML(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if string(body) != "recovered" {
		t.Fatalf("body = %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("server called %d times, want 3", got)
	}
}

func TestClient_RetryExhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := testClient(WithMaxAttempts(3)).GetHTML(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if errors.Is(err, ErrBlocked) {
		t.Fatal("5xx should not be ErrBlocked")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("server called %d times, want 3 (maxAttempts)", got)
	}
}

func TestClient_BlockedStatuses(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests} {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(code)
		}))

		_, err := testClient(WithMaxAttempts(3)).GetHTML(context.Background(), srv.URL)
		if !errors.Is(err, ErrBlocked) {
			t.Errorf("status %d: got err %v, want ErrBlocked", code, err)
		}
		// Blocked statuses must not be retried.
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("status %d: server called %d times, want 1 (no retry)", code, got)
		}
		srv.Close()
	}
}

func TestClient_Unblocker_RewritesURL(t *testing.T) {
	const target = "https://www.bauhaus.info/api/stores-search?storeId=520"
	var gotTarget string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client should call the unblocker with the target URL-encoded into ?u=.
		gotTarget = r.URL.Query().Get("u")
		_, _ = w.Write([]byte(`{"stores":[]}`))
	}))
	defer srv.Close()

	tmpl := srv.URL + "/v1/?api_key=k&premium_proxy=true&u={{url}}"
	body, err := testClient(WithUnblocker(tmpl)).GetHTML(context.Background(), target)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotTarget != target {
		t.Errorf("unblocker received target %q, want %q", gotTarget, target)
	}
	if string(body) != `{"stores":[]}` {
		t.Errorf("body = %q, want the unblocked target body", body)
	}
}

func TestClient_NoUnblocker_FetchesDirect(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	if _, err := testClient().GetHTML(context.Background(), srv.URL+"/api/stores-search"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/stores-search" {
		t.Errorf("direct fetch path = %q, want /api/stores-search", gotPath)
	}
}

func TestClient_Unblocker_403StillBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // unblocker itself returns blocked
	}))
	defer srv.Close()

	_, err := testClient(WithUnblocker(srv.URL+"/?u={{url}}")).GetHTML(context.Background(), "https://www.bauhaus.info/x")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("got %v, want ErrBlocked", err)
	}
}

func TestClient_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// With a real (non-zero) backoff, a cancelled context aborts before sleeping.
	_, err := NewClient(WithBaseBackoff(time.Second), WithMaxAttempts(3)).GetHTML(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
