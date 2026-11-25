package extract

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

// phpBB: an extension (ext/vendor/name) declares its pages in
// config/routing.yml; they answer under app.php. Everything outside ext/
// is phpBB's own, as is ext/phpbb/viglink.

var (
	phpbbVersion = regexp.MustCompile(`PHPBB_VERSION'\s*,\s*'([0-9][0-9A-Za-z.\-]{0,30})'`)
	phpbbGuard   = regexp.MustCompile(`defined\(\s*['"]IN_PHPBB['"]\s*\)`)
)

// phpbbExtension: vendor/name of ext/vendor/name/...; phpBB ships one
// extension of its own (ext/phpbb/viglink): the phpBB team's others
// (boardrules, pages…) are installed like anyone's.
func phpbbExtension(rel string) string {
	p := strings.Split(rel, "/")
	if len(p) > 3 && p[0] == "ext" && !(p[1] == "phpbb" && p[2] == "viglink") {
		return p[1] + "/" + p[2]
	}
	return ""
}

var phpbbKind = &kind{
	name:     "phpbb",
	markers:  [][]string{{"includes/constants.php", "viewtopic.php"}},
	versions: []string{"includes/constants.php"},
	core:     func(_ *app, rel string) bool { return phpbbExtension(rel) == "" },
	owner: func(_ *app, rel string) (string, string) {
		if e := phpbbExtension(rel); e != "" {
			return "extension", e
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel == "includes/constants.php" {
			if m := phpbbVersion.FindStringSubmatch(src); m != nil {
				s.version("phpbb", m[1])
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		ext := phpbbExtension(rel)
		in := strings.Join(strings.Split(rel, "/")[3:], "/")
		switch {
		case in == "composer.json":
			var c struct{ Version string }
			_ = json.Unmarshal([]byte(src), &c)
			s.extension("plugin", "extension", ext, c.Version)
		case path.Base(rel) == "routing.yml" && strings.HasPrefix(in, "config/"):
			for _, r := range symfonyRoutes(src) {
				methods := r.methods
				if len(methods) == 0 {
					methods = []string{"GET", "POST"}
				}
				s.declared(Route{Path: a.path("app.php/" + strings.TrimLeft(r.path, "/")), Methods: methods, Origin: "phpbb_route",
					Params: r.params(), Where: s.at(r.line)})
			}
		}
	},
	direct: func(_ *app, _, src string) bool { return !guarded(phpbbGuard, src) && acts(src) },
}
