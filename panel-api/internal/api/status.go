package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The status page: for each site what it needs to work; for the platform,
// whether its services answer (never tenant data).

const statusDays = 7

func (s *Server) tenantStatus(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.CanRead(tenant) {
		forbidden(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	sites, logsOK, err := s.SiteStatuses(ctx, tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": sites, "checked_at": time.Now().UTC(), "logs_available": logsOK})
}

// SiteStatuses evaluates every site of a tenant, as the status page shows
// them (also read by the email alerts).
func (s *Server) SiteStatuses(ctx context.Context, tenant string) ([]health.Site, bool, error) {
	apps, err := s.Store.Apps(ctx, tenant)
	if err != nil {
		return nil, false, err
	}
	var tunnels map[string]int
	var versions map[string][]string
	if s.Health != nil {
		if v, ok := s.Health.TunnelVersions(ctx); ok {
			versions, tunnels = v, map[string]int{}
			for id, vs := range v {
				tunnels[id] = len(vs)
			}
		}
	}
	recent, errs, drops, dates, logsOK := s.statusLogs(ctx, tenant)
	sites := make([]health.Site, 0, len(apps))
	for _, a := range apps {
		in := health.Input{App: a.ID, Name: a.Name, Domain: a.PublicDomain, Tunnels: tunnels, Traffic: logsOK, Exposure: a.Exposure.Status,
			TLSStatus: a.TLS.Status}
		if c := a.TLS.Certificate; c != nil {
			in.CertExpires = &c.NotAfter
		}
		if v, err := s.Store.Published(ctx, a.ID); err == nil {
			in.MapVersion, in.Map = v.Version, v.Document
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, false, err
		}
		in.Served, in.Errors = recent[a.ID]["request.served"], recent[a.ID]["legacy.error"]
		in.Versions, in.Platform = versions[health.Connector(in.Map)], s.Version
		site := health.Evaluate(in)
		site.Days = health.Days(dates, errs[a.ID], drops[health.Connector(in.Map)])
		sites = append(sites, site)
	}
	return sites, logsOK, nil
}

// statusLogs reads traffic and history; logsOK is false without a log store.
func (s *Server) statusLogs(ctx context.Context, tenant string) (map[string]map[string]int, map[string][]int, map[string][]int, []time.Time, bool) {
	dates := make([]time.Time, statusDays)
	day := 24 * time.Hour
	first := time.Now().UTC().Truncate(day).Add(-time.Duration(statusDays-1) * day)
	for i := range dates {
		dates[i] = first.Add(time.Duration(i) * day)
	}
	if s.Logs == nil {
		return nil, nil, nil, dates, false
	}
	recent, err := s.Logs.Recent(ctx, tenant, 15*time.Minute, "request.served", "legacy.error")
	if err != nil {
		s.Log.WarnContext(ctx, "status without traffic", "error", err.Error())
		return nil, nil, nil, dates, false
	}
	_, errs, err1 := s.Logs.Daily(ctx, tenant, "legacy.error", "app", statusDays)
	_, drops, err2 := s.Logs.Daily(ctx, tenant, "connector.down", "detail_connector_id", statusDays)
	if err1 != nil || err2 != nil {
		return recent, nil, nil, dates, true
	}
	return recent, errs, drops, dates, true
}

func (s *Server) platformStatus(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.Platform() {
		forbidden(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	services := []health.Check{{Name: "Database", State: health.OK, Detail: "answers"}}
	if err := s.Store.DB.PingContext(ctx); err != nil {
		services[0] = health.Check{Name: "Database", State: health.Down, Detail: "does not answer"}
	}
	if s.Health != nil {
		services = append(services, replicas("Gateway", s.Health, ctx, s.Health.GatewayAdmin),
			replicas("Tunnel relay", s.Health, ctx, s.Health.RelayAdmin))
		if s.Health.LokiReady(ctx) {
			services = append(services, health.Check{Name: "Log store", State: health.OK, Detail: "answers"})
		} else {
			services = append(services, health.Check{Name: "Log store", State: health.Down, Detail: "does not answer: the Logs page and traffic on the status page are unavailable"})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": services, "checked_at": time.Now().UTC()})
}

func replicas(name string, p *health.Probe, ctx context.Context, addr string) health.Check {
	if addr == "" {
		return health.Check{Name: name, State: health.Unknown, Detail: "address not configured"}
	}
	ready, running, total := p.Replicas(ctx, addr)
	return health.ReplicaCheck(name, ready, running, total)
}
