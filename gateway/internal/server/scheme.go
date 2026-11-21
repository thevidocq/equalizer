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
		h.ServeHTTP(&noHSTS{ResponseWriter: w}, r.WithContext(context.WithValue(r.Context(), plainKey{}, true)))
	})
}

// noHSTS drops Strict-Transport-Security from plain http answers: it means
// nothing there, and a site served in http must stay reachable in http.
type noHSTS struct{ http.ResponseWriter }

func (n *noHSTS) WriteHeader(code int) {
	n.ResponseWriter.Header().Del("Strict-Transport-Security")
	n.ResponseWriter.WriteHeader(code)
}

func (n *noHSTS) Write(b []byte) (int, error) {
	n.ResponseWriter.Header().Del("Strict-Transport-Security")
	return n.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach Flush and the deadlines.
func (n *noHSTS) Unwrap() http.ResponseWriter { return n.ResponseWriter }

func plainRequest(r *http.Request) bool {
	v, _ := r.Context().Value(plainKey{}).(bool)
	return v
}

// plainSite: visitors use plain http (cookies without __Host- and Secure).
func plainSite(st *site.Site) bool { return st.Model.Meta.Scheme() == "http" }

// plainPort: served on the plain port, in clear or behind a proxy that ends
// https in front of Equalizer (visitors still use https: Secure cookies).
func plainPort(st *site.Site) bool { return plainSite(st) || st.Model.Meta.HTTPSAtProxy() }

// toHTTPS sends a plain http request of an https site to https: 302 for
// GET and HEAD, 307 (method and body kept) for the others. Temporary, not
// permanent: the scheme is a setting of the site, and a browser that cached
// a 301 would keep going to https after the site is switched to http.
func toHTTPS(w http.ResponseWriter, r *http.Request, st *site.Site) {
	code := http.StatusFound
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		code = http.StatusTemporaryRedirect
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
