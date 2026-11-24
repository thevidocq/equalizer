package extract

import (
	"path"
	"regexp"
	"strings"
)

// Drupal 8 and later: routes in each module's *.routing.yml, the core in
// core/, modules and themes (contrib, custom) next to it. Drupal 7: routes
// in hook_menu, the core at the root.

var (
	drupalVersion  = regexp.MustCompile(`const VERSION = '([0-9][0-9A-Za-z.\-]{0,30})'`)
	drupal7Version = regexp.MustCompile(`define\(\s*'VERSION'\s*,\s*'([0-9][0-9A-Za-z.\-]{0,30})'`)
	infoVersion    = regexp.MustCompile(`(?m)^version\s*[:=]\s*['"]?([0-9A-Za-z.\-+]{1,40})['"]?\s*$`)
	menuItem       = regexp.MustCompile(`\$items\[\s*'([^'$]{1,200})'\s*\]\s*=`)
	menuPublic     = regexp.MustCompile(`'access callback'\s*=>\s*TRUE`)
)

// drupalOwner: the module, theme or profile of a file, wherever it sits
// (modules/contrib/x, sites/all/modules/x, profiles/p/modules/x).
func drupalOwner(_ *app, rel string) (string, string) {
	parts := strings.Split(rel, "/")
	for i := len(parts) - 2; i >= 0; i-- {
		switch parts[i] {
		case "modules", "themes":
			j := i + 1
			if j < len(parts)-1 && (parts[j] == "contrib" || parts[j] == "custom") {
				j++
			}
			if j < len(parts)-1 {
				if parts[i] == "themes" {
					return "theme", parts[j]
				}
				return "module", parts[j]
			}
		}
	}
	return "", ""
}

// drupalMeta: an extension's version from its info file.
func drupalMeta(s *Summary, a *app, rel, src string) {
	base := path.Base(rel)
	if !strings.HasSuffix(base, ".info.yml") && !strings.HasSuffix(base, ".info") {
		return
	}
	if a.core(a, rel) {
		return
	}
	kind, name := drupalOwner(a, rel)
	if name == "" || strings.TrimSuffix(strings.TrimSuffix(base, ".yml"), ".info") != name {
		return
	}
	v := ""
	if m := infoVersion.FindStringSubmatch(src); m != nil {
		v = m[1]
	}
	if kind == "theme" {
		s.extension("theme", "theme", name, v)
	} else {
		s.extension("plugin", "module", name, v)
	}
}

var drupalKind = &kind{
	name:         "drupal",
	markers:      [][]string{{"core/lib/Drupal.php"}},
	versions:     []string{"core/lib/Drupal.php"},
	corePackages: []string{"drupal/core", "drupal/core-recommended"},
	core: func(_ *app, rel string) bool {
		return under(rel, "core") || !strings.Contains(rel, "/") && isPHP(rel)
	},
	owner: drupalOwner,
	meta: func(s *Summary, a *app, rel, src string) {
		if rel == "core/lib/Drupal.php" {
			if m := drupalVersion.FindStringSubmatch(src); m != nil {
				s.version("drupal", m[1])
			}
		}
		drupalMeta(s, a, rel, src)
	},
	file: func(s *Summary, a *app, rel, src string) {
		if !strings.HasSuffix(rel, ".routing.yml") {
			return
		}
		for _, r := range symfonyRoutes(src) {
			methods := r.methods
			if len(methods) == 0 {
				methods = []string{"GET", "POST"} // a form posts to its own path
			}
			s.declared(Route{Path: a.path(r.path), Methods: methods, Origin: "drupal_route",
				Public: r.req["_access"] == "TRUE", Params: r.params(), Where: s.at(r.line)})
		}
	},
	// Drupal's web server rules refuse a PHP file in a folder: none is
	// opened directly
	direct: func(*app, string, string) bool { return false },
}

var drupal7Core = []string{"includes", "misc", "modules", "scripts", "themes", "profiles/minimal", "profiles/standard", "profiles/testing"}

var drupal7Kind = &kind{
	name:     "drupal",
	markers:  [][]string{{"includes/bootstrap.inc", "includes/menu.inc", "modules/system/system.module"}},
	versions: []string{"includes/bootstrap.inc"},
	core: func(_ *app, rel string) bool {
		return under(rel, drupal7Core...) || !strings.Contains(rel, "/") && isPHP(rel)
	},
	owner: drupalOwner,
	meta: func(s *Summary, a *app, rel, src string) {
		if rel == "includes/bootstrap.inc" {
			if m := drupal7Version.FindStringSubmatch(src); m != nil {
				s.version("drupal", m[1])
			}
		}
		drupalMeta(s, a, rel, src)
	},
	// hook_menu: $items['path/%'] = array(...)
	file: func(s *Summary, a *app, rel, src string) {
		if !strings.HasSuffix(rel, ".module") {
			return
		}
		lines := strings.Split(src, "\n")
		for i, line := range lines {
			m := menuItem.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			block := strings.Join(lines[i:min(len(lines), i+15)], "\n")
			if next := menuItem.FindStringIndex(block[len(m[0]):]); next != nil {
				block = block[:len(m[0])+next[0]]
			}
			p, params := drupal7Path(m[1])
			s.declared(Route{Path: a.path(p), Methods: []string{"GET", "POST"}, Origin: "drupal_route",
				Public: menuPublic.MatchString(block), Params: params, Where: s.at(i + 1)})
		}
	},
	direct: func(*app, string, string) bool { return false },
}

// drupal7Path turns node/%node/edit into /node/{node}/edit.
func drupal7Path(p string) (string, map[string]string) {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	params := map[string]string{}
	for i, part := range parts {
		if strings.HasPrefix(part, "%") {
			name := strings.TrimPrefix(part, "%")
			if name == "" {
				name = "arg" + string(rune('0'+i%10))
			}
			parts[i] = "{" + name + "}"
			params[name] = "unknown"
		}
	}
	if len(params) == 0 {
		params = nil
	}
	return "/" + strings.Join(parts, "/"), params
}
