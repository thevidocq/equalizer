package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Failed logins allowed per window (Throttle.Window, 15 minutes).
const (
	loginPerPair    = 5
	loginPerAddress = 20
	loginPerAccount = 50
)

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	// Three counts: one address on one account (5), one address on any
	// account (20), any address on one account (50). Locking someone out
	// takes 50 attempts from many addresses, not 5 from anywhere.
	ip := s.clientIP(r)
	keys := []string{"pair:" + ip + "|" + email, "ip:" + ip, "email:" + email}
	if s.Throttle.Exceeds(keys[0], loginPerPair) || s.Throttle.Exceeds(keys[1], loginPerAddress) ||
		s.Throttle.Exceeds(keys[2], loginPerAccount) {
		fail(w, http.StatusTooManyRequests, "too_many_attempts", "try again later")
		return
	}
	u, hash, err := s.Store.Credentials(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		auth.CheckDummy(in.Password)
	case err != nil:
		s.storeError(w, r, err)
		return
	}
	if err != nil || !auth.CheckPassword(hash, in.Password) {
		s.Throttle.Fail(keys...)
		s.Log.WarnContext(r.Context(), "panel login failed", slog.Group("detail", slog.String("ip", s.clientIP(r))))
		_ = s.Store.Record(r.Context(), nil, nil, "auth.login_failed", "", map[string]any{"ip": s.clientIP(r)})
		fail(w, http.StatusUnauthorized, "invalid_credentials", "")
		return
	}
	s.Throttle.Reset(keys[0]) // only this address on this account: a login of one's own must not clear the others
	// The session is complete only when no second step is due.
	u.MFAVerified = !u.MFAEnabled && !s.MFARequired
	id, err := s.Sessions.Create(r.Context(), u.ID, u.MFAVerified)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.setSessionCookie(w, id)
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.login", "", map[string]any{"ip": s.clientIP(r)})
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		_ = s.Sessions.Delete(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.logout", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName, Value: id, Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(s.Sessions.TTL.Seconds())})
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request, u *auth.User) {
	writeJSON(w, http.StatusOK, u)
}

// updateMe changes the person's own preferences: the time zone the panel
// and the alert emails show times in.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var in struct {
		TimeZone string `json:"time_zone"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil || in.TimeZone == "" || in.TimeZone == "Local" {
		fail(w, http.StatusUnprocessableEntity, "invalid", "time_zone: a time zone name such as Europe/Rome or UTC")
		return
	}
	if err := s.Store.SetTimeZone(r.Context(), u.ID, in.TimeZone); err != nil {
		s.storeError(w, r, err)
		return
	}
	u.TimeZone = in.TimeZone
	writeJSON(w, http.StatusOK, u)
}

// clientIP is the visitor's address: the peer, or behind the trusted
// proxies (Caddy, panel-ui) the last address in X-Forwarded-For that is
// not one of them. Without that, every visitor would share the proxy's
// address, and five wrong passwords would lock everyone out.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !s.trusted(peer.Unmap()) {
		return host
	}
	parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			break
		}
		if a = a.Unmap(); !s.trusted(a) {
			return a.String()
		}
	}
	return host
}

func (s *Server) trusted(a netip.Addr) bool {
	for _, p := range s.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
