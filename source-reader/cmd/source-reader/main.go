// Command source-reader reads a site's code where the site lives and sends
// the panel a summary of it: entry points, input types, risky calls,
// components. It only calls out, reads only, and never sends code, values
// or secrets.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/source-reader/internal/config"
	"github.com/thevidocq/equalizer/source-reader/internal/extract"
	"github.com/thevidocq/equalizer/source-reader/internal/fetch"
	"github.com/thevidocq/equalizer/source-reader/internal/keys"
	"github.com/thevidocq/equalizer/source-reader/internal/logging"
	"github.com/thevidocq/equalizer/source-reader/internal/panel"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	log := logging.New(os.Stdout, cfg.Instance, cfg.LogLevel)
	if err != nil {
		log.Error("configuration", slog.String("error", err.Error()))
		os.Exit(2)
	}
	// Without its key the reader still answers, and reads by smb or a
	// mounted folder; an sftp read fails and says why.
	priv, pub, err := keys.Ensure(cfg.State)
	if err != nil {
		log.Error("no ssh key: sftp reads will fail", slog.String("error", err.Error()))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	r := &reader{p: &panel.Client{Base: cfg.PanelURL + "/v1", Token: cfg.Token}, log: log, pub: pub,
		env: fetch.Env{Key: priv, State: cfg.State, Work: cfg.Work, Site: cfg.Site}}
	log.Info("source reader started", slog.String("version", version))
	r.run(ctx)
}

type reader struct {
	p   *panel.Client
	log *slog.Logger
	pub string
	env fetch.Env
}

// run says hello every minute and asks for a read every 20 seconds.
func (r *reader) run(ctx context.Context) {
	poll := time.NewTicker(20 * time.Second)
	defer poll.Stop()
	var hello time.Time
	for {
		if time.Since(hello) > time.Minute {
			if err := r.p.Hello(ctx, version, r.pub); err != nil {
				r.log.Warn("the panel did not answer", slog.String("error", err.Error()))
			} else {
				hello = time.Now()
			}
		}
		r.once(ctx)
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
		}
	}
}

func (r *reader) once(ctx context.Context) {
	job, err := r.p.Claim(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			r.log.Warn("no read taken", slog.String("error", err.Error()))
		}
		return
	}
	if job == nil {
		return
	}
	start := time.Now()
	summary, failure := r.read(ctx, job)
	if err := r.p.Result(ctx, job.ID, summary, failure); err != nil {
		r.log.Error("result not sent", slog.Int64("read", job.ID), slog.String("error", err.Error()))
		return
	}
	if failure != "" {
		r.log.Warn("read failed", slog.Int64("read", job.ID), slog.String("method", job.Method), slog.String("error", failure))
		return
	}
	r.log.Info("read done", slog.Int64("read", job.ID), slog.String("method", job.Method), slog.Int("files", summary.Files.Read),
		slog.Int("routes", len(summary.Routes)), slog.Int("inputs", len(summary.Inputs)), slog.Duration("took", time.Since(start)))
}

// read fetches the files and reads them; the work copy is deleted after.
func (r *reader) read(ctx context.Context, j *panel.Job) (*extract.Summary, string) {
	spec := fetch.Spec{Method: j.Method, Path: j.Path, Host: deref(j.Host), Username: deref(j.Username),
		Share: deref(j.Share), Password: deref(j.Password)}
	if j.Port != nil {
		spec.Port = *j.Port
	}
	dir, clean, err := fetch.Fetch(ctx, spec, r.env)
	defer clean()
	if err != nil {
		return nil, err.Error()
	}
	s, err := extract.Read(dir, time.Now())
	if err != nil {
		return nil, "reading the files: " + err.Error()
	}
	if s.Files.Read == 0 {
		return nil, "no code file found in " + j.Path + ": check the folder and the user's rights"
	}
	return s, ""
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
