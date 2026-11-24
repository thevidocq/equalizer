package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Moodle: most pages are PHP files opened directly (they require
// config.php), reading their parameters with optional_param and
// required_param, whose PARAM_ type is the input's; AJAX services are
// declared in db/services.php. The plugins Moodle ships are listed in
// lib/plugins.json; a plugin folder not listed there is the site's.

var (
	moodleRelease = regexp.MustCompile(`\$release\s*=\s*'([0-9][0-9.]{0,20})`)
	moodlePlugRel = regexp.MustCompile(`\$plugin->release\s*=\s*['"]([0-9A-Za-z.\-+ ]{1,40})['"]`)
	moodleGuard   = regexp.MustCompile(`defined\(\s*['"]MOODLE_INTERNAL['"]\s*\)`)
	moodleConfig  = regexp.MustCompile(`(?m)^\s*require(_once)?\s*\(?[^;]*config\.php['"]\s*\)?\s*;`)
	moodleParam   = regexp.MustCompile(`(optional|required)_param(?:_array)?\(\s*['"]([A-Za-z0-9_\-]{1,100})['"]\s*,[^;]*?PARAM_([A-Z]+)`)
	moodleFunc    = regexp.MustCompile(`(?m)^\s*['"]([a-z][a-z0-9]*_[a-z0-9_]{1,100})['"]\s*=>\s*(?:array\s*\(|\[)\s*$`)
	moodleAjax    = regexp.MustCompile(`['"]ajax['"]\s*=>\s*true`)
	moodleNoLogin = regexp.MustCompile(`['"]loginrequired['"]\s*=>\s*false`)
	// before plugins.json: the list in plugin_manager::standard_plugins_list
	moodleOldList = regexp.MustCompile(`'([a-z]+)'\s*=>\s*array\(([^)]*)\)`)
	moodleQuoted  = regexp.MustCompile(`'([a-z0-9_]+)'`)
)

var moodleTypes = map[string]string{
	"INT": "int", "BOOL": "bool", "EMAIL": "email", "URL": "url", "LOCALURL": "url",
	"ALPHA": "key", "ALPHAEXT": "key", "ALPHANUM": "key", "ALPHANUMEXT": "key", "PLUGIN": "key", "COMPONENT": "key",
	"AREA": "key", "SAFEDIR": "key", "SAFEPATH": "unknown", "TEXT": "text", "NOTAGS": "text", "CLEANHTML": "html",
	"RAW": "unknown", "RAW_TRIMMED": "unknown", "SEQUENCE": "unknown", "FLOAT": "unknown", "LOCALISEDFLOAT": "unknown",
}

// moodleCore: the folders of the plugins that are not Moodle's own.
type moodleCore struct{ added []string }

func loadMoodle(root, site string) any {
	comps := readCore(root, "lib/components.json")
	prefix := ""
	if comps == "" && filepath.Clean(root) != filepath.Clean(site) {
		// Moodle 5.1: the web root is public/, the lists next to it (still
		// in the site's folder)
		comps, prefix = readCore(root, "../lib/components.json"), "public/"
	}
	var c struct {
		Types map[string]string `json:"plugintypes"`
	}
	_ = json.Unmarshal([]byte(comps), &c)
	standard := map[string]map[string]bool{}
	var p struct {
		Standard map[string][]string `json:"standard"`
	}
	if json.Unmarshal([]byte(readCore(root, filepath.FromSlash(strings.Repeat("../", strings.Count(prefix, "/"))+"lib/plugins.json"))), &p) == nil &&
		len(p.Standard) > 0 {
		for t, names := range p.Standard {
			standard[t] = map[string]bool{}
			for _, n := range names {
				standard[t][n] = true
			}
		}
	} else {
		src := readCore(root, "lib/classes/plugin_manager.php")
		if i := strings.Index(src, "standard_plugins"); i >= 0 {
			for _, m := range moodleOldList.FindAllStringSubmatch(src[i:], -1) {
				standard[m[1]] = map[string]bool{}
				for _, q := range moodleQuoted.FindAllStringSubmatch(m[2], -1) {
					standard[m[1]][q[1]] = true
				}
			}
		}
	}
	out := moodleCore{}
	if len(standard) == 0 {
		return out // nothing to tell the core from plugins by: all core
	}
	for t, dir := range c.Types {
		dir = strings.TrimPrefix(dir, prefix)
		entries, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		for _, e := range entries {
			if !e.IsDir() || standard[t][e.Name()] {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), e.Name(), "version.php")); err == nil {
				out.added = append(out.added, dir+"/"+e.Name()+"\x00"+t+"_"+e.Name())
			}
		}
	}
	return out
}

