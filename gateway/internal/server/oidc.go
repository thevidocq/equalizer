package server

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/oidc"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Login with the organisation's identity provider, for maps that turn it on
// (roles.auth.provider: oidc). The gateway's own paths live under /.eq/oidc/;
// the role comes from the user's groups, never from the browser.

const (
	oidcPrefix   = "/.eq/oidc/"
	oidcCallback = oidcPrefix + "callback"
	pendingTTL   = 10 * time.Minute
	maxPending   = 4 // logins started in other tabs stay valid
)

// Secrets gives the client secret of a site's identity provider ("" for a
// public client using PKCE alone).
type Secrets interface {
	OIDCSecret(app string) string
}

func redirectURI(st *site.Site) string {
	return "https://" + st.Model.Meta.PublicDomain + oidcCallback
}

// oidcGate answers the gateway's login paths and, when the login is
// required, sends visitors without an identity to the provider. It returns
// true when the request was answered.
func (p *Public) oidcGate(w http.ResponseWriter, r *http.Request, st *site.Site, ss *sessionState) bool {
	if st.OIDC == nil {
		return false
	}
	switch r.URL.Path {
	case oidcPrefix + "login":
		p.oidcStart(w, r, st, ss, safeReturn(r.URL.Query().Get("return")))
		return true
	case oidcCallback:
		p.oidcFinish(w, r, st, ss)
		return true
	case oidcPrefix + "logout":
		back := "https://" + st.Model.Meta.PublicDomain + "/"
		dest := st.OIDC.LogoutURL(r.Context(), st.Model.Roles.Auth.ClientID, back)
		ss.end()
		if dest == "" {
			dest = "/"
		}
		http.Redirect(w, r, dest, http.StatusFound)
		return true
	}
	if strings.HasPrefix(r.URL.Path, oidcPrefix) {
		p.block(w, r, st, http.StatusNotFound, "route_not_mapped")
		return true
	}
	if (ss.s != nil && ss.s.Identity != nil) || !loginNeeded(st.Model.Roles.Auth, r.URL.Path) {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead || !navigation(r) {
		p.Events.Emit(p.event(r, st, "request.blocked", "login_required", http.StatusUnauthorized, nil))
		writeAPIError(w, http.StatusUnauthorized, "login_required")
		return true
	}
	p.oidcStart(w, r, st, ss, safeReturn(r.URL.RequestURI()))
	return true
}

// navigation reports whether the browser is loading a page (a redirect to the
// provider makes sense) rather than calling from a script (401 instead).
func navigation(r *http.Request) bool {
	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}
	accept := r.Header.Get("Accept")
	return !strings.Contains(accept, "application/json") || strings.Contains(accept, "text/html")
}

// safeReturn keeps only a path of this site: never another host.
func safeReturn(s string) string {
	if !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/\\") ||
		strings.HasPrefix(s, oidcPrefix) || len(s) > 2000 || strings.ContainsAny(s, "\r\n") {
		return "/"
	}
	return s
}

func (p *Public) oidcStart(w http.ResponseWriter, r *http.Request, st *site.Site, ss *sessionState, back string) {
	if ss.broken {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	if ss.s == nil {
		ss.s = newSession(ss)
	}
	pend := &session.Pending{State: oidc.Random(), Nonce: oidc.Random(), Verifier: oidc.Random(), Return: back,
		Expires: p.now().Add(pendingTTL)}
	a := st.Model.Roles.Auth
	dest, err := st.OIDC.AuthURL(r.Context(), a.ClientID, redirectURI(st), pend.State, pend.Nonce, pend.Verifier, a.Scopes)
	if err != nil {
		p.Log.ErrorContext(r.Context(), "identity provider unavailable", slog.String("error", err.Error()))
		writePage(w, http.StatusBadGateway, pageBadGateway)
		return
	}
	var keep []session.Pending
	for _, old := range ss.s.Pending {
		if p.now().Before(old.Expires) {
			keep = append(keep, old)
		}
	}
	if len(keep) >= maxPending {
		keep = keep[len(keep)-maxPending+1:]
	}
	ss.s.Pending = append(keep, *pend)
	ss.dirty = true
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, dest, http.StatusFound)
}

func (p *Public) oidcFinish(w http.ResponseWriter, r *http.Request, st *site.Site, ss *sessionState) {
	q := r.URL.Query()
	refuse := func(reason string, status int, page string, err error) {
		ev := p.event(r, st, "request.blocked", reason, status, nil)
		p.Events.Emit(ev)
		if err != nil {
			p.Log.WarnContext(r.Context(), "OIDC login refused", slog.String("reason", reason), slog.String("error", err.Error()))
		}
		if ss.s != nil {
			ss.s.Pending = nil
			ss.dirty = true
		}
		writePage(w, status, page)
	}
	if ss.s == nil || len(ss.s.Pending) == 0 {
		refuse("oidc_no_pending_login", http.StatusBadRequest, pageBadRequest, nil)
		return
	}
	var pend session.Pending
	found := false
	for _, c := range ss.s.Pending {
		if q.Get("state") != "" && equal(q.Get("state"), c.State) && p.now().Before(c.Expires) {
			pend, found = c, true
		}
	}
	if !found || q.Get("error") != "" || q.Get("code") == "" {
		refuse("oidc_bad_callback", http.StatusBadRequest, pageBadRequest, nil)
		return
	}
	a := st.Model.Roles.Auth
	secret := ""
	if p.OIDCSecrets != nil {
		secret = p.OIDCSecrets.OIDCSecret(st.Model.Meta.App)
	}
	raw, err := st.OIDC.Exchange(r.Context(), a.ClientID, secret, q.Get("code"), redirectURI(st), pend.Verifier)
	if err != nil {
		refuse("oidc_exchange_failed", http.StatusBadGateway, pageBadGateway, err)
		return
	}
	claims, err := st.OIDC.Verify(r.Context(), raw, a.ClientID, pend.Nonce, a.GroupsClaim)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, oidc.ErrProvider) {
			status = http.StatusBadGateway
		}
		refuse("oidc_invalid_token", status, pageForbidden, err)
		return
	}
	role := oidc.RoleFor(st.OIDCRules(), claims.Groups)
	if role == "" {
		refuse("oidc_no_role", http.StatusForbidden, pageForbidden, nil)
		return
	}
	ss.login(ss.s.Role) // new session id: no fixation
	ss.s.Identity = &session.Identity{Subject: claims.Subject, Email: claims.Email, Role: role}
	var rest []session.Pending // logins of other tabs stay valid
	for _, c := range ss.s.Pending {
		if c.State != pend.State {
			rest = append(rest, c)
		}
	}
	ss.s.Pending = rest
	p.Log.InfoContext(r.Context(), "OIDC login", slog.Group("detail", slog.String("role", role)))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, (&url.URL{Path: "/"}).ResolveReference(mustParse(pend.Return)).RequestURI(), http.StatusFound)
}

func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		return &url.URL{Path: "/"}
	}
	return u
}
