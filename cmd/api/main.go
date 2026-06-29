// Command api serves the PortaSplitFinder HTTP API. It runs DB migrations on
// boot then serves on API_PORT. Flags support seeding and healthchecks.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tranmh/portasplitfinder/internal/config"
	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/httpapi"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
	"github.com/tranmh/portasplitfinder/internal/store"
)

func main() {
	seedFlag := flag.Bool("seed", false, "seed products/sources/stores from the mock source, then exit")
	healthFlag := flag.Bool("healthcheck", false, "HTTP GET own /api/health and exit 0 (ok) or 1 (not ok)")
	migrateOnly := flag.Bool("migrate-only", false, "run DB migrations then exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Healthcheck does not need config beyond the port; it pings our own HTTP.
	if *healthFlag {
		os.Exit(runHealthcheck())
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Tolerate a slow/late database on boot (Postgres still starting, Docker
	// embedded DNS not yet resolving `db`) so a transient hiccup doesn't kill us.
	if err := store.WaitReady(ctx, cfg.DatabaseURL, 60*time.Second); err != nil {
		log.Error("wait for db", "err", err)
		os.Exit(1)
	}

	if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	if *migrateOnly {
		log.Info("migrations applied")
		return
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if *seedFlag {
		if err := seed(ctx, db); err != nil {
			log.Error("seed", "err", err)
			os.Exit(1)
		}
		log.Info("seed complete")
		return
	}

	// Auto-seed on boot when the catalogue is empty so the stack has data
	// unattended (the e2e / mock flow depends on it). Idempotent: skipped once
	// products exist. Disable with SEED_ON_BOOT=false.
	if getenvDefault("SEED_ON_BOOT", "true") != "false" {
		if products, perr := db.Products.List(ctx); perr == nil && len(products) == 0 {
			if err := seed(ctx, db); err != nil {
				log.Error("auto-seed", "err", err)
			} else {
				log.Info("auto-seed complete (empty catalogue)")
			}
		}
	}

	mailer := notify.NewSMTPMailer(notify.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
		TLS:      cfg.SMTPTLS,
	})

	srv := httpapi.New(httpapi.Config{
		Store:         db,
		Mailer:        mailer,
		PublicBaseURL: cfg.PublicBaseURL,
		WebOrigin:     os.Getenv("WEB_ORIGIN"),
		Logger:        log,
	})

	addr := fmt.Sprintf(":%d", cfg.APIPort)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutCtx)
	log.Info("api stopped")
}

// seed populates the DB from the mock source's deterministic catalogue.
func seed(ctx context.Context, db *store.DB) error {
	m := source.NewMock()
	var stores []domain.Store
	stores = append(stores, m.Stores()...)
	return db.Seed(ctx, m.Products(), m.SeedSources(), stores)
}

// runHealthcheck GETs the local /api/health and maps the HTTP status to an exit
// code (0 if 200, else 1). Used by the Docker healthcheck.
func runHealthcheck() int {
	port := getenvDefault("API_PORT", "8080")
	url := fmt.Sprintf("http://127.0.0.1:%s/api/health", port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	fmt.Fprintln(os.Stderr, "healthcheck status:", resp.StatusCode)
	return 1
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
