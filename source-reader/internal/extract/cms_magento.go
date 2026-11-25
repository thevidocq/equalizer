package extract

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Magento 2: a module's controllers answer at /frontName/folder/action,
// the frontName in its etc/frontend/routes.xml (etc/adminhtml for the
// administration); its web API at /rest/V1/... in etc/webapi.xml. The
// core is Magento's own modules (app/code/Magento, vendor/magento).

var (
	mageFront    = regexp.MustCompile(`frontName\s*=\s*"([A-Za-z0-9_\-]{1,60})"`)
	mageWebRoute = regexp.MustCompile(`<route\s+url\s*=\s*"([^"]{1,200})"\s+method\s*=\s*"([A-Za-z]{3,6})"`)
	mageAnon     = regexp.MustCompile(`<resource\s+ref\s*=\s*"anonymous"`)
)

// mageModule: app/code/Vendor/Module, the module folder of a file.
func mageModule(rel string) string {
	p := strings.Split(rel, "/")
	if len(p) > 4 && p[0] == "app" && p[1] == "code" {
		return strings.Join(p[:4], "/")
	}
	return ""
}

var magentoKind = &kind{
	name:         "magento",
	markers:      [][]string{{"app/bootstrap.php", "app/etc/NonComposerComponentRegistration.php"}},
	corePackages: []string{"magento/product-community-edition", "magento/product-enterprise-edition", "magento/magento2-base"},
	core: func(_ *app, rel string) bool {
		return !strings.Contains(rel, "/") || under(rel, "lib", "setup", "dev", "generated", "pub", "phpserver", "update", "bin",
			"app/code/Magento", "app/design/frontend/Magento", "app/design/adminhtml/Magento", "app/etc", "app/i18n")
	},
	owner: func(_ *app, rel string) (string, string) {
		if m := mageModule(rel); m != "" {
			p := strings.Split(m, "/")
			return "module", p[2] + "_" + p[3]
		}
		p := strings.Split(rel, "/")
		if len(p) > 5 && p[0] == "app" && p[1] == "design" {
			return "theme", p[3] + "/" + p[4]
		}
		return "", ""
	},
	file: func(s *Summary, a *app, rel, src string) {
		mod := mageModule(rel)
		if mod == "" {
			return
		}
		in := strings.TrimPrefix(rel, mod+"/")
		switch {
		case in == "composer.json":
			var c struct{ Version string }
			_ = json.Unmarshal([]byte(src), &c)
			p := strings.Split(mod, "/")
			s.extension("plugin", "module", p[2]+"_"+p[3], c.Version)
		case in == "etc/frontend/routes.xml" || in == "etc/adminhtml/routes.xml":
			if m := mageFront.FindStringSubmatch(src); m != nil {
				if strings.Contains(in, "adminhtml") {
					a.admin[mod] = m[1]
				} else {
					a.front[mod] = m[1]
				}
			}
		case in == "etc/webapi.xml":
			mageWebAPI(s, a, src)
		case strings.HasPrefix(in, "Controller/") && path.Ext(in) == ".php" && strings.Count(in, "/") >= 2:
			a.ctrl[mod] = append(a.ctrl[mod], strings.TrimSuffix(strings.TrimPrefix(in, "Controller/"), ".php"))
		}
	},
	// the web server serves pub/: no file of a module is opened directly
	direct: func(*app, string, string) bool { return false },
	finish: func(s *Summary, a *app) {
		mods := make([]string, 0, len(a.ctrl))
		for m := range a.ctrl {
			mods = append(mods, m)
		}
		sort.Strings(mods)
		for _, mod := range mods {
			for _, file := range a.ctrl[mod] {
				c := strings.ToLower(file)
				at := Where{File: a.base + mod + "/Controller/" + file + ".php", Line: 1}
				if s.app.owner != nil {
					k, n := s.app.owner(a, mod+"/x/y")
					at.Owner = k + " " + n
				}
				admin := strings.HasPrefix(c, "adminhtml/")
				switch {
				case admin && a.admin[mod] != "":
					s.declared(Route{Path: a.path("admin/" + a.admin[mod] + "/" + strings.TrimPrefix(c, "adminhtml/")),
						Methods: []string{"GET", "POST"}, Origin: "magento_route", Where: at})
				case !admin && a.front[mod] != "":
					s.declared(Route{Path: a.path(a.front[mod] + "/" + c), Methods: []string{"GET", "POST"}, Origin: "magento_route", Where: at})
				}
			}
		}
	},
}

// mageWebAPI: the REST routes of a module's webapi.xml; anonymous ones
// answer without a login.
func mageWebAPI(s *Summary, a *app, src string) {
	for _, m := range mageWebRoute.FindAllStringSubmatchIndex(src, -1) {
		block := src[m[1]:]
		if end := strings.Index(block, "</route>"); end >= 0 {
			block = block[:end]
		}
		method := methodList(src[m[4]:m[5]])
		if len(method) == 0 {
			continue
		}
		p, params := colonPath(src[m[2]:m[3]])
		s.declared(Route{Path: a.path("rest" + p), Methods: method, Origin: "magento_webapi", Public: mageAnon.MatchString(block),
			Params: params, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))})
	}
}
