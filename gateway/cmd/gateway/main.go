// Command gateway starts public traffic and the administration endpoints.
//
// Usage:
//
//	gateway              start the service
//	gateway healthcheck  check /healthz locally (for the Docker healthcheck)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/config"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/server"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gateway:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	level := new(slog.LevelVar)
	level.Set(telemetry.ParseLevel(cfg.LogLevel))
	log := telemetry.Logger(os.Stdout, cfg.Instance, level).With(slog.String("module", "main"))
	events := telemetry.NewEmitter(os.Stdout, cfg.Instance)

	pub, err := mapstore.ParsePublicKey(cfg.MapPublicKey)
	if err != nil {
		return err
	}
	relay, err := transport.ParseRelay(cfg.RelayURL, cfg.RelayToken)
	if err != nil {
		return fmt.Errorf("EQ_RELAY_URL/EQ_RELAY_TOKEN: %w", err)
	}
	if relay == nil {
		log.Warn("EQ_RELAY_URL not set: maps in connector mode answer 502")
	}
	store := mapstore.NewStore(cfg.MapFile, pub, cfg.MapPublicKeyID, site.Options{Relay: relay})
	active, err := store.Reload()
	logReload(log, active, err)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	trigger := make(chan struct{}, 1)
	go func() {
		for range hup {
			select {
			case trigger <- struct{}{}:
			default:
			}
		}
	}()
	go store.Watch(ctx, 15*time.Second, trigger, func(st *site.Site, err error) { logReload(log, st, err) })

	formKey := cfg.FormKey
	if formKey == nil {
		formKey = formtoken.RandomKey()
		log.Warn("EQ_FORM_KEY not set: random form key, forms work only with a single replica")
	}
	sealer, err := formtoken.New(formKey)
	if err != nil {
		return err
	}

	sessions, closeStore, err := sessionManager(ctx, cfg, formKey, log)
	if err != nil {
		return err
	}
	defer closeStore()

	public := &server.Public{
		Forms:    sealer,
		Sessions: sessions,
		Maps:     store,
		IPs:      clientip.New(cfg.TrustedProxies),
		Log:      telemetry.Logger(os.Stdout, cfg.Instance, level).With(slog.String("module", "server.public")),
		Events:   events,
	}
	admin := &server.Admin{Maps: store, Version: version}

	servers := []*http.Server{
		{Addr: cfg.PublicAddr, Handler: public.Handler(), ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 32 << 10},
		{Addr: cfg.AdminAddr, Handler: admin.Handler(), ReadHeaderTimeout: 5 * time.Second},
	}
	errs := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- err
			}
		}(s)
	}
	log.Info("gateway started", slog.Group("detail",
		slog.String("version", version),
		slog.String("public_addr", cfg.PublicAddr),
		slog.String("admin_addr", cfg.AdminAddr),
	))

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
	log.Info("gateway stopped")
	return nil
}

// sessionManager builds the session store: Redis when configured (shared by
// the replicas), otherwise in process. Sessions use a key derived from the
// gateway secret (EQ_FORM_KEY).
func sessionManager(ctx context.Context, cfg config.Config, key []byte, log *slog.Logger) (*session.Manager, func(), error) {
	var store session.Store
	closeStore := func() {}
	if cfg.RedisURL == "" {
		log.Warn("EQ_REDIS_URL not set: sessions in memory, single replica only")
		store = session.NewMemory()
	} else {
		r, err := session.NewRedis(cfg.RedisURL)
		if err != nil {
			return nil, nil, fmt.Errorf("EQ_REDIS_URL: %w", err)
		}
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := r.Ping(pingCtx); err != nil {
			// Not fatal: anonymous pages keep working and Redis may come up later.
			log.Error("session store unreachable", slog.String("error", err.Error()))
		}
		store, closeStore = r, func() { _ = r.Close() }
	}
	m, err := session.NewManager(store, key)
	return m, closeStore, err
}

func logReload(log *slog.Logger, a *site.Site, err error) {
	if err != nil {
		log.Error("map not loaded", slog.String("error", err.Error()))
		return
	}
	log.Info("map active", slog.Group("detail",
		slog.String("tenant", a.Model.Meta.Tenant),
		slog.String("app", a.Model.Meta.App),
		slog.Int("map_version", a.Model.Meta.Version),
	))
}

func healthcheck() int {
	addr := os.Getenv("EQ_ADMIN_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1" + portOf(addr) + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func portOf(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i:]
		}
	}
	return ":" + addr
}
