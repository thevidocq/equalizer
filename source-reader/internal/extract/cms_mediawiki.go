package extract

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// MediaWiki: an extension declares, in its extension.json (or skin.json),
// special pages (index.php?title=Special:Name), API modules
// (api.php?action=name, list=, prop=, meta=) and REST routes (rest.php/...).
// Older extensions set the same in PHP ($wgSpecialPages['Name'] = ...).

var (
	mwVersion  = regexp.MustCompile(`(?:MW_VERSION'\s*,|\$wgVersion\s*=)\s*'([0-9][0-9A-Za-z.\-]{0,30})'`)
	mwLegacy   = regexp.MustCompile(`\$wg(SpecialPages|APIModules)\[\s*['"]([A-Za-z0-9_\-]{1,100})['"]\s*\]\s*=`)
	mwGuard    = regexp.MustCompile(`defined\(\s*['"]MEDIAWIKI['"]\s*\)`)
	mwRestPart = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)
)

// mwExtension: extensions/X or skins/X.
func mwExtension(rel string) (string, string) {
	switch segment(rel, 0) {
	case "extensions":
		return "extension", segment(rel, 1)
	case "skins":
		return "skin", segment(rel, 1)
	}
	return "", ""
}

var mediawikiKind = &kind{
	name:     "mediawiki",
	markers:  [][]string{{"includes/Defines.php", "api.php"}, {"includes/DefaultSettings.php", "api.php"}},
	versions: []string{"includes/Defines.php", "includes/DefaultSettings.php"},
	core: func(_ *app, rel string) bool {
		_, name := mwExtension(rel)
		return name == ""
	},
	owner: func(_ *app, rel string) (string, string) { return mwExtension(rel) },
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel == "includes/Defines.php" || rel == "includes/DefaultSettings.php" {
			if m := mwVersion.FindStringSubmatch(src); m != nil {
				s.version("mediawiki", m[1])
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		kind, name := mwExtension(rel)
		if name == "" {
			return
		}
		base := path.Base(rel)
		if (base == "extension.json" || base == "skin.json") && strings.Count(rel, "/") == 2 {
			mwManifest(s, a, kind, name, src)
			return
		}
		if isPHP(rel) {
			for _, m := range mwLegacy.FindAllStringSubmatchIndex(src, -1) {
				r := mwEntry(a, src[m[2]:m[3]], src[m[4]:m[5]])
				r.Where = s.at(1 + strings.Count(src[:m[0]], "\n"))
				s.declared(r)
			}
		}
	},
	direct: func(_ *app, _, src string) bool { return !guarded(mwGuard, src) && acts(src) },
}

// mwEntry is a special page or an API module.
func mwEntry(a *app, what, name string) Route {
	if what == "SpecialPages" {
		return Route{Path: a.path("index.php"), Methods: []string{"GET", "POST"}, Origin: "mediawiki_special",
			Query: map[string]string{"title": "Special:" + name}}
	}
	return Route{Path: a.path("api.php"), Methods: []string{"GET", "POST"}, Origin: "mediawiki_api",
		Query: map[string]string{"action": name}}
}

func mwManifest(s *Summary, a *app, kind, name, src string) {
	var m struct {
		Version        string                     `json:"version"`
		SpecialPages   map[string]json.RawMessage `json:"SpecialPages"`
		APIModules     map[string]json.RawMessage `json:"APIModules"`
		APIListModules map[string]json.RawMessage `json:"APIListModules"`
		APIPropModules map[string]json.RawMessage `json:"APIPropModules"`
		APIMetaModules map[string]json.RawMessage `json:"APIMetaModules"`
		RestRoutes     []mwRest                   `json:"RestRoutes"`
	}
	if json.Unmarshal([]byte(src), &m) != nil {
		return
	}
	if kind == "skin" {
		s.extension("theme", "skin", name, m.Version)
	} else {
		s.extension("plugin", "extension", name, m.Version)
	}
	lineOf := func(key string) int {
		if i := strings.Index(src, `"`+key+`"`); i >= 0 {
			return 1 + strings.Count(src[:i], "\n")
		}
		return 1
	}
	add := func(r Route, key string) {
		r.Where = s.at(lineOf(key))
		s.declared(r)
	}
	for _, n := range jsonKeys(m.SpecialPages) {
		add(mwEntry(a, "SpecialPages", n), n)
	}
	for _, n := range jsonKeys(m.APIModules) {
		add(mwEntry(a, "APIModules", n), n)
	}
	for q, mods := range map[string]map[string]json.RawMessage{"list": m.APIListModules, "prop": m.APIPropModules, "meta": m.APIMetaModules} {
		for _, n := range jsonKeys(mods) {
			add(Route{Path: a.path("api.php"), Methods: []string{"GET", "POST"}, Origin: "mediawiki_api",
				Query: map[string]string{"action": "query", q: n}}, n)
		}
	}
	for _, r := range m.RestRoutes {
		if !strings.HasPrefix(r.Path, "/") || len(r.Path) > 200 {
			continue
		}
		methods := methodList(strings.Join(r.methods(), ","))
		if len(methods) == 0 {
			methods = []string{"GET"}
		}
		params := map[string]string{}
		for _, p := range mwRestPart.FindAllStringSubmatch(r.Path, -1) {
			params[p[1]] = "unknown"
		}
		if len(params) == 0 {
			params = nil
		}
		add(Route{Path: a.path("rest.php" + r.Path), Methods: methods, Origin: "mediawiki_rest", Params: params}, r.Path)
	}
}

// mwRest: a REST route; its method is a string or a list.
type mwRest struct {
	Path   string          `json:"path"`
	Method json.RawMessage `json:"method"`
}

func (r mwRest) methods() []string {
	var one string
	if json.Unmarshal(r.Method, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(r.Method, &many)
	return many
}

func jsonKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if len(k) <= 100 && k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
