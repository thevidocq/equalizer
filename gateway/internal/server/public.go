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
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// MapSource provides the active site (compiled map).
type MapSource interface {
	Current() *site.Site
}

// Public handles public traffic: it routes, validates, rebuilds the request
// towards the legacy application and returns the response. APIs are not
// served yet (see CHANGELOG).
type Public struct {
	Maps    MapSource
	IPs     *clientip.Resolver
	Log     *slog.Logger
	Events  *telemetry.Emitter
	Forms   *formtoken.Sealer // nil: forms are neither bound nor accepted
	nowFunc func() time.Time
}

// Handler returns the HTTP handler with request_id and access log.
func (p *Public) Handler() http.Handler {
	return telemetry.RequestIDMiddleware(http.HandlerFunc(p.serve))
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
		// Answered by the gateway, never forwarded. No CORS policy for now.
		w.Header().Set("Allow", "GET, HEAD, POST")
		w.WriteHeader(http.StatusNoContent)
		return ""
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		p.block(w, r, nil, http.StatusMethodNotAllowed, "method_not_allowed")
		return ""
	}

	active := p.Maps.Current()
	if active == nil {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return ""
	}

	m, err := active.Router.Match(r.Method, r.URL.Path)
	switch {
	case errors.Is(err, router.ErrNotMapped):
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
