package securityscan

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/cveradar"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Store is what the runner needs from the database.
type Store interface {
	ClaimSecurityScans(ctx context.Context, limit int) ([]store.SecurityScan, error)
	SetSecurityScanStatus(ctx context.Context, id int64, status string) error
	FailSecurityScan(ctx context.Context, id int64, reason string) error
	CompleteSecurityScan(ctx context.Context, id int64, risk *int, findings []store.SecurityFinding, note *string) (bool, error)
	Record(ctx context.Context, userID *int64, tenant *string, action, target string, detail map[string]any) error
	PendingSecurityDrafts(ctx context.Context, limit int) ([]int64, error)
	ScannerAnswered(ctx context.Context, tenant string, refused bool) error
}

// Clients opens a tenant's scanner; store.ErrNotFound when it has none.
type Clients func(ctx context.Context, tenant string) (*cveradar.Client, error)

// Runner follows the scans going on, once a minute: their status, and
// their findings when they end.
type Runner struct {
	Store   Store
	Clients Clients
	Log     *slog.Logger
	GiveUp  time.Duration // a scan without an end after this fails; 0: 2 hours
	// AfterComplete runs once a scan's findings are saved (the map draft
	// that closes them); nil: nothing.
	AfterComplete func(ctx context.Context, scan int64)
}

const (
	perRound    = 20              // scans polled per minute: 20 requests of the scanner's 60
	parallel    = 4               // scans polled at once, so one slow scanner does not hold the others
	pollTimeout = 2 * time.Minute // for one scan, findings included
)

// Run polls every minute until ctx ends.
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

// Once polls the scans that are due, then builds the drafts still to build
// (the map was busy, or a replica stopped while building one).
func (r *Runner) Once(ctx context.Context) {
	defer r.retryDrafts(ctx)
	scans, err := r.Store.ClaimSecurityScans(ctx, perRound)
	if err != nil {
		r.Log.Error("security scans: listing", slog.String("error", err.Error()))
		return
	}
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for _, sc := range scans {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(sc store.SecurityScan) {
			defer func() { <-sem; wg.Done() }()
			pctx, cancel := context.WithTimeout(ctx, pollTimeout)
			defer cancel()
			r.Poll(pctx, sc)
		}(sc)
	}
	wg.Wait()
}

// Follow polls one scan now, with the same deadline as a round (a quick
// scan the scanner already ended).
func (r *Runner) Follow(ctx context.Context, sc store.SecurityScan) {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()
	r.Poll(ctx, sc)
}

