package server

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/origin"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// serveRoute serves a read route: it checks the role, validates parameters
// and query, rebuilds the request towards the legacy application and returns
// the filtered response, with faithful rendering for HTML pages.
func (p *Public) serveRoute(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match) {
	start := p.now()
	route, ok := st.Model.RouteByID(m.Entry.ID)
	if !ok {
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	ss := sessionOf(r)
	if !site.RoleAllowed(ss.role(st), route.Roles) {
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

	if !p.authorize(w, r, st, route.ID, route.Authz, placeholder.Values{"params": params, "query": queryVals}, false) {
		return
	}
	if !p.observe(w, r, st, route.ID, limits.Observation{Params: merge(params, queryVals)}, true) {
		return
	}

	req, err := st.Legacy.Build(r.Context(), route.LegacyRequest, placeholder.Values{"params": params, "query": queryVals})
	if err != nil {
		p.Log.ErrorContext(r.Context(), "cannot build the legacy request",
			slog.String("error", err.Error()), slog.Group("detail", slog.String("route", route.ID)))
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}

	p.prepare(req, r, st, ss)
	if !p.acquireLegacy(r, st, route.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Do(req)
	st.Budget.Release()
	if err != nil {
		p.legacyFailure(w, r, st, route.ID, err)
		return
	}
	p.absorbSession(r, st, ss, resp)
	if route.Session != nil && route.Session.End {
		ss.end()
	}

	status := p.writeLegacy(w, r, st, route.ID, req.URL, resp)
	ev := p.event(r, st, "request.served", "", status, telemetry.Ptr(route.ID))
	total := float64(p.now().Sub(start).Microseconds()) / 1000
	legacyMS := float64(resp.Duration.Microseconds()) / 1000
	ev.LatencyMS = &telemetry.Latency{Gateway: max(total-legacyMS, 0), Legacy: legacyMS}
	p.Events.Emit(ev)
}

// absorbSession keeps the legacy cookies and drops the role when the legacy
// application says its session ended.
func (p *Public) absorbSession(r *http.Request, st *site.Site, ss *sessionState, resp *legacyclient.Response) {
	ss.absorb(resp.Cookies)
	if ss.authenticated() && legacySessionExpired(st.Model.Legacy.SessionExpired, resp.Status, resp.Location, resp.Body) {
		ss.expire()
		p.Log.InfoContext(r.Context(), "legacy session expired: role dropped")
	}
}

// writeLegacy writes the legacy response and returns the status sent.
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
	case resp.Status == http.StatusUnauthorized && resp.Header.Get("WWW-Authenticate") != "":
		// The legacy site's HTTP login: the browser asks for the password.
		securityHeaders(w.Header())
		w.Header().Set("WWW-Authenticate", resp.Header.Get("WWW-Authenticate"))
		writePage(w, http.StatusUnauthorized, pageForbidden)
		return http.StatusUnauthorized
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
	if pipeline.IsHTML(resp.Header.Get("Content-Type")) && pipeline.Markup(resp.Body) {
		forms := p.newBinder(r, st.Forms)
		ctx := &pipeline.Context{PageURL: pageURL, URLs: p.issuing(st, sessionOf(r)), Forms: forms.pipelineBinder()}
		out, err := st.Pipeline.Render(resp.Body, resp.Header.Get("Content-Type"), ctx)
		if err != nil {
			// Fail closed: an error is better than a badly rendered page.
			p.Log.ErrorContext(r.Context(), "faithful rendering failed",
				slog.String("error", err.Error()), slog.Group("detail", slog.String("route", routeID)))
			writePage(w, http.StatusBadGateway, pageBadGateway)
			return http.StatusBadGateway
		}
		body = out
		st.NoteCharset(ctx.Stats.Charset)
		h.Set("Content-Type", pipeline.ContentType)
		forms.setCookie(w)
		p.Log.DebugContext(r.Context(), "page rendered", slog.Group("detail",
			slog.String("route", routeID), slog.String("charset", ctx.Stats.Charset),
			slog.Int("rewritten", ctx.Stats.Rewritten), slog.Int("unmapped", ctx.Stats.Unmapped),
			slog.Int("removed", ctx.Stats.Removed), slog.Int("forms", ctx.Stats.Forms),
			slog.Int("forms_unmapped", ctx.Stats.FormsUnmapped), slog.Any("unmapped_paths", ctx.Stats.UnmappedPaths)))
	}
	if st.Origin != nil && origin.Text(h.Get("Content-Type")) {
		body = st.Origin.Bytes(body)
		h.Del("Content-Length")
	}
	securityHeaders(h)
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	return resp.Status
}

// redirect turns a legacy redirect into a redirect towards the gateway.
// Redirects to external hosts are blocked: allowed external destinations
// will arrive with external_links.
func (p *Public) redirect(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, pageURL *url.URL, resp *legacyclient.Response) int {
	loc, err := url.Parse(resp.Location)
	if err == nil {
		loc = pageURL.ResolveReference(loc)
	}
	if err != nil || !st.URLs.Internal(loc) {
		p.Log.WarnContext(r.Context(), "legacy redirect to a destination that is not allowed",
			slog.Group("detail", slog.String("route", routeID), slog.Int("status", resp.Status)))
		writePage(w, http.StatusBadGateway, pageBadGateway)
		return http.StatusBadGateway
	}
	target, ok := st.URLs.Public(loc)
	if !ok {
		target = st.URLs.Fallback(loc)
		p.Log.InfoContext(r.Context(), "legacy redirect to an unmapped path",
			slog.Group("detail", slog.String("route", routeID), slog.String("legacy_path", loc.Path)))
	}
	(&issuer{URLMapper: st.URLs, st: st, ss: sessionOf(r)}).note(target)
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
	ev.Detail = map[string]any{"field": field} // never the value
	p.Events.Emit(ev)
	writePage(w, http.StatusBadRequest, pageBadRequest)
}

func (p *Public) legacyFailure(w http.ResponseWriter, r *http.Request, st *site.Site, routeID string, err error) {
	reason := "legacy_unreachable"
	switch {
	case errors.Is(err, transport.ErrDestinationNotAllowed):
		reason = "destination_not_allowed"
	case errors.Is(err, transport.ErrConnectorUnavailable):
		reason = "connector_unavailable"
	case errors.Is(err, transport.ErrLegacyUnreachable):
		reason = "legacy_unreachable"
	case errors.Is(err, legacyclient.ErrTooLarge):
		reason = "response_too_large"
	}
	ev := p.event(r, st, "legacy.error", reason, http.StatusBadGateway, telemetry.Ptr(routeID))
	p.Events.Emit(ev)
	p.Log.WarnContext(r.Context(), "legacy application unreachable",
		slog.String("error", err.Error()), slog.Group("detail", slog.String("route", routeID), slog.String("reason", reason)))
	writePage(w, http.StatusBadGateway, pageBadGateway)
}

// securityHeaders adds the modern headers the legacy application does not set.
func securityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Strict-Transport-Security", "max-age=31536000")
}

func merge(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range b {
		out[k] = v
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

func single(m map[string]string) url.Values {
	out := make(url.Values, len(m))
	for k, v := range m {
		out[k] = []string{v}
	}
	return out
}
