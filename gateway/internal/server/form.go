package server

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/charset"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// MaxFormBody is the largest form body accepted (1 MiB).
const MaxFormBody = 1 << 20

// serveForm serves a form submission: it checks role, origin and token,
// validates the declared fields, re-encodes them in the legacy charset and
// rebuilds the POST towards the legacy application.
func (p *Public) serveForm(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match) {
	start := p.now()
	form, ok := st.Model.FormByID(m.Entry.ID)
	if !ok {
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	ss := sessionOf(r)
	if !site.RoleAllowed(ss.role(st), form.Roles) {
		p.block(w, r, st, http.StatusForbidden, "authz_denied")
		return
	}
	if reason := st.FormUnsupported(form.ID); reason != "" {
		p.Log.WarnContext(r.Context(), "form not served yet",
			slog.Group("detail", slog.String("route", form.ID), slog.String("reason", reason)))
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	if form.Session != nil && form.Session.Start != "" && ss.broken {
		p.Log.ErrorContext(r.Context(), "login refused: no session store", slog.Group("detail", slog.String("route", form.ID)))
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	if crossSite(r) {
		p.block(w, r, st, http.StatusForbidden, "cross_site_request")
		return
	}
	body, uploads, status, reason := readSubmission(w, r, form)
	if reason != "" {
		p.block(w, r, st, status, reason)
		return
	}

	state, reason := p.openToken(r, body)
	if reason != "" {
		p.block(w, r, st, http.StatusForbidden, reason)
		return
	}
	params, err := st.Validator.Fields(form.Params, single(m.Params))
	if err != nil {
		p.reject(w, r, st, form.ID, err)
		return
	}
	if state.Form != form.ID || !sameParams(st, form, params, state.Params) {
		// The token was issued for another form or another record.
		p.block(w, r, st, http.StatusForbidden, "form_token_mismatch")
		return
	}
	text, fileSpecs := splitFiles(form.Fields)
	fields, err := st.Validator.Fields(text, body)
	if err != nil {
		p.reject(w, r, st, form.ID, err)
		return
	}
	files, err := p.checkFiles(r, fileSpecs, uploads)
	if err != nil {
		p.reject(w, r, st, form.ID, err)
		return
	}
	vals, err := encodeValues(state.Charset, map[string]map[string]string{
		"params": params, "fields": fields, "hidden": state.Hidden,
	})
	if err != nil {
		p.reject(w, r, st, form.ID, err)
		return
	}

	var req *http.Request
	if len(form.LegacyRequest.Files) > 0 {
		req, err = st.Legacy.BuildUpload(r.Context(), form.LegacyRequest, vals, files)
	} else {
		req, err = st.Legacy.Build(r.Context(), form.LegacyRequest, vals)
	}
	if err != nil {
		p.Log.ErrorContext(r.Context(), "cannot build the legacy request",
			slog.String("error", err.Error()), slog.Group("detail", slog.String("route", form.ID)))
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	p.prepare(req, r, st, ss)
	if !p.acquireLegacy(r, st, form.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Do(req)
	st.Budget.Release()
	if err != nil {
		p.legacyFailure(w, r, st, form.ID, err)
		return
	}
	ss.keepAll = form.Session != nil && form.Session.Start != ""
	p.absorbSession(r, st, ss, resp)

	outcome := "failure"
	var sent int
	if succeeded(form.Success, resp, ss.hasCookie) {
		outcome = "success"
		if form.Session != nil {
			if form.Session.Start != "" {
				ss.login(form.Session.Start)
			} else if form.Session.End {
				ss.end()
			}
		}
		if target, ok := successRedirect(st.Model, form, params); ok {
			securityHeaders(w.Header())
			w.Header().Set("Location", target)
			w.WriteHeader(http.StatusSeeOther)
			sent = http.StatusSeeOther
		}
	}
	login := form.Session != nil && form.Session.Start != ""
	// The answer is already decided: a rule crossed now applies from the next request.
	p.observe(w, r, st, form.ID, limits.Observation{Params: params, FailedLogin: login && outcome == "failure"}, false)
	if sent == 0 {
		// The legacy answer is shown as is: its own confirmation or error page.
		sent = p.writeLegacy(w, r, st, form.ID, req.URL, resp)
	}
	ev := p.event(r, st, "request.served", "", sent, telemetry.Ptr(form.ID))
	total := float64(p.now().Sub(start).Microseconds()) / 1000
	legacyMS := float64(resp.Duration.Microseconds()) / 1000
	ev.LatencyMS = &telemetry.Latency{Gateway: max(total-legacyMS, 0), Legacy: legacyMS}
	ev.Detail = map[string]any{"outcome": outcome}
	p.Events.Emit(ev)
}

// crossSite reports a submission coming from another site. The token
// already prevents forgery; this is a second, independent barrier.
func crossSite(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host)
}

// readForm reads an urlencoded body. On failure it returns the status and
// the reason for the blocked-request event.
func readForm(w http.ResponseWriter, r *http.Request) (url.Values, int, string) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/x-www-form-urlencoded" {
		// multipart/form-data arrives with file uploads.
		return nil, http.StatusUnsupportedMediaType, "unsupported_media_type"
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxFormBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, http.StatusRequestEntityTooLarge, "body_too_large"
		}
		return nil, http.StatusBadRequest, "body_unreadable"
	}
	vals, err := url.ParseQuery(string(raw))
	if err != nil {
		return nil, http.StatusBadRequest, "malformed_body"
	}
	return vals, 0, ""
}

func (p *Public) openToken(r *http.Request, body url.Values) (formtoken.State, string) {
	tokens := body[formtoken.Field]
	if len(tokens) != 1 {
		return formtoken.State{}, "form_token_missing"
	}
	cookie := ""
	if c, err := r.Cookie(bindingCookie); err == nil {
		cookie = c.Value
	}
	if p.Forms == nil {
		return formtoken.State{}, "form_token_invalid"
	}
	state, err := p.Forms.Open(tokens[0], cookie)
	switch {
	case errors.Is(err, formtoken.ErrExpired):
		return state, "form_token_expired"
	case errors.Is(err, formtoken.ErrBinding):
		return state, "form_token_binding"
	case err != nil:
		return state, "form_token_invalid"
	}
	return state, ""
}

// sameParams compares the action's parameters with those sealed in the
// token, both in canonical form (e.g. "0042" and "42" are the same id).
func sameParams(st *site.Site, form *mapmodel.Form, params, sealed map[string]string) bool {
	canon, err := st.Validator.Fields(form.Params, single(sealed))
	if err != nil || len(canon) != len(params) {
		return false
	}
	for k, v := range params {
		if canon[k] != v {
			return false
		}
	}
	return true
}

// encodeValues re-encodes every value in the legacy page's charset.
func encodeValues(cs string, groups map[string]map[string]string) (placeholder.Values, error) {
	out := placeholder.Values{}
	for ns, vals := range groups {
		out[ns] = make(map[string]string, len(vals))
		for k, v := range vals {
			enc, err := charset.FromUTF8(cs, v)
			if err != nil {
				return nil, &validate.Error{Field: k, Reason: validate.ReasonCharset}
			}
			out[ns][k] = enc
		}
	}
	return out, nil
}

// succeeded applies the form's success check. Without a check, any answer
// below 400 counts as success. cookie checks that the session holds the
// legacy cookie (set now or before).
func succeeded(sc *mapmodel.SuccessCheck, resp *legacyclient.Response, hasCookie func(string) bool) bool {
	if sc == nil {
		return resp.Status < 400
	}
	if len(sc.Status) > 0 {
		found := false
		for _, s := range sc.Status {
			found = found || s == resp.Status
		}
		if !found {
			return false
		}
	} else if resp.Status >= 400 {
		return false
	}
	if sc.BodyContains != "" || sc.NotContains != "" {
		text := pageText(resp)
		if sc.BodyContains != "" && !bytes.Contains(text, []byte(sc.BodyContains)) {
			return false
		}
		if sc.NotContains != "" && bytes.Contains(text, []byte(sc.NotContains)) {
			return false
		}
	}
	if sc.Cookie != "" && !hasCookie(sc.Cookie) {
		return false
	}
	if sc.RedirectTo != "" {
		loc, err := url.Parse(resp.Location)
		if err != nil || resp.Status < 300 || resp.Status >= 400 || !strings.HasSuffix(loc.Path, sc.RedirectTo) {
			return false
		}
	}
	return true
}

// pageText returns the body in UTF-8, so checks written in the map match
// accented text whatever the legacy charset.
func pageText(resp *legacyclient.Response) []byte {
	cs, err := charset.Detect(resp.Header.Get("Content-Type"), resp.Body)
	if err != nil {
		return resp.Body
	}
	out, err := charset.ToUTF8(cs, resp.Body)
	if err != nil {
		return resp.Body
	}
	return out
}

// successRedirect builds the public URL of on_success.redirect, filling the
// route's path parameters with the form's parameters of the same name.
func successRedirect(m *mapmodel.Model, form *mapmodel.Form, params map[string]string) (string, bool) {
	if form.OnSuccess == nil || form.OnSuccess.Redirect == "" {
		return "", false
	}
	route, ok := m.RouteByID(form.OnSuccess.Redirect)
	if !ok {
		return "", false
	}
	for _, spec := range route.Query {
		if (spec.Required == nil || *spec.Required) && spec.Default == nil {
			return "", false // the gateway cannot know that value
		}
	}
	var b strings.Builder
	for _, seg := range strings.Split(strings.Trim(route.Path, "/"), "/") {
		if seg == "" {
			continue
		}
		b.WriteByte('/')
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			v := params[seg[1:len(seg)-1]]
			if v == "" {
				return "", false
			}
			b.WriteString(url.PathEscape(v))
			continue
		}
		b.WriteString(seg)
	}
	if b.Len() == 0 {
		return "/", true
	}
	return b.String(), true
}
