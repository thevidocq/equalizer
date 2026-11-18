// Package server espone il traffico pubblico e gli endpoint di amministrazione.
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// MapSource fornisce il sito attivo (mappa compilata).
type MapSource interface {
	Current() *site.Site
}

// Public gestisce il traffico pubblico: instrada, valida, ricostruisce la
// richiesta verso il legacy e restituisce la risposta. Form e API non sono
// ancora serviti (vedi CHANGELOG).
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

	if m.Entry.Kind == router.KindRoute {
		p.serveRoute(w, r, active, m)
		return m.Entry.ID
	}
	p.Log.DebugContext(r.Context(), "voce trovata, tipo non ancora servito",
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

// event prepara un evento con i campi comuni della richiesta.
func (p *Public) event(r *http.Request, st *site.Site, typ, reason string, status int, route *string) telemetry.Event {
	ev := telemetry.Event{
		Type:      typ,
		RequestID: telemetry.RequestID(r.Context()),
		SrcIP:     p.IPs.IP(r),
		Method:    r.Method,
		Path:      r.URL.Path, // mai la query: può contenere dati personali
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
