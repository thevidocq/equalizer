package server

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	areabody "github.com/thevidocq/equalizer/gateway/internal/rebuild"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// Areas serve whole sections of the site as they are: Equalizer stays in
// between (the legacy application is never reached directly and every
// request is rebuilt) without deciding what the visitor may do. The legacy
// application keeps its own access control.

// serveArea serves a request that no precise entry matched.
func (p *Public) serveArea(w http.ResponseWriter, r *http.Request, st *site.Site, a *mapmodel.Area) {
	start := p.now()
	ss := sessionOf(r)
	if len(a.Roles) > 0 && !site.RoleAllowed(ss.role(st), a.Roles) {
		p.block(w, r, st, http.StatusForbidden, "authz_denied")
		return
	}
	if !a.Allows(r.Method) {
		p.block(w, r, st, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if !cleanPath(r.URL.Path) {
		p.block(w, r, st, http.StatusBadRequest, "malformed_path")
		return
	}
	query, err := areabody.Query(r.URL.RawQuery, a.Charset)
	if err != nil {
		p.refuseArea(w, r, st, a, err)
		return
	}
	body, ctype, status, reason := p.areaBody(w, r, a)
	if status != 0 {
		p.block(w, r, st, status, reason)
		return
	}
	req, err := st.Legacy.BuildRaw(r.Context(), r.Method, r.URL.Path, query, body, ctype, areaHeaders(r, st))
	if err != nil {
		p.Log.ErrorContext(r.Context(), "cannot build the legacy request", slog.String("error", err.Error()),
			slog.Group("detail", slog.String("route", a.ID)))
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	ss.apply(req)
	if !p.acquireLegacy(r, st, a.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Open(req)
	if err != nil {
		st.Budget.Release()
		p.legacyFailure(w, r, st, a.ID, err)
		return
	}
	defer func() { resp.Close(); st.Budget.Release() }()
	ss.absorb(resp.Cookies)
	if ss.authenticated() && legacySessionExpired(st.Model.Legacy.SessionExpired, resp.Status, resp.Location, nil) {
		ss.expire()
	}
	if r.Method == http.MethodGet {
		p.Unmapped.Record(st.Model.Meta.App, r.Method, r.URL.Path, p.IPs.IP(r)) // coverage for discovery
	}
	sent := p.writeArea(w, r, st, a, req.URL, resp)
	ev := p.event(r, st, "request.served", "", sent, telemetry.Ptr(a.ID))
	total := float64(p.now().Sub(start).Microseconds()) / 1000
	ev.LatencyMS = &telemetry.Latency{Gateway: total}
	p.Events.Emit(ev)
}

// areaBody reads and rebuilds the request body within the area's limit.
func (p *Public) areaBody(w http.ResponseWriter, r *http.Request, a *mapmodel.Area) ([]byte, string, int, string) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return nil, "", 0, ""
	}
	limit := int64(a.MaxBodyKB) * 1024
	if limit == 0 {
		limit = mapmodel.DefaultAreaBodyKB * 1024
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, "", http.StatusRequestEntityTooLarge, "body_too_large"
		}
		return nil, "", http.StatusBadRequest, "malformed_body"
	}
	body, ctype, err := areabody.Body(r.Header.Get("Content-Type"), raw, a.Charset)
	if err != nil {
		var re *areabody.Error
		if errors.As(err, &re) {
			if re.Reason == areabody.ReasonUnsupported {
				return nil, "", http.StatusUnsupportedMediaType, re.Reason
			}
			return nil, "", http.StatusBadRequest, re.Reason
		}
		return nil, "", http.StatusBadRequest, "malformed_body"
	}
	return body, ctype, 0, ""
}

func (p *Public) refuseArea(w http.ResponseWriter, r *http.Request, st *site.Site, a *mapmodel.Area, err error) {
	reason := "malformed_query"
	var re *areabody.Error
	if errors.As(err, &re) {
		reason = re.Reason
	}
	p.block(w, r, st, http.StatusBadRequest, reason)
}

// cleanPath refuses what no browser sends: control characters, invalid
// UTF-8, dot segments, overlong paths.
func cleanPath(path string) bool {
	if len(path) > 4096 || !utf8.ValidString(path) || !strings.HasPrefix(path, "/") {
		return false
	}
	for _, c := range path {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

var (
	tokenValue = regexp.MustCompile(`^[\x20-\x7e]{1,1000}$`)
	rangeValue = regexp.MustCompile(`^bytes=\d*-\d*(,\s*\d*-\d*){0,9}$`)
)

// areaHeaders rebuilds the few request headers sites rely on; the browser's
// Referer and Origin become the legacy site's own.
func areaHeaders(r *http.Request, st *site.Site) http.Header {
	h := http.Header{}
	for _, k := range []string{"Accept", "Accept-Language"} {
		if v := r.Header.Get(k); tokenValue.MatchString(v) {
			h.Set(k, v)
		}
	}
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		h.Set("X-Requested-With", "XMLHttpRequest")
	}
	if v := r.Header.Get("Range"); rangeValue.MatchString(v) {
		h.Set("Range", v)
	}
	base := st.Legacy.Base()
	if ref, err := url.Parse(r.Header.Get("Referer")); err == nil && sameSite(ref, st) {
		h.Set("Referer", base.Scheme+"://"+base.Host+strings.TrimSuffix(base.Path, "/")+ref.EscapedPath())
	}
	if o, err := url.Parse(r.Header.Get("Origin")); err == nil && sameSite(o, st) {
		h.Set("Origin", base.Scheme+"://"+base.Host)
	}
	return h
}

func sameSite(u *url.URL, st *site.Site) bool {
	return u.Host != "" && strings.EqualFold(u.Hostname(), st.Model.Meta.PublicDomain)
}

// writeArea passes the legacy answer on: status as it is, pages rendered
// like any other page, other content streamed.
func (p *Public) writeArea(w http.ResponseWriter, r *http.Request, st *site.Site, a *mapmodel.Area, pageURL *url.URL, resp *legacyclient.Stream) int {
	h := w.Header()
	if resp.Status >= 300 && resp.Status < 400 && resp.Location != "" {
		loc, err := url.Parse(resp.Location)
		if err != nil || (loc.Scheme != "" && loc.Scheme != "http" && loc.Scheme != "https") {
			writePage(w, http.StatusBadGateway, pageBadGateway)
			return http.StatusBadGateway
		}
		loc = pageURL.ResolveReference(loc)
		target := loc.String() // another site: the legacy application's choice
		if st.URLs.Internal(loc) {
			var ok bool
			if target, ok = st.URLs.Public(loc); !ok {
				target = st.URLs.Fallback(loc)
			}
			(&issuer{URLMapper: st.URLs, st: st, ss: sessionOf(r)}).note(target)
		}
		securityHeaders(h)
		h.Set("Location", target)
		w.WriteHeader(resp.Status)
		return resp.Status
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	securityHeaders(h)
	if !pipeline.IsHTML(resp.Header.Get("Content-Type")) {
		w.WriteHeader(resp.Status)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, resp.Body)
		}
		return resp.Status
	}
	body, err := resp.ReadAll()
	if err != nil {
		h.Del("Content-Length")
		p.legacyFailure(w, r, st, a.ID, err)
		return http.StatusBadGateway
	}
	forms := p.newBinder(r, st.Forms)
	ctx := &pipeline.Context{PageURL: pageURL, URLs: p.issuing(st, sessionOf(r)), Forms: forms.pipelineBinder()}
	out, err := st.Pipeline.Render(body, resp.Header.Get("Content-Type"), ctx)
	if err != nil {
		p.Log.ErrorContext(r.Context(), "faithful rendering failed", slog.String("error", err.Error()),
			slog.Group("detail", slog.String("route", a.ID)))
		h.Del("Content-Length")
		writePage(w, http.StatusBadGateway, pageBadGateway)
		return http.StatusBadGateway
	}
	h.Del("Content-Length")
	h.Set("Content-Type", pipeline.ContentType)
	forms.setCookie(w)
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(out)
	}
	return resp.Status
}
