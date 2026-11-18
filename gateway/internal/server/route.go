package server

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// serveRoute serve una rotta in lettura: controlla il ruolo, valida
// parametri e query, ricostruisce la richiesta verso il legacy e restituisce
// la risposta filtrata, con la resa fedele per le pagine HTML.
func (p *Public) serveRoute(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match) {
	start := p.now()
	route, ok := st.Model.RouteByID(m.Entry.ID)
	if !ok {
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	// Autenticazione non ancora presente: ogni visitatore ha il ruolo anonimo.
	if !site.RoleAllowed(st.Model.Roles.Anonymous, route.Roles) {
		p.block(w, r, st, http.StatusForbidden, "authz_denied")
		return
	}

	params, err := st.Validator.Fields(route.Params, single(m.Params))
	if err != nil {
		p.reject(w, r, st, route.ID, err)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		p.reject(w, r, st, route.ID, &validate.Error{Field: "(query)", Reason: "malformed_query"})
		return
	}
	queryVals, err := st.Validator.Fields(route.Query, query)
	if err != nil {
		p.reject(w, r, st, route.ID, err)
		return
	}

	req, err := st.Legacy.Build(r.Context(), route.LegacyRequest, placeholder.Values{"params": params, "query": queryVals})
	if err != nil {
		p.Log.ErrorContext(r.Context(), "richiesta verso il legacy non costruibile",
			slog.String("error", err.Error()), slog.Group("detail", slog.String("route", route.ID)))
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}

	if !st.Budget.Acquire(r.Context()) {
		p.Log.WarnContext(r.Context(), "budget verso il legacy esaurito",
			slog.Group("detail", slog.String("route", route.ID), slog.Int("in_flight", st.Budget.InFlight())))
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Do(req)
	st.Budget.Release()
	if err != nil {
		p.legacyFailure(w, r, st, route.ID, err)
		return
	}

	status := p.writeLegacy(w, r, st, route.ID, req.URL, resp)
	ev := p.event(r, st, "request.served", "", status, telemetry.Ptr(route.ID))
	total := float64(p.now().Sub(start).Microseconds()) / 1000
	legacyMS := float64(resp.Duration.Microseconds()) / 1000
	ev.LatencyMS = &telemetry.Latency{Gateway: max(total-legacyMS, 0), Legacy: legacyMS}
	p.Events.Emit(ev)
}

// writeLegacy scrive la risposta del legacy e restituisce lo status inviato.
func (p *Public) writeLegacy(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, pageURL *url.URL, resp *legacyclient.Response) int {
	switch {
	case resp.Status >= 500:
		writePage(w, http.StatusBadGateway, pageBadGateway)
		return http.StatusBadGateway
	case resp.Status == http.StatusNotFound || resp.Status == http.StatusGone:
		writePage(w, http.StatusNotFound, pageNotFound)
		return http.StatusNotFound
	case resp.Status >= 300 && resp.Status < 400:
		return p.redirect(w, r, st, routeID, pageURL, resp)
	case resp.Status >= 400:
		writePage(w, http.StatusBadRequest, pageBadRequest)
		return http.StatusBadRequest
	}

	body := resp.Body
	h := w.Header()
	for k, vs := range resp.Header {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	if pipeline.IsHTML(resp.Header.Get("Content-Type")) {
		ctx := &pipeline.Context{PageURL: pageURL, URLs: st.URLs}
		out, err := st.Pipeline.Render(resp.Body, resp.Header.Get("Content-Type"), ctx)
		if err != nil {
			// Fail closed: meglio un errore di una pagina resa male.
			p.Log.ErrorContext(r.Context(), "resa fedele non riuscita",
				slog.String("error", err.Error()), slog.Group("detail", slog.String("route", routeID)))
			writePage(w, http.StatusBadGateway, pageBadGateway)
			return http.StatusBadGateway
		}
		body = out
		h.Set("Content-Type", pipeline.ContentType)
		p.Log.DebugContext(r.Context(), "pagina resa", slog.Group("detail",
			slog.String("route", routeID), slog.String("charset", ctx.Stats.Charset),
			slog.Int("rewritten", ctx.Stats.Rewritten), slog.Int("unmapped", ctx.Stats.Unmapped),
			slog.Int("removed", ctx.Stats.Removed), slog.Any("unmapped_paths", ctx.Stats.UnmappedPaths)))
	}
	securityHeaders(h)
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	return resp.Status
}

// redirect traduce un redirect del legacy in un redirect verso il gateway.
// Un redirect verso un host esterno viene bloccato: le destinazioni esterne
// ammesse arriveranno con external_links.
func (p *Public) redirect(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, pageURL *url.URL, resp *legacyclient.Response) int {
	loc, err := url.Parse(resp.Location)
	if err == nil {
		loc = pageURL.ResolveReference(loc)
	}
	if err != nil || !st.URLs.Internal(loc) {
		p.Log.WarnContext(r.Context(), "redirect del legacy verso destinazione non ammessa",
			slog.Group("detail", slog.String("route", routeID), slog.Int("status", resp.Status)))
		writePage(w, http.StatusBadGateway, pageBadGateway)
		return http.StatusBadGateway
	}
	target, ok := st.URLs.Public(loc)
	if !ok {
		target = st.URLs.Fallback(loc)
		p.Log.InfoContext(r.Context(), "redirect del legacy verso un percorso non mappato",
			slog.Group("detail", slog.String("route", routeID), slog.String("legacy_path", loc.Path)))
	}
	securityHeaders(w.Header())
	w.Header().Set("Location", target)
	w.WriteHeader(resp.Status)
	return resp.Status
}

func (p *Public) reject(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, err error) {
	reason, field := "invalid", ""
	var ve *validate.Error
	if errors.As(err, &ve) {
		reason, field = ve.Reason, ve.Field
	}
	ev := p.event(r, st, "field.rejected", reason, http.StatusBadRequest, telemetry.Ptr(routeID))
	ev.Detail = map[string]any{"field": field} // mai il valore
	p.Events.Emit(ev)
	writePage(w, http.StatusBadRequest, pageBadRequest)
}

func (p *Public) legacyFailure(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, err error) {
	reason := "legacy_unreachable"
	switch {
	case errors.Is(err, transport.ErrDestinationNotAllowed):
		reason = "destination_not_allowed"
	case errors.Is(err, site.ErrConnectorUnavailable):
		reason = "connector_unavailable"
	case errors.Is(err, legacyclient.ErrTooLarge):
		reason = "response_too_large"
	}
	ev := p.event(r, st, "legacy.error", reason, http.StatusBadGateway, telemetry.Ptr(routeID))
	p.Events.Emit(ev)
	p.Log.WarnContext(r.Context(), "legacy non raggiungibile",
		slog.String("error", err.Error()), slog.Group("detail", slog.String("route", routeID), slog.String("reason", reason)))
	writePage(w, http.StatusBadGateway, pageBadGateway)
}

// securityHeaders aggiunge gli header moderni che il legacy non mette.
func securityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Strict-Transport-Security", "max-age=31536000")
}

func single(m map[string]string) url.Values {
	out := make(url.Values, len(m))
	for k, v := range m {
		out[k] = []string{v}
	}
	return out
}
