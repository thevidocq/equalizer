// Package pathkey gives the path under which a request is matched against
// the map: the way legacy servers read it, not the way the client wrote it.
// Servers disagree with a plain string comparison in a few known ways:
// Apache and nginx merge "//", Tomcat and Jetty drop ";parameters" from a
// segment, Windows drops trailing dots and spaces. Matching the raw path
// would let "//admin/" or "/admin;x/" skip an area that "/admin/" falls in.
package pathkey

import "strings"

// Of returns the matching key of a decoded path. The request still goes to
// the legacy application with its own path: only the map lookup uses this.
func Of(path string) string {
	segs := strings.Split(path, "/")
	out := make([]string, 0, len(segs))
	for i, s := range segs {
		s, _, _ = strings.Cut(s, ";")
		if t := strings.TrimRight(s, ". "); t != "" {
			s = t
		}
		if s == "" && i != 0 && i != len(segs)-1 {
			continue
		}
		out = append(out, s)
	}
	key := strings.Join(out, "/")
	if strings.HasSuffix(key, "//") { // "/a//" ends as "/a/"
		key = strings.TrimRight(key, "/") + "/"
	}
	return key
}
