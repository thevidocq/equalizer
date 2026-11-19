package server

import (
	"path"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// loginNeeded reports whether a request must pass the identity provider
// first: the whole site, or only the map's paths.
func loginNeeded(a *mapmodel.Auth, p string) bool {
	if a.LoginRequired() {
		return true
	}
	if len(a.Paths) == 0 {
		return false
	}
	c := loginPath(p)
	for _, prefix := range a.Paths {
		q := strings.ToLower(prefix)
		if strings.HasPrefix(c, q) || c+"/" == q {
			return true
		}
	}
	return false
}

// loginPath is the path as a legacy server could understand it, so that no
// spelling slips past a protected prefix: lower case (IIS), backslashes as
// slashes, ";params" and trailing dots or spaces of segments dropped, dot
// segments and double slashes resolved.
func loginPath(p string) string {
	p = strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if j := strings.IndexByte(s, ';'); j >= 0 {
			s = s[:j]
		}
		if s != "." && s != ".." {
			s = strings.TrimRight(s, ". ")
		}
		segs[i] = s
	}
	trailing := strings.HasSuffix(p, "/")
	c := path.Clean("/" + strings.Join(segs, "/"))
	if trailing && c != "/" {
		c += "/"
	}
	return c
}
