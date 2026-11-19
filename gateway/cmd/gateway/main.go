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
	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/oidcsecrets"
	"github.com/thevidocq/equalizer/gateway/internal/server"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
	"github.com/thevidocq/equalizer/gateway/internal/unmapped"
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
	formKey := cfg.FormKey
	if formKey == nil {
		formKey = formtoken.RandomKey()
		log.Warn("EQ_FORM_KEY not set: random form key, forms work only with a single replica")
	}
	sessions, counters, closeStore, err := stores(context.Background(), cfg, formKey, log)
	if err != nil {
		return err
	}
	defer closeStore()
	var source mapstore.Source = &mapstore.FileSource{Path: cfg.MapFile}
	if cfg.MapURL != "" {
		source = &mapstore.HTTPSource{URL: cfg.MapURL, Token: cfg.MapToken, Client: &http.Client{Timeout: 10 * time.Second}}
	}
	log.Info("map source", slog.Group("detail", slog.String("source", source.String())))
	store := mapstore.NewStoreFrom(source, pub, cfg.MapPublicKeyID, site.Options{Relay: relay, Limits: counters})
	rep, err := store.Reload()
	logReload(log, rep, err)

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
	go store.Watch(ctx, 15*time.Second, trigger, func(rep mapstore.Report, err error) { logReload(log, rep, err) })

	sealer, err := formtoken.New(formKey)
	if err != nil {
		return err
	}

	var reporter *unmapped.Reporter
	if cfg.UnmappedURL != "" {
		reporter = &unmapped.Reporter{URL: cfg.UnmappedURL, Token: cfg.MapToken, Client: &http.Client{Timeout: 10 * time.Second},
			Log: log.With(slog.String("module", "unmapped"))}
		go reporter.Run(ctx, time.Minute)
	}
	var secrets *oidcsecrets.Source
	if cfg.OIDCSecretsURL != "" {
		secrets = &oidcsecrets.Source{URL: cfg.OIDCSecretsURL, Token: cfg.MapToken, Client: &http.Client{Timeout: 10 * time.Second},
			Log: log.With(slog.String("module", "oidcsecrets"))}
		go secrets.Run(ctx, 15*time.Second)
	}
	ips := clientip.New(cfg.TrustedProxies)
	ips.Cloudflare = cfg.BehindCloudflare
	public := &server.Public{
		OIDCSecrets: secrets,
		Unmapped:    reporter,
		Forms:       sealer,
		Sessions:    sessions,
		Maps:        store,
		IPs:         ips,
		Log:         telemetry.Logger(os.Stdout, cfg.Instance, level).With(slog.String("module", "server.public")),
		Events:      events,
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

// stores builds the session and limits stores: Redis when configured
// (shared by the replicas), otherwise in process. Sessions use a key derived
// from the gateway secret (EQ_FORM_KEY).
func stores(ctx context.Context, cfg config.Config, key []byte, log *slog.Logger) (*session.Manager, limits.Store, func(), error) {
	var store session.Store
	var counters limits.Store
	closeStore := func() {}
	if cfg.RedisURL == "" {
		log.Warn("EQ_REDIS_URL not set: sessions and rate limits in memory, single replica only")
		store, counters = session.NewMemory(), limits.NewMemory()
	} else {
		r, err := session.NewRedis(cfg.RedisURL)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("EQ_REDIS_URL: %w", err)
		}
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := r.Ping(pingCtx); err != nil {
			// Not fatal: anonymous pages keep working and Redis may come up later.
			log.Error("session store unreachable", slog.String("error", err.Error()))
		}
		store, counters, closeStore = r, limits.NewRedis(r.Client()), func() { _ = r.Close() }
	}
	m, err := session.NewManager(store, key)
	return m, counters, closeStore, err
}

func logReload(log *slog.Logger, rep mapstore.Report, err error) {
	for _, f := range rep.Failed {
		log.Error("map not loaded", slog.String("error", f.Error()))
	}
	if err != nil && len(rep.Failed) == 0 {
		log.Error("maps not loaded", slog.String("error", err.Error()))
	}
	for _, a := range rep.Active {
		log.Info("map active", slog.Group("detail",
			slog.String("tenant", a.Model.Meta.Tenant),
			slog.String("app", a.Model.Meta.App),
			slog.String("domain", a.Model.Meta.PublicDomain),
			slog.Int("map_version", a.Model.Meta.Version),
		))
		for _, n := range a.Notes {
			log.Warn("map feature not applied yet", slog.Group("detail", slog.String("app", a.Model.Meta.App), slog.String("note", n)))
		}
	}
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
