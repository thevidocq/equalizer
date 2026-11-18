package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
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
	tenant string
	app    string

	id     string           // current id; "" without a session
	s      *session.Session // nil without a session
	oldID  string           // id replaced at login, to delete
	dirty  bool
	ended  bool
	broken bool // the store is unavailable: no session changes in this request
	done   bool
}

type sessionKey struct{}

// loadSession reads the browser's session. Without a manager, or when the
// store fails, the visitor is anonymous.
func (p *Public) loadSession(r *http.Request, st *site.Site) *sessionState {
	ss := &sessionState{mgr: p.Sessions, log: p.Log, policy: st.SessionPolicy(),
		tenant: st.Model.Meta.Tenant, app: st.Model.Meta.App}
	if p.Sessions == nil {
		ss.broken = true
		return ss
	}
	c, err := r.Cookie(session.CookieName)
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

// absorb keeps the cookies the legacy application set, creating a session
// when needed (many legacy apps need theirs before login).
func (ss *sessionState) absorb(cookies []*http.Cookie) {
	if len(cookies) == 0 || ss.broken || ss.ended {
		return
	}
	if ss.s == nil {
		ss.s = newSession(ss)
	}
	if ss.s.Cookies == nil {
		ss.s.Cookies = map[string]string{}
	}
	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		if c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(time.Now())) || c.Value == "" {
			delete(ss.s.Cookies, c.Name)
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
	ss.dirty = true
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
	if ss.done || ss.broken {
		return
	}
	ss.done = true
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
	cookie := &http.Cookie{Name: session.CookieName, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	if ss.s == nil {
		if ss.oldID != "" {
			cookie.MaxAge = -1
			h.Add("Set-Cookie", cookie.String())
		}
		return
	}
	if err := ss.mgr.Save(ctx, ss.id, ss.s, ss.policy); err != nil {
		ss.log.ErrorContext(ctx, "session not saved", slog.String("error", err.Error()))
		return
	}
	cookie.Value = ss.id
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
