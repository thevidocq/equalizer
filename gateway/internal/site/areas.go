package site

import (
	"fmt"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/charset"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

var areaMethods = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true}

// checkAreas validates the sections of the site served as they are.
func checkAreas(m *mapmodel.Model) error {
	ids := map[string]bool{}
	prefixes := map[string]bool{}
	for _, a := range m.Areas {
		switch {
		case a.ID == "" || ids[a.ID]:
			return fmt.Errorf("area %q: missing or repeated id", a.ID)
		case !strings.HasPrefix(a.Prefix, "/") || !strings.HasSuffix(a.Prefix, "/") || strings.ContainsAny(a.Prefix, " ?#\\") ||
			strings.Contains(a.Prefix, "/../") || strings.Contains(a.Prefix, "/./"):
			return fmt.Errorf("area %q: prefix must be a path starting and ending with /", a.ID)
		case prefixes[a.Prefix]:
			return fmt.Errorf("area %q: prefix %q already used", a.ID, a.Prefix)
		case a.MaxBodyKB < 0 || a.MaxBodyKB > 1<<20:
			return fmt.Errorf("area %q: max_body_kb out of range", a.ID)
		}
		ids[a.ID], prefixes[a.Prefix] = true, true
		for _, method := range a.Methods {
			if !areaMethods[method] {
				return fmt.Errorf("area %q: method %q not supported", a.ID, method)
			}
		}
		for _, r := range a.Roles {
			if _, ok := m.Roles.List[r]; !ok {
				return fmt.Errorf("area %q: role %q is not in roles.list", a.ID, r)
			}
		}
		if a.Charset != "" {
			if _, err := charset.FromUTF8(a.Charset, "a"); err != nil {
				return fmt.Errorf("area %q: unknown charset %q", a.ID, a.Charset)
			}
		}
	}
	return nil
}
