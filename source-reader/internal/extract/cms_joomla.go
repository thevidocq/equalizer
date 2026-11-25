package extract

import (
	"regexp"
	"strings"
)

// Joomla: a component answers at index.php?option=com_x (the site's,
// the administrator's), the web services at api/index.php/v1/... as their
// plugins declare them. The core's own extensions are listed in its
// ExtensionHelper; everything else is the site's.

var (
	joomlaCoreList = regexp.MustCompile(`\['(component|module|plugin|template|library|file|package|language)',\s*'([A-Za-z0-9_.\-]+)',\s*'([A-Za-z0-9_.\-]*)'`)
	joomlaVersion  = regexp.MustCompile(`(MAJOR_VERSION|MINOR_VERSION|PATCH_VERSION|RELEASE|DEV_LEVEL)\s*=\s*'?([0-9]{1,4}(?:\.[0-9]{1,4})?)'?`)
	joomlaGuard    = regexp.MustCompile(`defined\(\s*['"]_JEXEC['"]\s*\)`)
	joomlaCRUD     = regexp.MustCompile(`createCRUDRoutes\(\s*['"]([A-Za-z0-9_/:\-]{1,200})['"]\s*,\s*['"][^'"]*['"]\s*,\s*\[[^\]]*\](\s*,\s*(true|false))?`)
	joomlaRoute    = regexp.MustCompile(`new\s+Route\(\s*\[([^\]]*)\]\s*,\s*['"]([A-Za-z0-9_/:\-]{1,200})['"]`)
	joomlaPublic   = regexp.MustCompile(`['"]public['"]\s*=>\s*true`)
	joomlaInput    = regexp.MustCompile(`->(getInt|getUint|getCmd|getWord|getAlnum|getBool|getString|getUsername|getRaw|get)\(\s*['"]([A-Za-z0-9_\-]{1,100})['"]`)
	joomlaParam    = regexp.MustCompile(`:([A-Za-z0-9_]+)`)
)

// joomlaCore: the core's own extensions, from its ExtensionHelper, as
// "component:com_content", "plugin:system/cache"; a short list when the
// file is missing (Joomla 3 before 3.9).
type joomlaCore map[string]bool

var joomlaBasics = []string{"com_admin", "com_ajax", "com_banners", "com_cache", "com_categories", "com_checkin", "com_config",
	"com_contact", "com_content", "com_contenthistory", "com_cpanel", "com_fields", "com_finder", "com_installer", "com_joomlaupdate",
	"com_languages", "com_login", "com_media", "com_menus", "com_messages", "com_modules", "com_newsfeeds", "com_plugins",
	"com_postinstall", "com_privacy", "com_redirect", "com_search", "com_tags", "com_templates", "com_users", "com_wrapper",
	"com_actionlogs", "com_associations", "com_csp", "com_mails", "com_workflow", "com_scheduler", "com_guidedtours"}

func loadJoomla(root, _ string) any {
	core := joomlaCore{}
	for _, m := range joomlaCoreList.FindAllStringSubmatch(readCore(root, "libraries/src/Extension/ExtensionHelper.php"), -1) {
		if m[1] == "plugin" {
			core["plugin:"+m[3]+"/"+m[2]] = true
		} else {
			core[m[1]+":"+m[2]] = true
		}
	}
	if len(core) == 0 {
		for _, c := range joomlaBasics {
			core["component:"+c] = true
		}
	}
	return core
}

// joomlaExtension: the extension a file belongs to, as the core list
// names it, and how the panel shows it.
func joomlaExtension(rel string) (key, kind, name string) {
	p := strings.Split(rel, "/")
	if len(p) > 2 && (p[0] == "administrator" || p[0] == "api") {
		p = p[1:]
	}
	if len(p) < 3 {
		return "", "", ""
	}
	switch p[0] {
	case "components":
		return "component:" + p[1], "component", p[1]
	case "modules":
		return "module:" + p[1], "module", p[1]
	case "templates":
		return "template:" + p[1], "template", p[1]
	case "plugins":
		if len(p) > 3 {
			return "plugin:" + p[1] + "/" + p[2], "plugin", p[1] + "/" + p[2]
		}
	case "libraries":
		return "library:" + p[1], "library", p[1]
	}
	return "", "", ""
}

