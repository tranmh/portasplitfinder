// Command poller periodically scrapes the enabled sources, writes availability
// snapshots, and sends notification emails. It exposes GET /healthz for the
// Docker healthcheck.
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
	"github.com/tranmh/portasplitfinder/internal/jobs"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
	"github.com/tranmh/portasplitfinder/internal/store"
)

func main() {
	once := flag.Bool("once", false, "run a single poll cycle then exit (tests/CI)")
	healthFlag := flag.Bool("healthcheck", false, "GET own /healthz and exit 0/1")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if *healthFlag {
		os.Exit(runHealthcheck())
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signalContext()
	defer cancel()

	// Ensure schema exists (poller may start before/independently of api).
	if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Configure the web-unblocker transport (if any) before building real sources.
	source.SetUnblocker(cfg.UnblockerURL)

	srcs, err := source.Build(cfg.Sources)
	if err != nil {
		log.Error("build sources", "err", err)
		os.Exit(1)
	}

	mailer := notify.NewSMTPMailer(notify.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
		TLS:      cfg.SMTPTLS,
	})

	poller := jobs.New(jobs.Config{
		Sources:       srcs,
		Store:         db,
		Mailer:        mailer,
		PublicBaseURL: cfg.PublicBaseURL,
		Logger:        log,
	})

	if *once {
		sent, err := poller.PollOnce(ctx)
		if err != nil {
			log.Error("poll once", "err", err)
			os.Exit(1)
		}
		log.Info("poll once complete", "mails_sent", sent)
		return
	}

	lastPoll := &jobs.LastPoll{}

	// Health server.
	healthAddr := fmt.Sprintf(":%d", cfg.PollerHealthPort)
	healthSrv := &http.Server{
		Addr:              healthAddr,
		Handler:           healthHandler(lastPoll),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("poller health listening", "addr", healthAddr)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("health listen", "err", err)
		}
	}()

	log.Info("poller starting", "interval", cfg.PollInterval.String(), "sources", cfg.Sources)
	runErr := poller.Run(ctx, cfg.PollInterval, lastPoll)

	shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = healthSrv.Shutdown(shutCtx)

	if runErr != nil && runErr != context.Canceled {
		log.Error("poller stopped", "err", runErr)
	}
	log.Info("poller stopped")
}

// healthHandler returns 200 with the last-poll age while the poller is alive. It
// reports 200 even before the first poll completes (process is up); the body
// indicates whether a poll has happened yet.
func healthHandler(lp *jobs.LastPoll) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		age, ok := lp.Age(time.Now().UTC())
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if ok {
			fmt.Fprintf(w, `{"status":"ok","lastPollAgeSeconds":%d}`, int(age.Seconds()))
		} else {
			fmt.Fprint(w, `{"status":"starting","lastPollAgeSeconds":null}`)
		}
	})
	return mux
}

func runHealthcheck() int {
	port := getenvDefault("POLLER_HEALTH_PORT", "8090")
	url := fmt.Sprintf("http://127.0.0.1:%s/healthz", port)
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
	return 1
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
