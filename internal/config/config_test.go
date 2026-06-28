package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x/y")
	// Ensure optional vars are unset.
	for _, k := range []string{"API_PORT", "SMTP_PORT", "POLLER_HEALTH_PORT", "POLL_INTERVAL", "SMTP_TLS", "SOURCES", "PUBLIC_BASE_URL"} {
		t.Setenv(k, "")
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.APIPort != 8080 {
		t.Errorf("APIPort = %d, want 8080", c.APIPort)
	}
	if c.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", c.PollInterval)
	}
	if c.PollerHealthPort != 8090 {
		t.Errorf("PollerHealthPort = %d, want 8090", c.PollerHealthPort)
	}
	if c.SMTPTLS {
		t.Error("SMTPTLS should default false")
	}
	if len(c.Sources) != 1 || c.Sources[0] != "mock" {
		t.Errorf("Sources = %v, want [mock]", c.Sources)
	}
	if c.PublicBaseURL != "http://localhost:8080" {
		t.Errorf("PublicBaseURL = %q", c.PublicBaseURL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x/y")
	t.Setenv("API_PORT", "9000")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("SMTP_TLS", "true")
	t.Setenv("SOURCES", "mock, obi ,amazon")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.APIPort != 9000 {
		t.Errorf("APIPort = %d", c.APIPort)
	}
	if c.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v", c.PollInterval)
	}
	if !c.SMTPTLS {
		t.Error("SMTPTLS should be true")
	}
	if len(c.Sources) != 3 {
		t.Fatalf("Sources = %v, want 3 entries", c.Sources)
	}
	if c.Sources[1] != "obi" {
		t.Errorf("Sources not trimmed: %q", c.Sources[1])
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoad_BadInt(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x/y")
	t.Setenv("API_PORT", "notanint")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad API_PORT")
	}
}

func TestLoad_BadDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x/y")
	t.Setenv("POLL_INTERVAL", "5quux")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad POLL_INTERVAL")
	}
}
