//go:build integration

package main

import (
	"strings"
	"testing"
	"time"
)

func TestIntegrationMain_MigrateSeedServe(t *testing.T) {
	dsn := getenvDefault("AQAA_PG_URL", "")
	if dsn == "" {
		t.Skip("AQAA_PG_URL not set")
	}
	dbEnv := "DATABASE_URL=" + dsn

	code, out := runMain(t, 90*time.Second, "-migrate-only", dbEnv)
	if code != 0 || !strings.Contains(out, "migrations applied") {
		t.Fatalf("-migrate-only: code %d, output:\n%s", code, out)
	}

	code, out = runMain(t, 90*time.Second, "-seed", dbEnv)
	if code != 0 || !strings.Contains(out, "seed complete") {
		t.Fatalf("-seed: code %d, output:\n%s", code, out)
	}

	// Catalogue is now populated: booting must not auto-seed again.
	code, out = runMain(t, 5*time.Second, "", dbEnv, "API_PORT=18089")
	if code != -1 {
		t.Fatalf("server exited early (code %d); output:\n%s", code, out)
	}
	if !strings.Contains(out, "api listening") {
		t.Fatalf("server did not start; output:\n%s", out)
	}
	if strings.Contains(out, "auto-seed") {
		t.Fatalf("auto-seed ran on a non-empty catalogue; output:\n%s", out)
	}
}
