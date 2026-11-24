package extract

import (
	"path"
	"regexp"
	"strings"
)

// WordPress: REST routes, AJAX and admin-post actions, the core's version,
// plugins and themes from their headers, and which plugin files run when
// opened directly.

var (
	restRoute  = regexp.MustCompile(`register_rest_route\s*\(\s*['"]([^'"]{1,100})['"]\s*,\s*['"]([^'"]{1,200})['"]`)
	restMethod = regexp.MustCompile(`['"]methods['"]\s*=>\s*(?:['"]([A-Za-z, |]{1,40})['"]|(?:\\?WP_REST_Server::|)(READABLE|CREATABLE|EDITABLE|DELETABLE|ALLMETHODS))`)
	openPerm   = regexp.MustCompile(`['"]permission_callback['"]\s*=>\s*['"]__return_true['"]`)
	ajax       = regexp.MustCompile(`add_action\s*\(\s*['"](wp_ajax_nopriv_|wp_ajax_|admin_post_nopriv_|admin_post_)([A-Za-z0-9_\-]{1,100})['"]`)
	coreVer    = regexp.MustCompile(`\$wp_version\s*=\s*['"]([0-9][0-9A-Za-z.\-]{0,30})['"]`)
	header     = regexp.MustCompile(`(?m)^[ \t/*#@]*(Plugin Name|Theme Name|Version):\s*(.{1,120})$`)
	guard      = regexp.MustCompile(`(?i)defined\s*\(\s*['"](ABSPATH|WPINC)['"]\s*\)|if\s*\(\s*!\s*defined`)
	regexParam = regexp.MustCompile(`\(\?P<([A-Za-z0-9_]{1,40})>([^)]*)\)`)
)

var restMethods = map[string][]string{
	"READABLE": {"GET"}, "CREATABLE": {"POST"}, "EDITABLE": {"POST", "PUT", "PATCH"},
	"DELETABLE": {"DELETE"}, "ALLMETHODS": {"GET", "POST", "PUT", "PATCH", "DELETE"},
}

// wordpressFile reads what WordPress declares in one file.
func (s *Summary) wordpressFile(rel, src string) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		at := Where{File: rel, Line: i + 1}
		if m := restRoute.FindStringSubmatch(line); m != nil && len(s.Routes) < maxRoutes {
			// the route's options follow: up to 30 lines
			block := strings.Join(lines[i:min(len(lines), i+30)], "\n")
			if end := strings.Index(block[len(m[0]):], "register_rest_route"); end >= 0 {
				block = block[:len(m[0])+end]
			}
			p, params := restPath(m[1], m[2])
			s.Routes = append(s.Routes, Route{Path: p, Methods: methodsOf(block), Origin: "rest_route",
				Public: openPerm.MatchString(block), Params: params, Where: at})
		}
		for _, m := range ajax.FindAllStringSubmatch(line, -1) {
			if len(s.Routes) >= maxRoutes {
				break
			}
			r := Route{Methods: []string{"GET", "POST"}, Action: m[2], Where: at}
			switch m[1] {
			case "wp_ajax_nopriv_":
				r.Path, r.Origin, r.Public = "/wp-admin/admin-ajax.php", "wp_ajax_nopriv", true
			case "wp_ajax_":
				r.Path, r.Origin = "/wp-admin/admin-ajax.php", "wp_ajax"
			case "admin_post_nopriv_":
				r.Path, r.Origin, r.Public = "/wp-admin/admin-post.php", "admin_post", true
			default:
				r.Path, r.Origin = "/wp-admin/admin-post.php", "admin_post"
			}
			s.Routes = append(s.Routes, r)
		}
	}
}

// restPath turns a REST namespace and route into a map path: a named
// group becomes {name}, typed by its pattern.
func restPath(ns, route string) (string, map[string]string) {
	params := map[string]string{}
	route = regexParam.ReplaceAllStringFunc(route, func(g string) string {
		m := regexParam.FindStringSubmatch(g)
		switch {
		case strings.Contains(m[2], `\d`) && !strings.ContainsAny(strings.ReplaceAll(m[2], `\d`, ""), `wW.`):
			params[m[1]] = "int"
		case strings.Contains(m[2], `\w`) || strings.Contains(m[2], "a-z"):
			params[m[1]] = "slug"
		default:
			params[m[1]] = "unknown"
		}
		return "{" + m[1] + "}"
	})
	route = strings.TrimSuffix(strings.TrimPrefix(route, "^"), "$")
	p := "/wp-json/" + strings.Trim(ns, "/") + "/" + strings.TrimLeft(route, "/")
	if len(params) == 0 {
		params = nil
	}
	return strings.TrimRight(p, "/"), params
}

func methodsOf(block string) []string {
	m := restMethod.FindStringSubmatch(block)
	if m == nil {
		return []string{"GET"} // WordPress's default
	}
	if m[2] != "" {
		return restMethods[m[2]]
	}
	var out []string
	for _, v := range strings.FieldsFunc(strings.ToUpper(m[1]), func(r rune) bool { return r == ',' || r == '|' || r == ' ' }) {
		switch v {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"GET"}
	}
	return out
}

// component reads a plugin's main file, a theme's style.css or the core's
// version.php.
func (s *Summary) component(rel, src string) {
	if rel == "wp-includes/version.php" {
		if m := coreVer.FindStringSubmatch(src); m != nil {
			v := m[1]
			s.addComponent(Component{Name: "wordpress", Kind: "core", Version: &v})
		}
		return
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 3 || parts[0] != "wp-content" {
		return
	}
	head := src[:min(len(src), 8192)]
	name, version := "", ""
	for _, m := range header.FindAllStringSubmatch(head, -1) {
		switch m[1] {
		case "Plugin Name", "Theme Name":
			name = strings.TrimSpace(m[2])
		case "Version":
			version = strings.TrimSpace(m[2])
		}
	}
	if name == "" {
		return
	}
	kind := "plugin"
	if parts[1] == "themes" {
		kind = "theme"
	}
	c := Component{Name: kind + ":" + parts[2], Kind: kind}
	if version != "" {
		c.Version = &version
	}
	s.addComponent(c)
}

func (s *Summary) addComponent(c Component) {
	for _, have := range s.Components {
		if have.Name == c.Name {
			return
		}
	}
	if len(s.Components) < maxComponents {
		s.Components = append(s.Components, c)
	}
}

// directRoute is the path of a PHP file a browser can open and that runs
// by itself: any file of a plain PHP site; in WordPress, a file of a plugin
// or theme without the ABSPATH guard (the core's own entry points are
// the CMS's, the agent knows them).
func directRoute(stack, rel, src string) string {
	if stack == "wordpress" {
		if !strings.HasPrefix(rel, "wp-content/plugins/") && !strings.HasPrefix(rel, "wp-content/themes/") {
			return ""
		}
		if guard.MatchString(src[:min(len(src), 4096)]) {
			return ""
		}
	}
	if strings.ToLower(path.Ext(rel)) != ".php" {
		return ""
	}
	return "/" + rel
}
