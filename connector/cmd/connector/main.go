// Command connector runs next to the legacy application: it opens the
// tunnel towards the relay (outbound only) and executes the requests that
// arrive on it, towards the destinations of its local allowlist.
//
// Usage:
//
//	connector              start the service
//	connector healthcheck  exit 0 while the tunnel is open (Docker healthcheck)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/connector/internal/config"
	"github.com/thevidocq/equalizer/connector/internal/executor"
	"github.com/thevidocq/equalizer/connector/internal/logging"
	"github.com/thevidocq/equalizer/connector/internal/tunnel"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "connector:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.Instance, cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &tunnel.Client{
		RelayURL: cfg.RelayURL,
		ID:       cfg.ID,
		Token:    cfg.Token,
		Version:  version,
		Allow:    cfg.Allow.Items(),
		HTTP: &http.Client{Transport: &http.Transport{
			// The connector may sit behind a corporate proxy (HTTPS_PROXY).
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSClientConfig:       cfg.RelayTLS,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		}},
		Handler:   executor.New(cfg.Allow, cfg.LegacyTLS, log.With(slog.String("module", "executor"))),
		Log:       log.With(slog.String("module", "tunnel")),
		Connected: func(up bool) { markStatus(cfg.StatusFile, up) },
	}
	log.Info("connector started", slog.Group("detail", slog.String("version", version),
		slog.String("connector_id", cfg.ID), slog.String("relay", cfg.RelayURL.Host), slog.Any("allow", cfg.Allow.Items())))
	markStatus(cfg.StatusFile, false)
	client.Run(ctx)
	markStatus(cfg.StatusFile, false)
	log.Info("connector stopped")
	return nil
}

// markStatus creates the status file while the tunnel is open.
func markStatus(path string, up bool) {
	if path == "" {
		return
	}
	if up {
		_ = os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
		return
	}
	_ = os.Remove(path)
}

func healthcheck() int {
	path := os.Getenv("EQ_STATUS_FILE")
	if path == "" {
		path = "/tmp/eq-tunnel-up"
	}
	if _, err := os.Stat(path); err != nil {
		return 1
	}
	return 0
}
