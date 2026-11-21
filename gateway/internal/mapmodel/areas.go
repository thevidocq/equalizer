package mapmodel

import (
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/pathkey"
)

// AreaFor returns the area with the longest prefix of path, nil if none.
// path is matched as legacy servers read it (pathkey).
func (m *Model) AreaFor(path string) *Area {
	return m.areaFor(pathkey.Of(path), strings.HasPrefix)
}

// AreaAmbiguous reports whether path falls in another area when letters
// are compared without case, as Windows servers do: "/Admin/" must not
// escape the roles of "/admin/". Such a request is refused.
func (m *Model) AreaAmbiguous(path string) bool {
	key := pathkey.Of(path)
	return m.areaFor(key, strings.HasPrefix) != m.areaFor(key, hasPrefixFold)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func (m *Model) areaFor(path string, has func(s, prefix string) bool) *Area {
	var best *Area
	for i := range m.Areas {
		a := &m.Areas[i]
		if has(path, a.Prefix) || has(path+"/", a.Prefix) && len(path)+1 == len(a.Prefix) {
			if best == nil || len(a.Prefix) > len(best.Prefix) {
				best = a
			}
		}
	}
	return best
}
