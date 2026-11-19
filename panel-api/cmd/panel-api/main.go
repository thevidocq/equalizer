// Command panel-api is the control-plane API of Equalizer.
//
// Usage:
//
//	panel-api                                  start the service
//	panel-api create-admin -email E -name N    create a platform administrator
//	                                           (password read from standard input)
//	panel-api healthcheck                      check /healthz locally (Docker healthcheck)
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/alerts"
	"github.com/thevidocq/equalizer/panel-api/internal/api"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/config"
	"github.com/thevidocq/equalizer/panel-api/internal/db"
	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/logging"
	"github.com/thevidocq/equalizer/panel-api/internal/logsearch"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
	"github.com/thevidocq/equalizer/panel-api/internal/mfa"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
	"github.com/thevidocq/equalizer/panel-api/internal/ui"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	var err error
	switch {
	case len(os.Args) > 1 && os.Args[1] == "healthcheck":
		os.Exit(healthcheck())
	case len(os.Args) > 1 && os.Args[1] == "create-admin":
		err = createAdmin(os.Args[2:])
	default:
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "panel-api:", err)
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

	conn, err := db.Open(ctx, cfg.DatabaseURL, 60*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	applied, err := db.Migrate(ctx, conn)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if len(applied) > 0 {
		log.Info("migrations applied", slog.Group("detail", slog.Any("names", applied)))
	}

	st := store.New(conn)
	if n, err := st.CountPlatformAdmins(ctx); err == nil && n == 0 {
		log.Warn("no platform administrator yet: run `panel-api create-admin -email ... -name ...`")
	}
	sessions := &auth.Sessions{DB: conn, TTL: cfg.SessionTTL}
	go purge(ctx, sessions, log)

	box, err := mfa.NewBox(cfg.MFAKey)
	if err != nil {
		return err
	}
	if !cfg.MFARequired {
		log.Warn("EQ_MFA_REQUIRED=false: panel users may log in with a password only")
	}
	var sender mail.Sender = mail.Disabled{}
	switch cfg.MailMode {
	case "smtp":
		sender = mail.SMTP{Addr: cfg.SMTPAddr, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.MailFrom}
	case "log":
		sender = mail.Log{Logger: log}
		log.Warn("EQ_MAIL_MODE=log: invitation links are written to the logs, use it only for development")
	default:
		log.Warn("email not configured (EQ_SMTP_ADDR): invitations are created but not sent")
	}
	direct, err := api.ParseDestinations(cfg.DirectDestinations)
	if err != nil {
		return fmt.Errorf("EQ_DIRECT_DESTINATIONS: %w", err)
	}
	var checker *exposure.Runner
	if cfg.ExposureInterval > 0 {
		checker = &exposure.Runner{Store: st, Prober: &exposure.Prober{Timeout: 10 * time.Second},
			Interval: cfg.ExposureInterval, Log: log.With(slog.String("module", "exposure"))}
		go checker.Run(ctx)
	} else {
		log.Warn("EQ_EXPOSURE_INTERVAL=0: protected sites are not checked for direct exposure")
	}
	srv := &api.Server{
		DirectDestinations: direct, Exposure: checker, ExposureThrottle: auth.NewThrottle(1, time.Minute),
		Mail: sender, PanelURL: cfg.PanelURL,
		MFA: box, MFARequired: cfg.MFARequired, MFAThrottle: auth.NewThrottle(5, 15*time.Minute),
		Store: st, Sessions: sessions, Throttle: auth.NewThrottle(5, 15*time.Minute),
		Log: log.With(slog.String("module", "api")), SigningKey: cfg.SigningKey, KeyID: cfg.SigningKeyID,
		InternalToken: cfg.InternalToken,
	}
	if cfg.UIDir != "" {
		srv.UI = ui.Handler(cfg.UIDir)
	}
	if cfg.GatewayAdmin != "" {
		srv.MapCheck = &mapcheck.Checker{Addr: cfg.GatewayAdmin}
	} else {
		log.Warn("EQ_GATEWAY_ADMIN not set: maps are checked against the schema only")
	}
	srv.Health = &health.Probe{RelayAdmin: cfg.RelayAdmin, GatewayAdmin: cfg.GatewayAdmin, LokiURL: cfg.LokiURL}
	if cfg.LokiURL != "" {
		srv.Logs = &logsearch.Loki{URL: cfg.LokiURL}
	} else {
		log.Warn("EQ_LOKI_URL not set: the Logs page is not available")
	}
	go scheduleDiscoveries(ctx, srv, log.With(slog.String("module", "discovery")))
	srv.MailAvailable = cfg.MailMode != "off"
	if srv.MailAvailable {
		go (&alerts.Runner{Store: st, Statuses: srv.SiteStatuses, Mail: sender, PanelURL: cfg.PanelURL,
			Log: log.With(slog.String("module", "alerts"))}).Run(ctx)
	}
	ready := func(w http.ResponseWriter, r *http.Request) {
		if err := conn.PingContext(r.Context()); err != nil {
			http.Error(w, `{"status":"not_ready","reason":"database"}`, http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	})
	adminMux.HandleFunc("/readyz", ready)

	servers := []*http.Server{
		{Addr: cfg.Addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 32 << 10},
		{Addr: cfg.AdminAddr, Handler: adminMux, ReadHeaderTimeout: 5 * time.Second},
	}
	errs := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- err
			}
		}(s)
	}
	log.Info("panel-api started", slog.Group("detail", slog.String("version", version), slog.String("addr", cfg.Addr)))
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
	log.Info("panel-api stopped")
	return nil
}

// scheduleDiscoveries queues the continuous discoveries that are due.
func scheduleDiscoveries(ctx context.Context, srv *api.Server, log *slog.Logger) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		if n, err := srv.ScheduleDiscoveries(ctx); err != nil {
			log.Warn("continuous discovery not scheduled", slog.String("error", err.Error()))
		} else if n > 0 {
			log.Info("continuous discoveries queued", slog.Group("detail", slog.Int("count", n)))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func purge(ctx context.Context, s *auth.Sessions, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := s.Purge(ctx); err != nil {
				log.Warn("expired sessions not purged", slog.String("error", err.Error()))
			}
		}
	}
}

func createAdmin(args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	email := fs.String("email", "", "administrator email")
	name := fs.String("name", "", "administrator name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *name == "" {
		return errors.New("-email and -name are required")
	}
	dsn, err := config.DatabaseURL()
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "Password (at least 12 characters): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return errors.New("no password on standard input")
	}
	hash, err := auth.HashPassword(strings.TrimRight(line, "\r\n"))
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, dsn, 30*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	role := "admin"
	u, err := store.New(conn).CreateUser(ctx, store.NewUser{Email: *email, Name: *name, PasswordHash: hash, PlatformRole: &role})
	if errors.Is(err, store.ErrConflict) {
		return fmt.Errorf("a user with email %s already exists", *email)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nPlatform administrator %s created (id %d).\n", u.Email, u.ID)
	return nil
}

func healthcheck() int {
	addr := os.Getenv("EQ_ADMIN_ADDR")
	if addr == "" {
		addr = ":9092"
	}
	port := addr[strings.LastIndexByte(addr, ':'):]
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
