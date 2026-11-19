package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/session"
)

// Legacy cookies are of two kinds. Private ones (HttpOnly, or named like a
// session or a login) stay on the gateway: the browser never sees them and
// can never send one of that name. The others (consent banners, language,
// CSRF tokens the page's scripts read) work as on the legacy site: the
// browser keeps them, scripts read and write them, and they go back to the
// legacy application rebuilt.

const (
	maxBrowserCookies    = 50
	maxBrowserCookieSize = 4096
)

// Browser cookie values are rebuilt from a safe set: no quotes, spaces,
// semicolons or backslashes, nothing a cookie parser could misread.
var (
	cookieName  = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_|~-]{1,128}$`)
	cookieValue = regexp.MustCompile(`^[A-Za-z0-9!#$%&'()*+./:<=>?@\[\]^_{|}~,-]*$`)
)

// sessionLike reports whether a cookie name looks like a session or a
// login, whatever its flags say: old applications rarely set HttpOnly.
func sessionLike(name string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "csrf") || strings.Contains(n, "xsrf") {
		return false // read by the page's scripts on purpose
	}
	for _, s := range []string{"sess", "sid", "auth", "token", "login", "logged", "remember", "jwt", "user",
		"admin", "role", "priv", "perm", "level", "acl", "group", "debug",
		"uid", "pass", "pwd", "secret", "hash", "member", "account",
		"wordpress_", "wp_", "moodle", "joomla", "drupal", "phpbb", "asp.net", ".aspx", "jsession", "laravel", "ci_", "connect."} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

// gatewayCookie reports whether a browser cookie belongs to the gateway.
func gatewayCookie(name string) bool {
	return strings.HasPrefix(name, "__Host-eq") || name == bindingCookie || name == plainSessionCookie || name == plainBindingCookie
}

func persistent(c *http.Cookie, now time.Time) (time.Time, bool) {
	switch {
	case c.MaxAge > 0:
		return now.Add(time.Duration(c.MaxAge) * time.Second), true
	case !c.Expires.IsZero() && c.Expires.After(now):
		return c.Expires, true
	}
	return time.Time{}, false
}

// toBrowser rewrites a public legacy cookie for the public domain (Secure
// unless the site is served in http).
func toBrowser(c *http.Cookie, plain bool) *http.Cookie {
	out := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Expires: c.Expires, MaxAge: c.MaxAge,
		Secure: !plain, SameSite: c.SameSite}
	if !strings.HasPrefix(out.Path, "/") {
		out.Path = "/"
	}
	if out.SameSite == http.SameSiteDefaultMode || out.SameSite == http.SameSiteNoneMode {
		out.SameSite = http.SameSiteLaxMode
	}
	return out
}

// browserCookies adds to a legacy request the public cookies the browser
// holds (set by the legacy site or by its scripts), never private names.
func browserCookies(req *http.Request, r *http.Request, s *session.Session) {
	n := 0
	for _, c := range r.Cookies() {
		if n >= maxBrowserCookies || gatewayCookie(c.Name) || sessionLike(c.Name) || len(c.Value) > maxBrowserCookieSize ||
			!cookieName.MatchString(c.Name) || !cookieValue.MatchString(c.Value) {
			continue
		}
		if s != nil && (s.Private[c.Name] || s.Cookies[c.Name] != "") {
			continue
		}
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		n++
	}
}
