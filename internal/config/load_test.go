package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var allKeys = []string{
	"DATABASE_URL", "API_PORT", "PUBLIC_BASE_URL", "SMTP_HOST", "SMTP_PORT",
	"SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS", "SOURCES",
	"UNBLOCKER_URL", "POLL_INTERVAL", "POLLER_HEALTH_PORT",
}

// cleanEnv blanks every variable Load reads (blank counts as unset) and sets a
// valid DATABASE_URL.
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/psf?sslmode=disable")
}

func TestLoad_AllDefaults(t *testing.T) {
	cleanEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		DatabaseURL:      "postgres://u:p@db:5432/psf?sslmode=disable",
		APIPort:          8080,
		PublicBaseURL:    "http://localhost:8080",
		SMTPHost:         "mailhog",
		SMTPPort:         1025,
		SMTPFrom:         "PortaSplitFinder <noreply@portasplitfinder.local>",
		SMTPTLS:          false,
		Sources:          []string{"mock"},
		PollInterval:     5 * time.Minute,
		PollerHealthPort: 8090,
	}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("Load() = %+v\nwant     %+v", c, want)
	}
}

func TestLoad_MissingDatabaseURLWithValidRest(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("API_PORT", "9000")
	c, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is empty")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error %q should name DATABASE_URL", err)
	}
	if !reflect.DeepEqual(c, Config{}) {
		t.Errorf("config should be zero on error, got %+v", c)
	}
}

func TestLoad_InvalidInts(t *testing.T) {
	cases := []struct{ key, val string }{
		{"API_PORT", "notanint"},
		{"API_PORT", "80.5"},
		{"API_PORT", " 8080"},
		{"SMTP_PORT", "smtp"},
		{"SMTP_PORT", "1e3"},
		{"POLLER_HEALTH_PORT", "x"},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.val, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv(c.key, c.val)
			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for %s=%q", c.key, c.val)
			}
			if !strings.Contains(err.Error(), c.key) {
				t.Errorf("error %q should name %s", err, c.key)
			}
		})
	}
}

func TestLoad_IntOverrides(t *testing.T) {
	cleanEnv(t)
	t.Setenv("API_PORT", "9000")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("POLLER_HEALTH_PORT", "9090")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.APIPort != 9000 || c.SMTPPort != 587 || c.PollerHealthPort != 9090 {
		t.Errorf("ports = %d/%d/%d, want 9000/587/9090", c.APIPort, c.SMTPPort, c.PollerHealthPort)
	}
}

func TestLoad_Durations(t *testing.T) {
	cases := []struct {
		val     string
		want    time.Duration
		wantErr bool
	}{
		{"30s", 30 * time.Second, false},
		{"1h30m", 90 * time.Minute, false},
		{"250ms", 250 * time.Millisecond, false},
		{"5", 0, true},
		{"5quux", 0, true},
		{"five minutes", 0, true},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("POLL_INTERVAL", c.val)
			got, err := Load()
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for POLL_INTERVAL=%q", c.val)
				}
				if !strings.Contains(err.Error(), "POLL_INTERVAL") {
					t.Errorf("error %q should name POLL_INTERVAL", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.PollInterval != c.want {
				t.Errorf("PollInterval = %v, want %v", got.PollInterval, c.want)
			}
		})
	}
}

func TestLoad_SMTPTLSBool(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"on", true},
		{" true ", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"maybe", false},
	}
	for _, c := range cases {
		t.Run("SMTP_TLS="+c.val, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("SMTP_TLS", c.val)
			got, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.SMTPTLS != c.want {
				t.Errorf("SMTPTLS = %v, want %v", got.SMTPTLS, c.want)
			}
		})
	}
}

func TestLoad_Sources(t *testing.T) {
	cases := []struct {
		val  string
		want []string
	}{
		{"", []string{"mock"}},
		{"obi", []string{"obi"}},
		{"mock,obi,bauhaus", []string{"mock", "obi", "bauhaus"}},
		{" mock , obi ,amazon ", []string{"mock", "obi", "amazon"}},
		{"mock,,obi", []string{"mock", "obi"}},
		{", ,obi, ,", []string{"obi"}},
		{" , , ", []string{}},
	}
	for _, c := range cases {
		t.Run("SOURCES="+c.val, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("SOURCES", c.val)
			got, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Sources) != len(c.want) || (len(c.want) > 0 && !reflect.DeepEqual(got.Sources, c.want)) {
				t.Errorf("Sources = %q, want %q", got.Sources, c.want)
			}
		})
	}
}

func TestLoad_StringOverrides(t *testing.T) {
	cleanEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "https://psf.example.de")
	t.Setenv("SMTP_HOST", "smtp.example.de")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM", "PSF <psf@example.de>")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicBaseURL != "https://psf.example.de" || c.SMTPHost != "smtp.example.de" ||
		c.SMTPUsername != "user" || c.SMTPPassword != "secret" || c.SMTPFrom != "PSF <psf@example.de>" {
		t.Errorf("string overrides not applied: %+v", c)
	}
}