// moodlePlugin: the added plugin a file belongs to: its folder and its
// name (mod_attendance).
func moodlePlugin(a *app, rel string) (string, string) {
	for _, d := range a.data.(moodleCore).added {
		dir, name, _ := strings.Cut(d, "\x00")
		if strings.HasPrefix(rel, dir+"/") {
			return dir, name
		}
	}
	return "", ""
}

var moodleKind = &kind{
	name:     "moodle",
	markers:  [][]string{{"lib/moodlelib.php", "version.php"}},
	versions: []string{"version.php"},
	load:     loadMoodle,
	core: func(a *app, rel string) bool {
		_, name := moodlePlugin(a, rel)
		return name == ""
	},
	owner: func(a *app, rel string) (string, string) {
		if _, name := moodlePlugin(a, rel); name != "" {
			return "plugin", name
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel == "version.php" {
			if m := moodleRelease.FindStringSubmatch(src); m != nil {
				s.version("moodle", strings.TrimRight(m[1], "."))
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		dir, name := moodlePlugin(a, rel)
		switch strings.TrimPrefix(rel, dir+"/") {
		case "version.php":
			v := ""
			if m := moodlePlugRel.FindStringSubmatch(src); m != nil {
				v = m[1]
			}
			s.extension("plugin", "plugin", name, v)
		case "db/services.php":
			moodleServices(s, a, src)
		}
		if isPHP(rel) {
			route := ""
			if a.direct(a, rel, src) {
				route = a.path(rel)
			}
			moodleInputs(s, src, route)
		}
	},
	// a page: it loads Moodle's config.php itself, without the guard the
	// included files have
	direct: func(_ *app, _, src string) bool {
		return !guarded(moodleGuard, src) && (moodleConfig.MatchString(src) || acts(src))
	},
}

// moodleServices: the functions a plugin offers to Moodle's AJAX; those
// with loginrequired false answer without a login.
func moodleServices(s *Summary, a *app, src string) {
	all := moodleFunc.FindAllStringSubmatchIndex(src, -1)
	for i, m := range all {
		end := len(src)
		if i+1 < len(all) {
			end = all[i+1][0]
		}
		block := src[m[1]:end]
		if !moodleAjax.MatchString(block) {
			continue
		}
		r := Route{Path: a.path("lib/ajax/service.php"), Methods: []string{"POST"}, Origin: "moodle_service",
			Query: map[string]string{"info": src[m[2]:m[3]]}, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))}
		if moodleNoLogin.MatchString(block) {
			r.Path, r.Public = a.path("lib/ajax/service-nologin.php"), true
		}
		s.declared(r)
	}
}

// moodleInputs: optional_param('id', 0, PARAM_INT) is an int.
func moodleInputs(s *Summary, src, route string) {
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "_param") {
			continue
		}
		for _, m := range moodleParam.FindAllStringSubmatch(line, -1) {
			t, ok := moodleTypes[m[3]]
			if !ok {
				t = "unknown"
			}
			s.addInput(seen, Input{Name: m[2], Source: "request", Type: t, Route: route, Where: s.at(i + 1)})
		}
	}
}