// Poll reads one scan's status and records what changed.
func (r *Runner) Poll(ctx context.Context, sc store.SecurityScan) {
	giveUp := r.GiveUp
	if giveUp == 0 {
		giveUp = 2 * time.Hour
	}
	c, err := r.Clients(ctx, sc.Tenant)
	if errors.Is(err, store.ErrNotFound) {
		r.fail(ctx, sc, "the scanner was disconnected (Settings → Security scanner)")
		return
	}
	if err != nil {
		r.Log.Error("security scans: opening the scanner", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
		return
	}
	remote, err := c.GetScan(ctx, sc.RemoteScan)
	var se *cveradar.Error
	if refused := errors.As(err, &se) && (se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden); refused || err == nil {
		_ = r.Store.ScannerAnswered(ctx, sc.Tenant, refused)
	}
	switch {
	case errors.As(err, &se) && (se.Status == http.StatusNotFound || se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden):
		r.fail(ctx, sc, "the scanner no longer knows this scan or this key: "+se.Error())
		return
	case err != nil:
		if time.Since(sc.CreatedAt) > giveUp {
			r.fail(ctx, sc, "the scanner did not answer: "+err.Error())
			return
		}
		r.Log.Warn("security scans: polling", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
		return
	}
	switch remote.Status {
	case "completed":
		if err := r.complete(ctx, c, sc); err != nil {
			if time.Since(sc.CreatedAt) > giveUp {
				r.fail(ctx, sc, "the findings could not be read: "+clip(err.Error(), 300))
				return
			}
			r.Log.Warn("security scans: reading findings", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
		}
	case "failed":
		reason := "the scanner could not finish the scan"
		if remote.Error != nil && strings.TrimSpace(*remote.Error) != "" {
			reason += ": " + clip(*remote.Error, 300)
		}
		r.fail(ctx, sc, reason)
	case "pending", "running":
		if time.Since(sc.CreatedAt) > giveUp {
			r.fail(ctx, sc, "the scan did not end within "+giveUp.String())
			return
		}
		if remote.Status != sc.Status {
			_ = r.Store.SetSecurityScanStatus(ctx, sc.ID, remote.Status)
		}
	}
}

// complete copies the findings of an ended scan; an error is retried the
// next minute, until the scan is given up.
func (r *Runner) complete(ctx context.Context, c *cveradar.Client, sc store.SecurityScan) error {
	vulns, err := c.OpenVulnerabilities(ctx, sc.RemoteTarget)
	var note *string
	if errors.Is(err, cveradar.ErrTooMany) {
		n := "only the first findings were read: " + err.Error()
		note = &n
	} else if err != nil {
		return err
	}
	var risk *int
	if t, err := c.GetTarget(ctx, sc.RemoteTarget); err == nil {
		risk = t.RiskScore
	}
	findings := make([]store.SecurityFinding, 0, len(vulns))
	for _, v := range vulns {
		findings = append(findings, Finding(v))
	}
	done, err := r.Store.CompleteSecurityScan(ctx, sc.ID, risk, findings, note)
	if err != nil {
		return err
	}
	if !done {
		return nil // ended by another poll, which records it and builds the draft
	}
	tenant := sc.Tenant
	_ = r.Store.Record(ctx, nil, &tenant, "security.scan_completed", sc.App,
		map[string]any{"target": sc.Target, "domain": sc.Domain, "findings": len(findings)})
	r.draft(ctx, sc.ID)
	r.Log.Info("security scan completed", slog.Group("detail", slog.String("tenant", sc.Tenant), slog.String("app", sc.App),
		slog.String("target", sc.Target), slog.Int("findings", len(findings))))
	return nil
}

// draft builds the draft of a scan with a deadline of its own: reading
// the findings may have used most of the poll's.
func (r *Runner) draft(ctx context.Context, scan int64) {
	if r.AfterComplete == nil {
		return
	}
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pollTimeout)
	defer cancel()
	r.AfterComplete(dctx, scan)
}

func (r *Runner) retryDrafts(ctx context.Context) {
	if r.AfterComplete == nil || ctx.Err() != nil {
		return
	}
	ids, err := r.Store.PendingSecurityDrafts(ctx, perRound)
	if err != nil {
		return
	}
	for _, id := range ids {
		r.draft(ctx, id)
	}
}

func (r *Runner) fail(ctx context.Context, sc store.SecurityScan, reason string) {
	if err := r.Store.FailSecurityScan(ctx, sc.ID, reason); err != nil {
		r.Log.Error("security scans: recording a failure", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
		return
	}
	tenant := sc.Tenant
	_ = r.Store.Record(ctx, nil, &tenant, "security.scan_failed", sc.App, map[string]any{"target": sc.Target, "reason": reason})
}

var severities = map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "info": true}

// Finding is the stored copy of a scanner's finding: severities the
// contract does not know become info, long texts are cut.
func Finding(v cveradar.Vulnerability) store.SecurityFinding {
	sev := strings.ToLower(strings.TrimSpace(v.Severity))
	if !severities[sev] {
		sev = "info"
	}
	cut := func(p *string, n int) *string {
		if p == nil {
			return nil
		}
		s := clip(*p, n)
		return &s
	}
	return store.SecurityFinding{
		RemoteID: clip(v.ID, 100), Source: clip(v.Source, 40), Title: clip(v.Title, 500), Severity: sev, CVSS: v.CVSS,
		CVE: limit(v.CVE, 20, 30), CWE: limit(v.CWE, 20, 30), URL: cut(v.URL, 2000), HTTPMethod: cut(v.HTTPMethod, 10),
		Parameter: cut(v.Parameter, 200), Evidence: cut(v.Evidence, 2000), FirstSeenAt: v.FirstSeenAt, LastSeenAt: v.LastSeenAt,
	}
}

func limit(list []string, n, each int) []string {
	out := []string{}
	for i, s := range list {
		if i == n {
			break
		}
		out = append(out, clip(s, each))
	}
	return out
}

// clip cuts s to n characters and drops NUL, which PostgreSQL text refuses.
func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
