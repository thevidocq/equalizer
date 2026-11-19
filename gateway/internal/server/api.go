package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/jsonshape"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// serveAPI serves a JSON endpoint of the map: role, path, query and body
// checked field by field, the request rebuilt towards the legacy
// application, and only the declared fields of its answer returned.
func (p *Public) serveAPI(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match) {
	start := p.now()
	api, ok := st.Model.APIByID(m.Entry.ID)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "error")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
		p.Events.Emit(p.event(r, st, "request.blocked", "cross_origin", http.StatusForbidden, telemetry.Ptr(api.ID)))
		writeAPIError(w, http.StatusForbidden, "forbidden")
		return
	}
	ss := sessionOf(r)
	if !site.RoleAllowed(ss.role(st), api.Roles) {
		p.Events.Emit(p.event(r, st, "request.blocked", "authz_denied", http.StatusForbidden, telemetry.Ptr(api.ID)))
		writeAPIError(w, http.StatusForbidden, "forbidden")
		return
	}
	vals, err := p.apiValues(r, st, api, m)
	if err != nil {
		p.rejectAPI(w, r, st, api.ID, err)
		return
	}
	if !p.authorize(w, r, st, api.ID, api.Authz, vals, true) {
		return
	}
	if !p.observe(w, r, st, api.ID, limits.Observation{Params: merge(vals["params"], vals["query"])}, true) {
		return
	}
	kinds := legacyclient.Kinds(jsonshape.Kinds(st.Validator, "body", api.Body)) // never nil
	req, err := st.Legacy.BuildTyped(r.Context(), api.LegacyRequest, vals, kinds)
	if err != nil {
		p.Log.ErrorContext(r.Context(), "cannot build the legacy request",
			slog.String("error", err.Error()), slog.Group("detail", slog.String("route", api.ID)))
		writeAPIError(w, http.StatusInternalServerError, "error")
		return
	}
	p.prepare(req, r, st, ss)
	if !p.acquireLegacy(r, st, api.ID) {
		writeAPIError(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	resp, err := st.Legacy.Do(req)
	st.Budget.Release()
	if err != nil {
		p.legacyFailure(apiErrorWriter{w}, r, st, api.ID, err)
		return
	}
	p.absorbSession(r, st, ss, resp)

	status := p.writeAPI(w, r, st, api, resp)
	ev := p.event(r, st, "request.served", "", status, telemetry.Ptr(api.ID))
	total := float64(p.now().Sub(start).Microseconds()) / 1000
	legacyMS := float64(resp.Duration.Microseconds()) / 1000
	ev.LatencyMS = &telemetry.Latency{Gateway: max(total-legacyMS, 0), Legacy: legacyMS}
	p.Events.Emit(ev)
}

func (p *Public) apiValues(r *http.Request, st *site.Site, api *mapmodel.APIRoute, m router.Match) (placeholder.Values, error) {
	params, err := st.Validator.Fields(api.Params, single(m.Params))
	if err != nil {
		return nil, err
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, &validate.Error{Field: "(query)", Reason: "malformed_query"}
	}
	queryVals, err := st.Validator.Fields(api.Query, query)
	if err != nil {
		return nil, err
	}
	vals := placeholder.Values{"params": params, "query": queryVals, "body": {}}
	if len(api.Body) == 0 {
		return vals, nil
	}
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		return nil, &validate.Error{Field: "(body)", Reason: jsonshape.ReasonNotJSON}
	}
	if vals["body"], err = jsonshape.Body(st.Validator, api.Body, r.Body); err != nil {
		return nil, err
	}
	return vals, nil
}

// writeAPI returns the declared part of a legacy JSON answer. Legacy errors
// become a short JSON error: their body never reaches the client.
func (p *Public) writeAPI(w http.ResponseWriter, r *http.Request, st *site.Site, api *mapmodel.APIRoute, resp *legacyclient.Response) int {
	switch {
	case resp.Status >= 500:
		writeAPIError(w, http.StatusBadGateway, "unavailable")
		return http.StatusBadGateway
	case resp.Status == http.StatusNotFound || resp.Status == http.StatusGone:
		writeAPIError(w, http.StatusNotFound, "not_found")
		return http.StatusNotFound
	case resp.Status == http.StatusUnauthorized || resp.Status == http.StatusForbidden:
		writeAPIError(w, http.StatusForbidden, "forbidden")
		return http.StatusForbidden
	case resp.Status >= 300:
		// Redirects and other answers have no place in a JSON endpoint.
		writeAPIError(w, http.StatusBadRequest, "rejected")
		return http.StatusBadRequest
	}
	if resp.Status == http.StatusNoContent || len(strings.TrimSpace(string(resp.Body))) == 0 {
		apiHeaders(w.Header())
		w.WriteHeader(http.StatusNoContent)
		return http.StatusNoContent
	}
	out, err := jsonshape.Response(st.Validator, api.Response, resp.Body)
	if err != nil {
		// Fail closed: the legacy answer changed shape.
		p.Log.WarnContext(r.Context(), "legacy JSON not as declared",
			slog.String("error", err.Error()), slog.Group("detail", slog.String("route", api.ID)))
		writeAPIError(w, http.StatusBadGateway, "unavailable")
		return http.StatusBadGateway
	}
	issueFromJSON(sessionOf(r), api, out)
	apiHeaders(w.Header())
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(st.Origin.Bytes(out))
	}
	return resp.Status
}

func (p *Public) rejectAPI(w http.ResponseWriter, r *http.Request, st *site.Site, id string, err error) {
	reason, field := "invalid", ""
	var ve *validate.Error
	if errors.As(err, &ve) {
		reason, field = ve.Reason, ve.Field
	}
	ev := p.event(r, st, "field.rejected", reason, http.StatusBadRequest, telemetry.Ptr(id))
	ev.Detail = map[string]any{"field": field} // never the value
	p.Events.Emit(ev)
	apiHeaders(w.Header())
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid", "field": field})
}

// sameOrigin refuses state-changing calls from other sites: the browser
// says where the call comes from (Sec-Fetch-Site, Origin).
func sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "cross-site", "same-site":
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

func apiHeaders(h http.Header) {
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	securityHeaders(h)
}

func writeAPIError(w http.ResponseWriter, status int, code string) {
	apiHeaders(w.Header())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// apiErrorWriter turns the HTML error pages of shared helpers into JSON.
type apiErrorWriter struct{ http.ResponseWriter }

func (a apiErrorWriter) WriteHeader(code int) {
	apiHeaders(a.ResponseWriter.Header())
	a.ResponseWriter.WriteHeader(code)
}

func (a apiErrorWriter) Write([]byte) (int, error) {
	return a.ResponseWriter.Write([]byte(`{"error":"unavailable"}` + "\n"))
}
