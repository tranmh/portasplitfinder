package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMainHelper runs main() when the test binary is re-executed by runMain.
func TestMainHelper(t *testing.T) {
	if os.Getenv("AQAA_RUN_MAIN") != "1" {
		t.Skip("helper process only")
	}
	os.Args = append([]string{"api"}, strings.Fields(os.Getenv("AQAA_MAIN_ARGS"))...)
	main()
	os.Exit(0)
}

// runMain re-executes the test binary to run main() with the given args and
// environment. It returns the exit code (-1 when killed at the timeout) and the
// combined output.
func runMain(t *testing.T, timeout time.Duration, args string, env ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainHelper$")
	cmd.Env = append([]string{
		"AQAA_RUN_MAIN=1",
		"AQAA_MAIN_ARGS=" + args,
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}, env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() != nil {
		return -1, out.String()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), out.String()
	}
	if err != nil {
		t.Fatalf("run main: %v", err)
	}
	return 0, out.String()
}

func TestMain_InvalidConfigExits1(t *testing.T) {
	code, out := runMain(t, 20*time.Second, "",
		"API_PORT=notanumber", "DATABASE_URL=postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, `"msg":"config"`) {
		t.Fatalf("expected config error log, got:\n%s", out)
	}
}

func TestMain_WaitsForUnreachableDB(t *testing.T) {
	// The database never comes up: main must keep retrying (up to 60s) instead
	// of giving up straight away.
	code, out := runMain(t, 3*time.Second, "-migrate-only",
		"DATABASE_URL=postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if code != -1 {
		t.Fatalf("main exited (code %d) before the 60s DB wait elapsed; output:\n%s", code, out)
	}
	if strings.Contains(out, "wait for db") || strings.Contains(out, `"msg":"migrate"`) {
		t.Fatalf("main gave up waiting for the db; output:\n%s", out)
	}
}
