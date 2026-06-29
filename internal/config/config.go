// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the API and poller.
type Config struct {
	DatabaseURL string

	APIPort       int
	PublicBaseURL string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPTLS      bool

	Sources []string

	// UnblockerURL, when set, is a web-unblocker API URL template (containing the
	// placeholder "{{url}}") that real source adapters route requests through to
	// get past bot protection (e.g. Cloudflare on bauhaus.info). Empty = adapters
	// fetch directly and degrade gracefully when blocked.
	UnblockerURL string

	PollInterval     time.Duration
	PollerHealthPort int
}

// Load reads configuration from the environment, applying defaults that match
// .env.example. It returns an error only when a required value is missing or a
// typed value cannot be parsed.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		PublicBaseURL: getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		SMTPHost:      getEnv("SMTP_HOST", "mailhog"),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:      getEnv("SMTP_FROM", "PortaSplitFinder <noreply@portasplitfinder.local>"),
		Sources:       parseSources(getEnv("SOURCES", "mock")),
		UnblockerURL:  getEnv("UNBLOCKER_URL", ""),
	}

	var err error
	if c.APIPort, err = getEnvInt("API_PORT", 8080); err != nil {
		return Config{}, err
	}
	if c.SMTPPort, err = getEnvInt("SMTP_PORT", 1025); err != nil {
		return Config{}, err
	}
	if c.PollerHealthPort, err = getEnvInt("POLLER_HEALTH_PORT", 8090); err != nil {
		return Config{}, err
	}
	c.SMTPTLS = getEnvBool("SMTP_TLS", false)

	if c.PollInterval, err = getEnvDuration("POLL_INTERVAL", 5*time.Minute); err != nil {
		return Config{}, err
	}

	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}

	return c, nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer: %w", key, err)
	}
	return n, nil
}

func getEnvBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func getEnvDuration(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a Go duration: %w", key, err)
	}
	return d, nil
}

func parseSources(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
