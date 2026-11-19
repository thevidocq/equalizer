package server

import (
	"context"
	"errors"
	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/useragent"
)

// Legacy cookies kept per session: a bound against a legacy app that sets
// one cookie per page.
const (
	maxLegacyCookies    = 32
	maxLegacyCookieSize = 4096
	// A session seen less than this ago is not saved again just to extend it.
	touchEvery = time.Minute
)

// sessionState is the gateway session of one request. Changes are written
// to the store and to the browser once, just before the response headers
// (see commitWriter).
type sessionState struct {
	mgr    *session.Manager
	log    *slog.Logger
	policy session.Policy
	out    []*http.Cookie // public legacy cookies for the browser
	// keepAll: every cookie of this answer is private (a login form: its
	// cookies are the login, whatever their names).
	keepAll bool
	tenant  string
	app     string

	id     string           // current id; "" without a session
	s      *session.Session // nil without a session
	oldID  string           // id replaced at login, to delete
	dirty  bool
	ended  bool
	broken bool // the store is unavailable: no session changes in this request
	done   bool
	fresh  bool   // created in this request
	ipKey  string // the visitor's address, for the new-session limit
	plain  bool   // an http site: cookies without __Host- and Secure
}

type sessionKey struct{}

// loadSession reads the browser's session. Without a manager, or when the
// store fails, the visitor is anonymous.
func (p *Public) loadSession(r *http.Request, st *site.Site) *sessionState {
	ss := &sessionState{mgr: p.Sessions, log: p.Log, policy: st.SessionPolicy(),
		tenant: st.Model.Meta.Tenant, app: st.Model.Meta.App, plain: plainSite(st)}
	if p.IPs != nil {
		ss.ipKey = clientip.LimitKey(p.IPs.IP(r))
	}
	if p.Sessions == nil {
		ss.broken = true
		return ss
	}
	c, err := r.Cookie(sessionCookieName(ss.plain))
	if err != nil {
		return ss
	}
	s, err := p.Sessions.Load(r.Context(), c.Value, ss.tenant, ss.app, ss.policy)
	switch {
	case err == nil:
		ss.id, ss.s = c.Value, s
		if time.Since(s.LastSeen) > touchEvery {
			ss.dirty = true
		}
	case errors.Is(err, session.ErrNotFound):
		ss.oldID = c.Value // clear the stale cookie
		ss.dirty = true
	default:
		ss.broken = true
		p.Log.ErrorContext(r.Context(), "session store unavailable", slog.String("error", err.Error()))
	}
	return ss
}

func withSession(r *http.Request, ss *sessionState) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionKey{}, ss))
}

// sessionOf returns the request's session state (never nil).
func sessionOf(r *http.Request) *sessionState {
	if ss, ok := r.Context().Value(sessionKey{}).(*sessionState); ok {
		return ss
	}
	return &sessionState{broken: true}
}

// role returns the visitor's role: the one granted at login, or the map's
// anonymous role.
func (ss *sessionState) role(st *site.Site) string {
	if st.OIDC != nil {
		// The identity provider decides; the legacy login does not.
		if ss.s != nil && ss.s.Identity != nil {
			return ss.s.Identity.Role
		}
		return st.Model.Roles.Anonymous
	}
	if ss.s != nil && ss.s.Role != "" {
		return ss.s.Role
	}
	return st.Model.Roles.Anonymous
}

func (ss *sessionState) authenticated() bool { return ss.s != nil && ss.s.Role != "" }

// hash identifies the session in events.
func (ss *sessionState) hash() *string {
	if ss.id == "" {
		return nil
	}
	h := session.Hash(ss.id)
	return &h
}

// apply adds the legacy cookies of the session to a rebuilt request.
func (ss *sessionState) apply(req *http.Request) {
	if ss.s == nil || len(ss.s.Cookies) == 0 {
		return
	}
	req.Header.Del("Cookie")
	for name, value := range ss.s.Cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
}

