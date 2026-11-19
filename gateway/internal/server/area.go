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
	"github.com/thevidocq/equalizer/gateway/internal/origin"
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
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		// Frames would reach the legacy application as the client wrote
		// them: not served until they can be rebuilt one by one.
		p.block(w, r, st, http.StatusNotImplemented, "websocket_not_supported")
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
	cs := a.Charset
	if cs == "" {
		cs = st.LegacyCharset() // learnt from the site's own pages
	}
	query, err := areabody.Query(r.URL.RawQuery, cs)
	if err != nil {
		p.refuseArea(w, r, st, a, err)
		return
	}
	query = string(st.Inbound.Bytes([]byte(query)))
	body, status, reason := p.areaBody(w, r, st, a, cs)
	if status != 0 {
		p.block(w, r, st, status, reason)
		return
	}
	defer body.close()
	req, err := st.Legacy.BuildRaw(r.Context(), r.Method, r.URL.Path, query, body.r, body.size, body.ctype, areaHeaders(r, st))
	if err != nil {
		p.Log.ErrorContext(r.Context(), "cannot build the legacy request", slog.String("error", err.Error()),
			slog.Group("detail", slog.String("route", a.ID)))
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	p.prepare(req, r, st, ss)
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
	authValue  = regexp.MustCompile(`^(Basic|Bearer|Digest|Token) [\x20-\x7e]+$`)
	appName    = regexp.MustCompile(`^X-[A-Za-z0-9][A-Za-z0-9-]{0,62}$`)
	etagValue  = regexp.MustCompile(`^(\*|(W/)?"[\x21\x23-\x7e]*"(\s*,\s*(W/)?"[\x21\x23-\x7e]*")*)$`)
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
	n := 0
	for k, vs := range r.Header {
		if n >= 30 || !requestAppHeader(k) || len(vs) != 1 || len(vs[0]) > 4096 || !tokenValue.MatchString(vs[0]) {
			continue
		}
		h.Set(k, vs[0]) // the application's own: X-WP-Nonce, X-CSRF-Token, X-XSRF-TOKEN, …
		n++
	}
	if d := rawDisposition(r); d != "" {
		h.Set("Content-Disposition", d) // the name of a file sent as the whole body
	}
	if v := r.Header.Get("Range"); rangeValue.MatchString(v) {
		h.Set("Range", v)
	}
	// Browser caches of files (pages carry no validators: they are rendered
	// per visitor, see writeArea).
	if v := r.Header.Get("If-None-Match"); len(v) <= 1000 && etagValue.MatchString(v) {
		h.Set("If-None-Match", v)
	}
	if v := r.Header.Get("If-Modified-Since"); v != "" {
		if t, err := http.ParseTime(v); err == nil {
			h.Set("If-Modified-Since", t.UTC().Format(http.TimeFormat))
		}
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

// requestAppHeader reports whether a request header is the application's
// own (X-…): never those a proxy or the gateway sets, nor those some
// frameworks read as the client's address or host.
func requestAppHeader(k string) bool {
	k = http.CanonicalHeaderKey(k)
	if !appName.MatchString(k) {
		return false
	}
	for _, p := range blockedPrefixes {
		if strings.HasPrefix(k, p) {
			return false
		}
	}
	// identity set by a single sign-on proxy (X-Remote-User, X-Auth-User…)
	return !strings.HasPrefix(k, "X-User") && !strings.Contains(k, "-User")
}

// blockedPrefixes are request headers some servers and frameworks trust as
// set by a proxy: client address, host, rewritten URL, method override,
// identity, internal sub-requests.
var blockedPrefixes = []string{
	"X-Forwarded", "X-Eq-", "X-Original", "X-Real-Ip", "X-Client-Ip", "X-Host", "X-Rewrite-Url",
	"X-Cluster-Client-Ip", "X-Proxy", "X-Remote-", "X-Originating-", "X-Custom-Ip", "X-True-Client",
	"X-Http-Method", "X-Method-Override", "X-Middleware-Subrequest", "X-Ms-Client-Principal",
	"X-Server-", "X-Envoy-", "X-Amzn-", "X-Backend", "X-Internal",
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
		if eventStream(resp.Header.Get("Content-Type")) {
			h.Set("X-Accel-Buffering", "no") // proxies in front must not buffer
		}
		if st.Origin != nil && origin.Text(resp.Header.Get("Content-Type")) {
			h.Del("Content-Length") // the legacy origin may be rewritten
			h.Del("ETag")
		}
		w.WriteHeader(resp.Status)
		if r.Method != http.MethodHead {
			streamCopy(w, resp.Body, resp.Header.Get("Content-Type"), st.Origin)
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
	st.NoteCharset(ctx.Stats.Charset)
	h.Del("Content-Length")
	h.Del("ETag") // the page is rendered per visitor (form tokens): never revalidated
	h.Del("Last-Modified")
	h.Set("Content-Type", pipeline.ContentType)
	forms.setCookie(w)
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(st.Origin.Bytes(out))
	}
	return resp.Status
}

// streamCopy copies a legacy body to the browser. Live updates
// (text/event-stream) are flushed event by event instead of buffered.
// Text bodies have the legacy origin replaced by the public one.
func streamCopy(w http.ResponseWriter, body io.Reader, ctype string, o *origin.Rewriter) {
	if !eventStream(ctype) {
		if o != nil && origin.Text(ctype) {
			ow := o.Writer(w)
			_, _ = io.Copy(ow, body)
			_ = ow.Close()
			return
		}
		_, _ = io.Copy(w, body)
		return
	}
	rc := http.NewResponseController(w)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			_ = rc.Flush()
		}
		if err != nil {
			return
		}
	}
}

func eventStream(ctype string) bool {
	mt, _, _ := strings.Cut(ctype, ";")
	return strings.EqualFold(strings.TrimSpace(mt), "text/event-stream")
}
