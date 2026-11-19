// Command relay terminates the tunnels opened by connectors and forwards
// the gateway's rebuilt requests into them (schema/tunnel/tunnel.md).
//
// Usage:
//
//	relay              start the service
//	relay healthcheck  check /healthz locally (for the Docker healthcheck)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/admin"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/config"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/forward"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/logging"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/peers"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/registry"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/tunnel"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

// How often the connectors file is checked for changes.
const reloadEvery = 15 * time.Second

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "relay:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.Instance, cfg.LogLevel)
	events := logging.NewEvents(os.Stdout, cfg.Instance)

	reg := registry.New(cfg.ConnectorsFile)
	if cfg.ConnectorsURL != "" {
		reg = registry.NewFromPanel(cfg.ConnectorsURL, cfg.PanelToken)
	}
	log.Info("connector source", slog.Group("detail", slog.String("source", reg.Source())))
	pool := sessions.NewPool()
	var ready atomic.Bool
	reload := func() {
		changed, err := reg.Reload()
		if err != nil {
			log.Error("connectors not loaded", slog.String("error", err.Error()))
			return
		}
		ready.Store(true)
		if !changed {
			return
		}
		closed := pool.CloseExcept(func(s *sessions.Session) bool {
			c, ok := reg.Get(s.ConnectorID)
			return ok && c.TokenSHA256 == s.TokenHash
		})
		log.Info("connectors loaded", slog.Group("detail", slog.Int("tunnels_closed", closed)))
	}
	reload()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		tick := time.NewTicker(reloadEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				reload()
			case <-tick.C:
				reload()
			}
		}
	}()

	fwd := forward.New(pool, cfg.GatewayToken, log.With(slog.String("module", "forward")))
	if cfg.Peers != "" {
		_, port, _ := net.SplitHostPort(cfg.InternalAddr)
		fwd.Peers = peers.New(cfg.Peers, port).Peers
		log.Info("replicas found through DNS", slog.Group("detail", slog.String("name", cfg.Peers)))
	}
	tunnels := &tunnel.Handler{Registry: reg, Pool: pool, Log: log.With(slog.String("module", "tunnel")), Events: events}
	servers := []*http.Server{
		// Tunnels are hijacked: the server's limits cover only the upgrade.
		{Addr: cfg.TunnelAddr, Handler: tunnels, ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10,
			IdleTimeout: 60 * time.Second},
		// Gateway requests stream uploads and live updates: no read or write
		// timeout, the gateway sets the pace.
		{Addr: cfg.InternalAddr, Handler: fwd, ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 64 << 10,
			IdleTimeout: 120 * time.Second},
		{Addr: cfg.AdminAddr, Handler: &admin.Handler{Pool: pool, Ready: &ready, Version: version}, ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10},
	}
	errs := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- err
			}
		}(s)
	}
	log.Info("relay started", slog.Group("detail", slog.String("version", version),
		slog.String("tunnel_addr", cfg.TunnelAddr), slog.String("internal_addr", cfg.InternalAddr), slog.String("admin_addr", cfg.AdminAddr)))

	select {
	case <-ctx.Done():
	case err := <-errs:
		log.Error("server stopped", slog.String("error", err.Error()))
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(shutdownCtx)
	}
	pool.CloseExcept(func(*sessions.Session) bool { return false })
	log.Info("relay stopped")
	return nil
}

func healthcheck() int {
	addr := os.Getenv("EQ_ADMIN_ADDR")
	if addr == "" {
		addr = ":9091"
	}
	port := addr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port = addr[i:]
			break
		}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
