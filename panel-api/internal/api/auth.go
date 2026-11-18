package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	keys := []string{"ip:" + clientIP(r), "email:" + email}
	if s.Throttle.Blocked(keys...) {
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
		s.Log.WarnContext(r.Context(), "panel login failed", slog.Group("detail", slog.String("ip", clientIP(r))))
		_ = s.Store.Record(r.Context(), nil, nil, "auth.login_failed", "", map[string]any{"ip": clientIP(r)})
		fail(w, http.StatusUnauthorized, "invalid_credentials", "")
		return
	}
	s.Throttle.Reset(keys...)
	// The session is complete only when no second step is due.
	u.MFAVerified = !u.MFAEnabled && !s.MFARequired
	id, err := s.Sessions.Create(r.Context(), u.ID, u.MFAVerified)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.setSessionCookie(w, id)
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.login", "", map[string]any{"ip": clientIP(r)})
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

// clientIP is the peer address: the panel is reached directly (VPN or SSH
// tunnel), never through a proxy.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