var joomlaKind = &kind{
	name:     "joomla",
	markers:  [][]string{{"libraries/src/Version.php", "administrator/index.php"}, {"libraries/cms/version/version.php", "administrator/index.php"}},
	versions: []string{"libraries/src/Version.php", "libraries/cms/version/version.php"},
	load:     loadJoomla,
	core: func(a *app, rel string) bool {
		if !strings.Contains(rel, "/") || under(rel, "includes", "layouts", "cli", "installation", "media", "language",
			"administrator/includes", "administrator/language", "api/includes", "libraries/src", "libraries/cms", "libraries/joomla", "libraries/legacy") {
			return true
		}
		key, _, _ := joomlaExtension(rel)
		return key != "" && a.data.(joomlaCore)[key] || strings.HasPrefix(key, "library:")
	},
	owner: func(_ *app, rel string) (string, string) {
		if _, kind, name := joomlaExtension(rel); kind != "" && kind != "library" {
			return kind, name
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel != "libraries/src/Version.php" && rel != "libraries/cms/version/version.php" {
			return
		}
		got := map[string]string{}
		for _, m := range joomlaVersion.FindAllStringSubmatch(src, -1) {
			got[m[1]] = m[2]
		}
		switch {
		case got["MAJOR_VERSION"] != "":
			s.version("joomla", got["MAJOR_VERSION"]+"."+got["MINOR_VERSION"]+"."+got["PATCH_VERSION"])
		case got["RELEASE"] != "":
			s.version("joomla", got["RELEASE"]+"."+got["DEV_LEVEL"])
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		_, kind, name := joomlaExtension(rel)
		if kind == "component" && isPHP(rel) {
			s.extension("plugin", "component", name, "")
			r := Route{Methods: []string{"GET", "POST"}, Origin: "joomla_component", Query: map[string]string{"option": name},
				Where: Where{File: s.file0, Line: 1, Owner: s.owner}}
			switch {
			case strings.HasPrefix(rel, "administrator/"):
				r.Path = a.path("administrator/index.php")
			case strings.HasPrefix(rel, "api/"):
				r.Path = a.path("api/index.php")
			default:
				r.Path, r.Public = a.path("index.php"), true
			}
			s.declared(r)
		} else if kind != "" {
			s.extension("plugin", kind, name, "")
		}
		if kind == "plugin" {
			joomlaAPI(s, a, src)
		}
		joomlaInputs(s, src)
	},
	direct: func(_ *app, _, src string) bool { return !guarded(joomlaGuard, src) && acts(src) },
}

// joomlaAPI: the web services a plugin adds to api/index.php.
func joomlaAPI(s *Summary, a *app, src string) {
	for _, m := range joomlaCRUD.FindAllStringSubmatchIndex(src, -1) {
		p := src[m[2]:m[3]]
		public := m[6] >= 0 && src[m[6]:m[7]] == "true"
		at := s.at(1 + strings.Count(src[:m[0]], "\n"))
		s.declared(Route{Path: a.path("api/index.php/" + strings.Trim(p, "/")), Methods: []string{"GET", "POST"},
			Origin: "joomla_api", Public: public, Where: at})
		s.declared(Route{Path: a.path("api/index.php/"+strings.Trim(p, "/")) + "/{id}", Methods: []string{"GET", "PATCH", "DELETE"},
			Origin: "joomla_api", Public: public, Params: map[string]string{"id": "int"}, Where: at})
	}
	for _, m := range joomlaRoute.FindAllStringSubmatchIndex(src, -1) {
		methods := methodList(src[m[2]:m[3]])
		if len(methods) == 0 {
			continue
		}
		p, params := colonPath(src[m[4]:m[5]])
		tail := src[m[1]:min(len(src), m[1]+400)]
		if end := strings.Index(tail, ");"); end >= 0 {
			tail = tail[:end]
		}
		s.declared(Route{Path: a.path("api/index.php" + p), Methods: methods, Origin: "joomla_api",
			Public: joomlaPublic.MatchString(tail), Params: params, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))})
	}
}

// colonPath turns v1/x/:id into /v1/x/{id}.
func colonPath(p string) (string, map[string]string) {
	params := map[string]string{}
	p = joomlaParam.ReplaceAllStringFunc(p, func(g string) string {
		params[g[1:]] = "unknown"
		return "{" + g[1:] + "}"
	})
	if len(params) == 0 {
		params = nil
	}
	return "/" + strings.Trim(p, "/"), params
}

// joomlaInputs: what Joomla's input filter makes of a request key.
func joomlaInputs(s *Summary, src string) {
	types := map[string]string{"getInt": "int", "getUint": "int", "getCmd": "key", "getWord": "key", "getAlnum": "key",
		"getBool": "bool", "getString": "text", "getUsername": "text", "getRaw": "unknown", "get": "unknown"}
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		if !strings.Contains(strings.ToLower(line), "input") {
			continue
		}
		for _, m := range joomlaInput.FindAllStringSubmatch(line, -1) {
			s.addInput(seen, Input{Name: m[2], Source: "request", Type: types[m[1]], Where: s.at(i + 1)})
		}
	}
}
