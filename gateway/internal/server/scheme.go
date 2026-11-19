package server

import (
	"context"
	"net/http"

	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Sites are reached in https (default) or, as the legacy site was, in http
// (meta.public_scheme). The gateway listens on two ports behind Caddy: one
// for https traffic, one for plain http, so the scheme comes from the
// listener and never from a header.

type plainKey struct{}

// HandlerPlain serves the plain http port: https sites are redirected to
// https, http sites are served.
func (p *Public) HandlerPlain() http.Handler {
	h := p.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), plainKey{}, true)))
	})
}

func plainRequest(r *http.Request) bool {
	v, _ := r.Context().Value(plainKey{}).(bool)
	return v
}

func plainSite(st *site.Site) bool { return st.Model.Meta.Scheme() == "http" }

// toHTTPS sends a plain http request of an https site to https: 301 for
// GET and HEAD, 308 (method and body kept) for the others.
func toHTTPS(w http.ResponseWriter, r *http.Request, st *site.Site) {
	code := http.StatusMovedPermanently
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		code = http.StatusPermanentRedirect
	}
	http.Redirect(w, r, "https://"+st.Model.Meta.PublicDomain+safeReturn(r.URL.RequestURI()), code)
}

// Cookie names: in https the __Host- prefix binds the cookie to the exact
// host and to https; browsers refuse that prefix without Secure, so an http
// site gets plain names.
const (
	plainSessionCookie = "eq_session"
	plainBindingCookie = "eq_form"
)

func sessionCookieName(plain bool) string {
	if plain {
		return plainSessionCookie
	}
	return session.CookieName
}

func bindingCookieName(plain bool) string {
	if plain {
		return plainBindingCookie
	}
	return bindingCookie
}
