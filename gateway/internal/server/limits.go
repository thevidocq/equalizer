package server

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// How often a failing limits store is reported in the logs.
const storeErrorEvery = 10 * time.Second

var lastStoreError atomic.Int64

// subject identifies the visitor for limits.
func (p *Public) subject(r *http.Request) limits.Subject {
	ss := sessionOf(r)
	subj := limits.Subject{IP: p.IPs.IP(r), Authenticated: ss.authenticated()}
	if h := ss.hash(); h != nil {
		subj.Session = *h
	}
	return subj
}

// checkLimits applies behaviour marks and rate limits before serving.
// It returns false when it already answered.
func (p *Public) checkLimits(w http.ResponseWriter, r *http.Request, st *site.Site, entry string, withDefaults bool) bool {
	d, err := st.Limits.Check(r.Context(), p.subject(r), entry, withDefaults)
	if err != nil {
		p.storeError(r, err)
		return true // fail open: an unreachable store must not take the site down
	}
	return p.enforce(w, r, st, entry, d)
}

// observe feeds the behaviour rules; it returns false when it already answered.
func (p *Public) observe(w http.ResponseWriter, r *http.Request, st *site.Site, entry string, obs limits.Observation, enforce bool) bool {
	alerts, d, err := st.Limits.Observe(r.Context(), p.subject(r), entry, obs)
	if err != nil {
		p.storeError(r, err)
		return true
	}
	for _, a := range alerts {
		ev := p.event(r, st, "behavior.alert", a.Rule, 0, telemetry.Ptr(entry))
		ev.Detail = map[string]any{"signal": a.Signal, "count": a.Count}
		p.Events.Emit(ev)
	}
	if !enforce {
		return true
	}
	return p.enforce(w, r, st, entry, d)
}

func (p *Public) enforce(w http.ResponseWriter, r *http.Request, st *site.Site, entry string, d limits.Decision) bool {
	if d.Allowed() {
		return true
	}
	typ := "ratelimit.hit"
	if d.Status == http.StatusForbidden {
		typ = "request.blocked"
	}
	ev := p.event(r, st, typ, d.Reason, d.Status, telemetry.Ptr(entry))
	ev.Detail = map[string]any{"rule": d.Rule}
	p.Events.Emit(ev)
	if d.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.RetryAfter.Seconds()))))
	}
	if d.Status == http.StatusForbidden {
		writePage(w, d.Status, pageForbidden)
	} else {
		writePage(w, d.Status, pageTooMany)
	}
	return false
}

// acquireLegacy applies the legacy budget (concurrent requests in this
// replica, requests per second across replicas). The caller releases with
// st.Budget.Release when it returns true.
func (p *Public) acquireLegacy(r *http.Request, st *site.Site, entry string) bool {
	ok, err := st.Limits.LegacyAllowed(r.Context())
	if err != nil {
		p.storeError(r, err)
	}
	if !ok {
		p.Log.WarnContext(r.Context(), "legacy requests per second exhausted", slog.Group("detail", slog.String("route", entry)))
		return false
	}
	if !st.Budget.Acquire(r.Context()) {
		p.Log.WarnContext(r.Context(), "legacy budget exhausted",
			slog.Group("detail", slog.String("route", entry), slog.Int("in_flight", st.Budget.InFlight())))
		return false
	}
	return true
}

func (p *Public) storeError(r *http.Request, err error) {
	now := time.Now().UnixNano()
	last := lastStoreError.Load()
	if now-last < int64(storeErrorEvery) || !lastStoreError.CompareAndSwap(last, now) {
		return
	}
	p.Log.ErrorContext(r.Context(), "limits store unavailable: limits not applied", slog.String("error", err.Error()))
}
