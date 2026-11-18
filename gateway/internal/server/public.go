// Package server espone il traffico pubblico e gli endpoint di amministrazione.
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// MapSource fornisce la mappa attiva.
type MapSource interface {
	Current() *mapstore.Active
}

// Public gestisce il traffico pubblico. Per ora instrada e blocca: la
// ricostruzione verso il legacy arriva nel prossimo passo (vedi CHANGELOG).
type Public struct {
	Maps    MapSource
	IPs     *clientip.Resolver
	Log     *slog.Logger
	Events  *telemetry.Emitter
	nowFunc func() time.Time
}

// Handler restituisce l'handler HTTP con request_id e log di accesso.
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
	p.Log.InfoContext(r.Context(), "richiesta",
		slog.Group("detail",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path), // mai la query: può contenere dati personali
			slog.Int("status", rec.status),
			slog.String("route", routeID),
			slog.Float64("duration_ms", float64(p.now().Sub(start).Microseconds())/1000),
		),
	)
}

// handle risponde alla richiesta e restituisce l'id della rotta trovata ("" se nessuna).
func (p *Public) handle(w http.ResponseWriter, r *http.Request) string {
	switch r.Method {
	case http.MethodOptions:
		// Risposto dal gateway, mai inoltrato. Nessuna politica CORS per ora.
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
		p.Log.ErrorContext(r.Context(), "errore di instradamento", slog.String("error", err.Error()))
		writePage(w, http.StatusInternalServerError, pageError)
		return ""
	}

	p.Log.DebugContext(r.Context(), "rotta trovata, ricostruzione non ancora implementata",
		slog.Group("detail", slog.String("route", m.Entry.ID), slog.String("kind", string(m.Entry.Kind))))
	writePage(w, http.StatusServiceUnavailable, pageUnavailable)
	return m.Entry.ID
}

func (p *Public) block(w http.ResponseWriter, r *http.Request, a *mapstore.Active, status int, reason string) {
	ev := telemetry.Event{
		Type:      "request.blocked",
		Reason:    telemetry.Ptr(reason),
		RequestID: telemetry.RequestID(r.Context()),
		SrcIP:     p.IPs.IP(r),
		Method:    r.Method,
		Path:      r.URL.Path,
		Status:    status,
	}
	if a != nil {
		ev.Tenant, ev.App = a.Model.Meta.Tenant, a.Model.Meta.App
		ev.MapVersion = telemetry.Ptr(a.Model.Meta.Version)
	}
	p.Events.Emit(ev)
	if status == http.StatusNotFound {
		writePage(w, status, pageNotFound)
		return
	}
	writePage(w, status, pageNotAllowed)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
