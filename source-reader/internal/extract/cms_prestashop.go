package extract

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// PrestaShop: a module's front controllers answer at
// index.php?fc=module&module=x&controller=y; the modules PrestaShop ships
// are in its composer.json, the rest are the site's. The administration
// folder has a name of its own: it is found by its content.

var (
	psModule  = regexp.MustCompile(`"prestashop/([a-z0-9_\-]{1,60})"`)
	psVersion = regexp.MustCompile(`const\s+VERSION\s*=\s*'([0-9][0-9A-Za-z.\-]{0,30})'`)
	psModVer  = regexp.MustCompile(`\$this->version\s*=\s*['"]([0-9A-Za-z.\-]{1,40})['"]`)
	psGuard   = regexp.MustCompile(`defined\(\s*['"]_PS_VERSION_['"]\s*\)`)
	psAuth    = regexp.MustCompile(`(?i)public\s+\$auth\s*=\s*true`)
)

type psCore struct {
	modules map[string]bool // shipped with PrestaShop
	admin   string          // the administration's folder
}

func loadPrestaShop(root, _ string) any {
	c := psCore{modules: map[string]bool{}}
	for _, m := range psModule.FindAllStringSubmatch(readCore(root, "composer.json"), -1) {
		c.modules[m[1]] = true
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, sign := range []string{"filemanager", "ajax-tab.php", "get-file-admin.php"} {
			if _, err := os.Stat(filepath.Join(root, e.Name(), sign)); err == nil && e.Name() != "js" {
				c.admin = e.Name()
			}
		}
	}
	return c
}

var prestashopKind = &kind{
	name:     "prestashop",
	markers:  [][]string{{"classes/ObjectModel.php", "config/defines.inc.php"}},
	versions: []string{"src/Core/Version.php", "app/AppKernel.php"},
	load:     loadPrestaShop,
	core: func(a *app, rel string) bool {
		c := a.data.(psCore)
		if !strings.Contains(rel, "/") || under(rel, "classes", "controllers", "src", "app", "config", "translations", "webservice",
			"install", "install-dev", "tools", "var", "mails", "pdf", "localization", "admin-api", "js", "img", "download", "upload") {
			return true
		}
		if c.admin != "" && under(rel, c.admin) {
			return true
		}
		return segment(rel, 0) == "modules" && c.modules[segment(rel, 1)]
	},
	owner: func(_ *app, rel string) (string, string) {
		switch segment(rel, 0) {
		case "modules":
			return "module", segment(rel, 1)
		case "themes":
			return "theme", segment(rel, 1)
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if rel == "src/Core/Version.php" || rel == "app/AppKernel.php" {
			if m := psVersion.FindStringSubmatch(src); m != nil {
				s.version("prestashop", m[1])
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		if segment(rel, 0) != "modules" || !isPHP(rel) {
			return
		}
		mod := segment(rel, 1)
		p := strings.Split(rel, "/")
		switch {
		case len(p) == 3 && p[2] == mod+".php":
			v := ""
			if m := psModVer.FindStringSubmatch(src); m != nil {
				v = m[1]
			}
			s.extension("plugin", "module", mod, v)
		case len(p) == 5 && p[2] == "controllers" && p[3] == "front":
			ctrl := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
			s.declared(Route{Path: a.path("index.php"), Methods: []string{"GET", "POST"}, Origin: "prestashop_controller",
				Query: map[string]string{"fc": "module", "module": mod, "controller": ctrl}, Public: !psAuth.MatchString(src),
				Where: s.at(1)})
		}
	},
	direct: func(_ *app, _, src string) bool { return !guarded(psGuard, src) && acts(src) },
}
