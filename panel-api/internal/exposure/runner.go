package exposure

import (
	"context"
	"log/slog"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Store is what the runner needs from the database.
type Store interface {
	ExposureTargets(ctx context.Context) ([]store.ExposureTarget, error)
	SaveExposure(ctx context.Context, app, ip, status, reason string) (string, error)
	Record(ctx context.Context, userID *int64, tenant *string, action, target string, detail map[string]any) error
}

// Runner checks every site with an old public IP once per Interval.
type Runner struct {
	Store    Store
	Prober   *Prober
	Interval time.Duration
	Log      *slog.Logger
}

// Run checks due sites every minute until ctx ends.
func (r *Runner) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		r.Once(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Once checks the sites whose last check is older than Interval.
func (r *Runner) Once(ctx context.Context) {
	targets, err := r.Store.ExposureTargets(ctx)
	if err != nil {
		r.Log.Error("exposure check: listing sites", slog.String("error", err.Error()))
		return
	}
	for _, t := range targets {
		if ctx.Err() != nil {
			return
		}
		if t.CheckedAt != nil && time.Since(*t.CheckedAt) < r.Interval {
			continue
		}
		r.CheckOne(ctx, t)
	}
}

// CheckOne probes a site, stores the result and reports a change of status.
func (r *Runner) CheckOne(ctx context.Context, t store.ExposureTarget) Result {
	res := r.Prober.Check(ctx, t.IP, t.Domain)
	before, err := r.Store.SaveExposure(ctx, t.App, t.IP, res.Status, res.Reason)
	if err != nil {
		// The IP changed or the site went away during the check.
		return res
	}
	if before == res.Status {
		return res
	}
	attrs := slog.Group("detail", slog.String("tenant", t.Tenant), slog.String("app", t.App),
		slog.String("status", res.Status), slog.String("reason", res.Reason), slog.String("before", before))
	switch res.Status {
	case Exposed:
		r.Log.Warn("site reachable directly, bypassing Equalizer", attrs)
	case Error:
		r.Log.Warn("exposure check could not run", attrs)
	default:
		r.Log.Info("site not reachable directly", attrs)
	}
	tenant := t.Tenant
	_ = r.Store.Record(ctx, nil, &tenant, "site.exposure", t.App, map[string]any{"status": res.Status, "reason": res.Reason})
	return res
}
