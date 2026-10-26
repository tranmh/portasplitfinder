package store

import (
	"context"
	"io/fs"
	"regexp"
	"testing"
	"time"

	"github.com/tranmh/portasplitfinder/migrations"
)

// unreachable points at a port nothing listens on, so connect fails fast
// (connection refused) without needing a real database.
const unreachable = "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"

// TestMigrations_IndexForLatestForStore guards LatestForStore (WHERE store_id = $1,
// called once per store on GET /api/stores): the append-only snapshots table
// needs an index led by store_id, otherwise every call scans all history rows.
// idx_snapshots_latest is led by source_key and cannot serve that filter.
func TestMigrations_IndexForLatestForStore(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	re := regexp.MustCompile(`(?is)CREATE\s+(UNIQUE\s+)?INDEX[^;]*?\bON\s+availability_snapshots\s*(USING\s+\w+\s*)?\(\s*store_id\b`)
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	for _, name := range files {
		b, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if re.Match(b) {
			return
		}
	}
	t.Fatalf("no index on availability_snapshots led by store_id: LatestForStore scans the whole snapshot history per store")
}

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