// absorb keeps the private cookies the legacy application set, creating a
// session when needed (many legacy apps need theirs before login); public
// ones go to the browser (see cookies.go).
func (ss *sessionState) absorb(cookies []*http.Cookie) {
	now := time.Now()
	var remember time.Time // longest persistent login cookie of this answer
	forgot := false        // a login cookie was deleted
	for _, c := range cookies {
		if c.Name == "" || gatewayCookie(c.Name) {
			continue
		}
		if !ss.keepAll && !c.HttpOnly && !sessionLike(c.Name) && (ss.s == nil || !ss.s.Private[c.Name]) {
			ss.out = append(ss.out, toBrowser(c, ss.plain))
			continue
		}
		gone := c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(now)) || c.Value == ""
		if ss.broken || ss.ended || (gone && ss.s == nil) {
			continue // no session to keep it in, or nothing to delete
		}
		if ss.s == nil {
			ss.s = newSession(ss)
		}
		if ss.s.Cookies == nil {
			ss.s.Cookies = map[string]string{}
		}
		if ss.s.Private == nil {
			ss.s.Private = map[string]bool{}
		}
		if len(ss.s.Private) < maxLegacyCookies {
			ss.s.Private[c.Name] = true
		}
		ss.dirty = true
		if until, ok := persistent(c, now); ok && !gone && ss.policy.Remember > 0 {
			// "remember me": the session lives as long as the legacy login
			if until = minTime(until, now.Add(ss.policy.Remember)); until.After(remember) {
				remember = until
			}
		}
		if gone {
			if _, had := ss.s.Cookies[c.Name]; had {
				delete(ss.s.Cookies, c.Name)
				forgot = true
			}
			continue
		}
		if len(c.Value) > maxLegacyCookieSize {
			continue
		}
		if _, known := ss.s.Cookies[c.Name]; !known && len(ss.s.Cookies) >= maxLegacyCookies {
			continue
		}
		ss.s.Cookies[c.Name] = c.Value
	}
	switch {
	case ss.s == nil:
	case remember.After(ss.s.RememberUntil):
		ss.s.RememberUntil = remember // "remember me": as long as the legacy login
	case forgot && remember.IsZero():
		ss.s.RememberUntil = time.Time{} // the legacy login is being undone
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (ss *sessionState) hasCookie(name string) bool {
	return ss.s != nil && ss.s.Cookies[name] != ""
}

// login grants the role and changes the session id (no session fixation).
func (ss *sessionState) login(role string) {
	if ss.s == nil {
		ss.s = &session.Session{Tenant: ss.tenant, App: ss.app}
	}
	if ss.id != "" {
		ss.oldID = ss.id
	}
	ss.id = session.NewID()
	ss.s.Role = role
	ss.s.Created = time.Time{} // the absolute limit restarts at login
	ss.dirty = true
}

// newSession starts a session for this visitor.
func newSession(ss *sessionState) *session.Session {
	ss.id = session.NewID()
	ss.fresh = true
	return &session.Session{Tenant: ss.tenant, App: ss.app}
}

// expire drops the role, the legacy cookies and the issued values: the
// legacy session ended.
func (ss *sessionState) expire() {
	if ss.s == nil {
		return
	}
	ss.s.Role = ""
	ss.s.Cookies = nil
	ss.s.Issued = nil
	ss.s.RememberUntil = time.Time{}
	ss.dirty = true
}

// end removes the session (logout).
func (ss *sessionState) end() {
	if ss.id != "" {
		ss.oldID = ss.id
	}
	ss.id, ss.s, ss.ended, ss.dirty = "", nil, true, true
}

// commit writes the session to the store and to the browser.
func (ss *sessionState) commit(ctx context.Context, h http.Header) {
	if ss.done {
		return
	}
	ss.done = true
	for _, c := range ss.out {
		h.Add("Set-Cookie", c.String())
	}
	if len(ss.out) > 0 {
		h.Set("Cache-Control", "private, no-store") // a cookie is never shared
	}
	if ss.broken {
		return
	}
	if ss.s != nil {
		// Pages of a session never go to shared caches.
		h.Set("Cache-Control", "private, no-store")
	}
	if !ss.dirty {
		return
	}
	if ss.oldID != "" && ss.oldID != ss.id {
		if err := ss.mgr.Delete(ctx, ss.oldID); err != nil {
			ss.log.ErrorContext(ctx, "session not deleted", slog.String("error", err.Error()))
		}
	}
	cookie := &http.Cookie{Name: sessionCookieName(ss.plain), Path: "/", Secure: !ss.plain, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	if ss.s == nil {
		if ss.oldID != "" {
			cookie.MaxAge = -1
			h.Add("Set-Cookie", cookie.String())
		}
		return
	}
	policy := ss.policy
	if ss.fresh && !ss.authenticated() {
		if ok, first := anonSessions.check(ss.ipKey, time.Now()); !ok {
			// Reported once per address, not at every request of a flood.
			// The page is served without a session.
			if first {
				ss.log.WarnContext(ctx, "new sessions refused: too many from one address")
			}
			return
		}
		// Until the visitor comes back with the cookie (a bot never does),
		// the session lives a few minutes only; the next save after
		// touchEvery gives it the site's own idle time.
		policy.Idle = min(policy.Idle, freshIdle)
	}
	if err := ss.mgr.Save(ctx, ss.id, ss.s, policy); err != nil {
		ss.log.ErrorContext(ctx, "session not saved", slog.String("error", err.Error()))
		return
	}
	cookie.Value = ss.id
	if ss.s.Remembered(time.Now()) {
		cookie.Expires = ss.s.RememberUntil // kept when the browser closes
	}
	h.Add("Set-Cookie", cookie.String())
}

// commitWriter commits the session right before the first header is written.
type commitWriter struct {
	http.ResponseWriter
	ctx context.Context
	ss  *sessionState
}

func (c *commitWriter) WriteHeader(code int) {
	c.ss.commit(c.ctx, c.ResponseWriter.Header())
	c.ResponseWriter.WriteHeader(code)
}

func (c *commitWriter) Write(b []byte) (int, error) {
	c.ss.commit(c.ctx, c.ResponseWriter.Header())
	return c.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the server's writer.
func (c *commitWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// Flush sends what is buffered (live updates, see streamCopy).
func (c *commitWriter) Flush() {
	c.ss.commit(c.ctx, c.ResponseWriter.Header())
	_ = http.NewResponseController(c.ResponseWriter).Flush()
}

// legacySessionExpired reports whether the legacy answer says its own
// session ended (legacy.session_expired in the map).
func legacySessionExpired(se *mapmodel.SessionExpired, status int, location string, body []byte) bool {
	if se == nil {
		return false
	}
	if se.RedirectTo != "" && status >= 300 && status < 400 {
		path, _, _ := strings.Cut(location, "?")
		if strings.HasSuffix(path, se.RedirectTo) {
			return true
		}
	}
	for _, s := range se.Status {
		if s == status {
			return true
		}
	}
	return se.BodyContains != "" && strings.Contains(string(body), se.BodyContains)
}

// prepare readies a legacy request: the legacy cookies of the session and,
// unless the map says none, the visitor's address written by the gateway
// (a client's own X-Forwarded-For never gets this far).
func (p *Public) prepare(req *http.Request, r *http.Request, st *site.Site, ss *sessionState) {
	ss.apply(req)
	browserCookies(req, r, ss.s)
	if ua := useragent.Canonical(r.UserAgent()); ua != "" {
		req.Header.Set("User-Agent", ua) // the kind of browser, never its own string
	}
	// Behind the identity provider, the legacy site's own HTTP login (e.g.
	// WordPress application passwords) only for those who passed it.
	viaProvider := !st.Model.OIDC() || (ss.s != nil && ss.s.Identity != nil)
	if v := r.Header.Get("Authorization"); viaProvider && len(v) <= 8192 && authValue.MatchString(v) {
		req.Header.Set("Authorization", v) // the legacy site's own HTTP login
	}
	if st.Model.Legacy.ClientIP == "none" {
		return
	}
	if ip := p.IPs.IP(r); ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
		req.Header.Set("X-Real-IP", ip)
	}
}
