package extract

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

// TYPO3: an extension's eID scripts answer at index.php?eID=name, its
// backend routes under /typo3/ (/typo3/ajax/ for the AJAX ones). The core
// is typo3/sysext (vendor/typo3 with Composer); extensions sit in
// typo3conf/ext or packages.

var (
	typo3Version = regexp.MustCompile(`VERSION\s*=\s*'([0-9][0-9A-Za-z.\-]{0,30})'`)
	typo3EID     = regexp.MustCompile(`\['eID_include'\]\s*\[\s*['"]([A-Za-z0-9_\-]{1,100})['"]\s*\]`)
	typo3Path    = regexp.MustCompile(`['"]path['"]\s*=>\s*['"](/[A-Za-z0-9_/{}\-.]{0,200})['"]`)
	typo3Methods = regexp.MustCompile(`['"]methods['"]\s*=>\s*\[([^\]]*)\]`)
	typo3Public  = regexp.MustCompile(`['"]access['"]\s*=>\s*['"]public['"]`)
	typo3Emconf  = regexp.MustCompile(`['"]version['"]\s*=>\s*['"]([0-9A-Za-z.\-]{1,40})['"]`)
	typo3Guard   = regexp.MustCompile(`defined\(\s*['"](TYPO3|TYPO3_MODE)['"]\s*\)`)
	typo3Param   = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)
)

// typo3Extension: the extension of a file, by its folder.
func typo3Extension(rel string) string {
	p := strings.Split(rel, "/")
	for i := 0; i+2 < len(p); i++ {
		if p[i] == "typo3conf" && p[i+1] == "ext" && i+3 < len(p) {
			return p[i+2]
		}
		if p[i] == "packages" && i == 0 {
			return p[1]
		}
	}
	return ""
}

var typo3Kind = &kind{
	name: "typo3",
	markers: [][]string{{"typo3/sysext/core/Classes/Information/Typo3Version.php"},
		{"vendor/typo3/cms-core/Classes/Information/Typo3Version.php"}, {"typo3/sysext/core/Classes/Core/Bootstrap.php"}},
	corePackages: []string{"typo3/cms-core"},
	versions:     []string{"typo3/sysext/core/Classes/Information/Typo3Version.php"},
	probe: func(root, base string) bool {
		return strings.Contains(readCore(root, base+"public/index.php"), `TYPO3\CMS\`)
	},
	core: func(_ *app, rel string) bool {
		return typo3Extension(rel) == "" && (under(rel, "typo3", "public/typo3") || !strings.Contains(rel, "/") ||
			strings.Count(rel, "/") == 1 && segment(rel, 0) == "public")
	},
	owner: func(_ *app, rel string) (string, string) {
		if e := typo3Extension(rel); e != "" {
			return "extension", e
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) {
		if strings.HasSuffix(rel, "sysext/core/Classes/Information/Typo3Version.php") {
			if m := typo3Version.FindStringSubmatch(src); m != nil {
				s.version("typo3", m[1])
			}
		}
	},
	file: func(s *Summary, a *app, rel, src string) {
		ext := typo3Extension(rel)
		if ext == "" {
			return
		}
		switch path.Base(rel) {
		case "ext_emconf.php":
			if m := typo3Emconf.FindStringSubmatch(src); m != nil {
				s.extension("plugin", "extension", ext, m[1])
			}
		case "composer.json":
			var c struct{ Version string }
			_ = json.Unmarshal([]byte(src), &c)
			s.extension("plugin", "extension", ext, c.Version)
		case "ext_localconf.php":
			for _, m := range typo3EID.FindAllStringSubmatchIndex(src, -1) {
				s.declared(Route{Path: a.path("index.php"), Methods: []string{"GET", "POST"}, Origin: "typo3_eid", Public: true,
					Query: map[string]string{"eID": src[m[2]:m[3]]}, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))})
			}
		case "Routes.php", "AjaxRoutes.php":
			if !strings.HasSuffix(path.Dir(rel), "Configuration/Backend") {
				return
			}
			prefix := "typo3"
			if path.Base(rel) == "AjaxRoutes.php" {
				prefix = "typo3/ajax"
			}
			typo3Backend(s, a, prefix, src)
		}
	},
	direct: func(_ *app, _, src string) bool { return !guarded(typo3Guard, src) && acts(src) },
}

// typo3Backend: the routes of a Routes.php or AjaxRoutes.php, each block
// from one 'path' to the next.
func typo3Backend(s *Summary, a *app, prefix, src string) {
	all := typo3Path.FindAllStringSubmatchIndex(src, -1)
	for i, m := range all {
		end := len(src)
		if i+1 < len(all) {
			end = all[i+1][0]
		}
		block := src[m[1]:end]
		methods := []string{"GET", "POST"}
		if mm := typo3Methods.FindStringSubmatch(block); mm != nil {
			if l := methodList(mm[1]); len(l) > 0 {
				methods = l
			}
		}
		p := src[m[2]:m[3]]
		params := map[string]string{}
		for _, pm := range typo3Param.FindAllStringSubmatch(p, -1) {
			params[pm[1]] = "unknown"
		}
		if len(params) == 0 {
			params = nil
		}
		s.declared(Route{Path: a.path(prefix + p), Methods: methods, Origin: "typo3_backend", Public: typo3Public.MatchString(block),
			Params: params, Where: s.at(1 + strings.Count(src[:m[0]], "\n"))})
	}
}
