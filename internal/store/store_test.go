package store

import (
	"context"
	"testing"
	"time"
)

// unreachable points at a port nothing listens on, so connect fails fast
// (connection refused) without needing a real database.
const unreachable = "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"

func TestWaitReadyTimesOut(t *testing.T) {
	start := time.Now()
	err := WaitReady(context.Background(), unreachable, 600*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when DB is unreachable, got nil")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("WaitReady ran far past maxWait: %s", elapsed)
	}
}

func TestWaitReadyRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	err := WaitReady(ctx, unreachable, 60*time.Second)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}
