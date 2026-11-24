package extract

import (
	"encoding/json"
	"regexp"
	"strings"
)

// OpenCart: every page is index.php?route=folder/file (and .method); an
// extension's catalog controllers answer there, its admin ones at
// admin/index.php. OpenCart 4 keeps extensions in extension/<name>/
// (extension/opencart is its own); OpenCart 3 in the controller folders'
// extension/ subfolder.

var (
	ocVersion = regexp.MustCompile(`define\(\s*'VERSION'\s*,\s*'([0-9][0-9A-Za-z. \-]{0,30})'`)
	ocMethod  = regexp.MustCompile(`\bpublic\s+function\s+([a-zA-Z][A-Za-z0-9_]{0,60})\s*\(`)
)

type ocCore struct{ sep string } // between a route and its method

func loadOpenCart(root, _ string) any {
	v := ""
	for _, f := range []string{"index.php", "admin/index.php", "action.php"} {
		if m := ocVersion.FindStringSubmatch(readCore(root, f)); m != nil {
			v = m[1]
			break
		}
	}
	switch {
	case strings.HasPrefix(v, "4.0.0") || strings.HasPrefix(v, "4.0.1"):
		return ocCore{sep: "|"}
	case strings.HasPrefix(v, "4") || strings.HasPrefix(v, "5"):
		return ocCore{sep: "."}
	}
	return ocCore{sep: "/"}
}

// ocExtension: OpenCart 4's extension/<name>/, OpenCart 3's
// <app>/controller/extension/<type>/<name>.php.
func ocExtension(rel string) string {
	p := strings.Split(rel, "/")
	switch {
	case len(p) > 2 && p[0] == "extension" && p[1] != "opencart":
		return p[1]
	case len(p) == 5 && (p[0] == "catalog" || p[0] == "admin") && p[2] == "extension":
		return strings.TrimSuffix(p[4], ".php")
	}
	return ""
}

var opencartKind = &kind{
	name:     "opencart",
	markers:  [][]string{{"system/startup.php", "catalog/controller"}},
	versions: []string{"index.php", "admin/index.php", "action.php"},
	load:     loadOpenCart,
	core:     func(_ *app, rel string) bool { return ocExtension(rel) == "" },
	owner: func(_ *app, rel string) (string, string) {
		if e := ocExtension(rel); e != "" {
			return "extension", e
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel == "index.php" || rel == "admin/index.php" || rel == "action.php" {
			if m := ocVersion.FindStringSubmatch(src); m != nil {
				s.version("opencart", strings.Fields(m[1])[0])
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		p := strings.Split(rel, "/")
		if p[0] == "extension" && len(p) == 3 && p[2] == "install.json" {
			var c struct{ Version string }
			_ = json.Unmarshal([]byte(src), &c)
			s.extension("plugin", "extension", p[1], c.Version)
			return
		}
		if !isPHP(rel) {
			return
		}
		route, admin := "", false
		switch {
		case len(p) > 4 && p[0] == "extension" && p[3] == "controller": // 4: extension/x/catalog/controller/module/y.php
			route, admin = "extension/"+p[1]+"/"+strings.TrimSuffix(strings.Join(p[4:], "/"), ".php"), p[2] == "admin"
		case len(p) == 5 && p[1] == "controller" && p[2] == "extension": // 3: catalog/controller/extension/module/y.php
			route, admin = strings.TrimSuffix(strings.Join(p[2:], "/"), ".php"), p[0] == "admin"
			s.extension("plugin", "extension", ocExtension(rel), "")
		default:
			return
		}
		entry := a.path("index.php")
		if admin {
			entry = a.path("admin/index.php")
		}
		sep := a.data.(ocCore).sep
		for _, m := range ocMethod.FindAllStringSubmatchIndex(src, -1) {
			r := route
			if name := src[m[2]:m[3]]; name != "index" {
				if strings.HasPrefix(name, "__") {
					continue
				}
				r += sep + name
			}
			s.declared(Route{Path: entry, Methods: []string{"GET", "POST"}, Origin: "opencart_route", Public: !admin,
				Query: map[string]string{"route": r}, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))})
		}
	},
}
