// Package server serves public traffic and the administration endpoints.
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/unmapped"
)

// MapSource provides the site (compiled map) served on a host.
type MapSource interface {
	For(host string) *site.Site
	Ready() bool // at least one site is loaded
}

// Public handles public traffic: it routes, validates, rebuilds the request
// towards the legacy application and returns the response. APIs are not
// served yet (see CHANGELOG).
type Public struct {
	Maps     MapSource
	IPs      *clientip.Resolver
	Log      *slog.Logger
	Events   *telemetry.Emitter
	Forms    *formtoken.Sealer  // nil: forms are neither bound nor accepted
	Sessions *session.Manager   // nil: no sessions, every visitor is anonymous
	Unmapped *unmapped.Reporter // nil: refused paths are not reported to the panel
	// OIDCSecrets gives the client secrets of sites logging in with OIDC.
	OIDCSecrets Secrets
	nowFunc     func() time.Time
}

// Handler returns the HTTP handler with request_id, access log and the
// guard against clients that hold the server (guard.go).
func (p *Public) Handler() http.Handler {
	return telemetry.RequestIDMiddleware(guard(p.Log, http.HandlerFunc(p.serve)))
}

func (p *Public) now() time.Time {
	if p.nowFunc != nil {
		return p.nowFunc()
	}
	return time.Now()
}

func (p *Public) serve(w http.ResponseWriter, r *http.Request) {
	start := p.now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	routeID := p.handle(rec, r)
	p.Log.InfoContext(r.Context(), "request",
		slog.Group("detail",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path), // never the query: it may hold personal data
			slog.Int("status", rec.status),
			slog.String("route", routeID),
			slog.Float64("duration_ms", float64(p.now().Sub(start).Microseconds())/1000),
		),
	)
}

// handle answers the request and returns the matched route id ("" if none).
func (p *Public) handle(w http.ResponseWriter, r *http.Request) string {
	switch r.Method {
	case http.MethodOptions:
		if !p.optionsToLegacy(r) {
			// Answered by the gateway: CORS preflights (no cross-site
			// policy for now) and areas that do not list OPTIONS.
			w.Header().Set("Allow", "GET, HEAD, POST")
			w.WriteHeader(http.StatusNoContent)
			return ""
		}
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		p.block(w, r, nil, http.StatusMethodNotAllowed, "method_not_allowed")
		return ""
	}

	active := p.Maps.For(r.Host)
	if active == nil {
		if !p.Maps.Ready() {
			writePage(w, http.StatusServiceUnavailable, pageUnavailable)
			return ""
		}
		if as, ok := p.Maps.(interface{ AliasOf(string) *site.Site }); ok {
			if s := as.AliasOf(r.Host); s != nil {
				// Another domain of the site: one address, as sites do.
				code := http.StatusMovedPermanently
				if r.Method != http.MethodGet && r.Method != http.MethodHead {
					code = http.StatusPermanentRedirect // keeps the method and the body
				}
				http.Redirect(w, r, s.Model.Meta.Scheme()+"://"+s.Model.Meta.PublicDomain+safeReturn(r.URL.RequestURI()), code)
				return "alias_redirect"
			}
		}
		// No published map for this domain: it does not exist here.
		p.block(w, r, nil, http.StatusNotFound, "unknown_host")
		return ""
	}

	if plainRequest(r) && !plainPort(active) {
		toHTTPS(w, r, active)
		return "https_redirect"
	}

	ss := p.loadSession(r, active)
	r = withSession(r, ss)
	w = &commitWriter{ResponseWriter: w, ctx: r.Context(), ss: ss}
	if p.oidcGate(w, r, active, ss) {
		return ""
	}

	m, err := active.Router.Match(r.Method, r.URL.Path)
	if errors.Is(err, router.ErrNotMapped) || errors.Is(err, router.ErrMethodNotAllowed) {
		if a := active.Model.AreaFor(r.URL.Path); a != nil {
			// No precise entry: the area serves it as the legacy site would.
			if !p.checkLimits(w, r, active, a.ID, false) {
				return a.ID
			}
			p.serveArea(w, r, active, a)
			return a.ID
		}
	}
	switch {
	case errors.Is(err, router.ErrNotMapped):
		p.Unmapped.Record(active.Model.Meta.App, r.Method, r.URL.Path, p.IPs.IP(r))
		p.block(w, r, active, http.StatusNotFound, "route_not_mapped")
		return ""
	case errors.Is(err, router.ErrMethodNotAllowed):
		p.block(w, r, active, http.StatusMethodNotAllowed, "method_not_allowed")
		return ""
	case err != nil:
		p.Log.ErrorContext(r.Context(), "routing error", slog.String("error", err.Error()))
		writePage(w, http.StatusInternalServerError, pageError)
		return ""
	}

	// A path that only looks like a precise entry (/{slug}/ against
	// /wp-login.php), or a query the route does not declare, belongs to the
	// area: both are part of the match. Values of a real match are still
	// checked.
	if a := active.Model.AreaFor(r.URL.Path); a != nil && (!pathFits(active, m) || !queryFits(active, m, r.URL.RawQuery)) {
		if !p.checkLimits(w, r, active, a.ID, false) {
			return a.ID
		}
		p.serveArea(w, r, active, a)
		return a.ID
	}

	// Assets: only behaviour blocks, never the page rate limits.
	if !p.checkLimits(w, r, active, m.Entry.ID, m.Entry.Kind != router.KindAsset) {
		return m.Entry.ID
	}

	switch m.Entry.Kind {
	case router.KindRoute:
		p.serveRoute(w, r, active, m)
		return m.Entry.ID
	case router.KindAsset:
		p.serveAsset(w, r, active, m)
		return m.Entry.ID
	case router.KindForm:
		p.serveForm(w, r, active, m)
		return m.Entry.ID
	case router.KindAPI:
		p.serveAPI(w, r, active, m)
		return m.Entry.ID
	}
	p.Log.DebugContext(r.Context(), "entry matched, kind not served yet",
		slog.Group("detail", slog.String("route", m.Entry.ID), slog.String("kind", string(m.Entry.Kind))))
	writePage(w, http.StatusServiceUnavailable, pageUnavailable)
	return m.Entry.ID
}

func (p *Public) block(w http.ResponseWriter, r *http.Request, st *site.Site, status int, reason string) {
	p.Events.Emit(p.event(r, st, "request.blocked", reason, status, nil))
	switch status {
	case http.StatusNotFound:
		writePage(w, status, pageNotFound)
	case http.StatusForbidden:
		writePage(w, status, pageForbidden)
	default:
		writePage(w, status, pageNotAllowed)
	}
}

// event prepares an event with the request's common fields.
func (p *Public) event(r *http.Request, st *site.Site, typ, reason string, status int, route *string) telemetry.Event {
	ev := telemetry.Event{
		Type:      typ,
		RequestID: telemetry.RequestID(r.Context()),
		SrcIP:     p.IPs.IP(r),
		Method:    r.Method,
		Path:      r.URL.Path, // never the query: it may hold personal data
		Status:    status,
		Route:     route,
	}
	if reason != "" {
		ev.Reason = telemetry.Ptr(reason)
	}
	if ss, ok := r.Context().Value(sessionKey{}).(*sessionState); ok {
		ev.Session = ss.hash()
	}
	if st != nil {
		ev.Tenant, ev.App = st.Model.Meta.Tenant, st.Model.Meta.App
		ev.MapVersion = telemetry.Ptr(st.Model.Meta.Version)
	}
	return ev
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the server's writer (flushing
// live updates).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
