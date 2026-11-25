package extract

import (
	"regexp"
	"strings"
)

// WordPress: REST routes, AJAX and admin-post actions, the core's version,
// plugins and themes from their headers, and which plugin files run when
// opened directly.

var (
	restCall    = regexp.MustCompile(`\bregister_rest_route\s*\(`)
	hookCall    = regexp.MustCompile(`\badd_action\s*\(`)
	ajaxHook    = regexp.MustCompile(`^(wp_ajax_nopriv_|wp_ajax_|admin_post_nopriv_|admin_post_)(.{0,100})$`)
	placeholder = regexp.MustCompile(`\{([A-Za-z_]\w*)\}`)
	restMethod  = regexp.MustCompile(`['"]methods['"]\s*=>\s*(?:['"]([A-Za-z, |]{1,40})['"]|(?:\\?WP_REST_Server::|)(READABLE|CREATABLE|EDITABLE|DELETABLE|ALLMETHODS))`)
	openPerm    = regexp.MustCompile(`['"]permission_callback['"]\s*=>\s*['"]__return_true['"]`)
	coreVer     = regexp.MustCompile(`\$wp_version\s*=\s*['"]([0-9][0-9A-Za-z.\-]{0,30})['"]`)
	header      = regexp.MustCompile(`(?m)^[ \t/*#@]*(Plugin Name|Theme Name|Version):\s*(.{1,120})$`)
	guard       = regexp.MustCompile(`(?i)defined\s*\(\s*['"](ABSPATH|WPINC)['"]\s*\)|if\s*\(\s*!\s*defined`)
	regexParam  = regexp.MustCompile(`\(\?P<([A-Za-z0-9_]{1,40})>([^)]*)\)`)
)

var wordpressKind = &kind{
	name:     "wordpress",
	markers:  [][]string{{"wp-includes/version.php"}},
	versions: []string{"wp-includes/version.php"},
	core: func(_ *app, rel string) bool {
		// a custom file next to them stays the site's own
		return under(rel, "wp-admin", "wp-includes") || rel == "index.php" || rel == "xmlrpc.php" ||
			!strings.Contains(rel, "/") && strings.HasPrefix(rel, "wp-")
	},
	owner: func(_ *app, rel string) (string, string) {
		if p := segment(rel, 2); segment(rel, 0) == "wp-content" && p != "" {
			switch segment(rel, 1) {
			case "plugins":
				return "plugin", p
			case "themes":
				return "theme", p
			}
		}
		return "", ""
	},
	meta: func(s *Summary, _ *app, rel, src string) { s.component(rel, src) },
	file: func(s *Summary, a *app, rel, src string) {
		if isPHP(rel) {
			s.wordpressFile(a, src)
		}
	},
	// a file outside the core without the ABSPATH guard: a plugin's, a
	// theme's or the site's own; the core's entry points are the CMS's, the
	// agent knows them
	direct: func(_ *app, _, src string) bool { return !guarded(guard, src) && acts(src) },
}

var restMethods = map[string][]string{
	"READABLE": {"GET"}, "CREATABLE": {"POST"}, "EDITABLE": {"POST", "PUT", "PATCH"},
	"DELETABLE": {"DELETE"}, "ALLMETHODS": {"GET", "POST", "PUT", "PATCH", "DELETE"},
}

// wordpressFile reads what WordPress declares in one file: REST routes and
// AJAX and admin-post actions, their names resolved from the file's own
// constants, properties and variables where they are not plain strings.
func (s *Summary) wordpressFile(a *app, src string) {
	lit := fileLiterals(src)
	lineAt := func(i int) Where { return s.at(1 + strings.Count(src[:i], "\n")) }
	calls := restCall.FindAllStringIndex(src, -1)
	for k, m := range calls {
		in := args(src, m[1], 3)
		if len(in) < 2 {
			continue
		}
		ns, okNS := lit.text(in[0])
		route, okRoute := lit.text(in[1])
		if !okNS && !okRoute {
			continue
		}
		// the route's options: up to the next route
		end := len(src)
		if k+1 < len(calls) {
			end = calls[k+1][0]
		}
		block := src[m[0]:min(end, m[0]+3000)]
		p, params := restPath(ns, route)
		for _, ph := range placeholder.FindAllStringSubmatch(p, -1) {
			if params == nil {
				params = map[string]string{}
			}
			if _, typed := params[ph[1]]; !typed {
				params[ph[1]] = "unknown"
			}
		}
		s.declared(Route{Path: a.path(p), Methods: methodsOf(block), Origin: "rest_route",
			Public: openPerm.MatchString(block), Params: params, Where: lineAt(m[0])})
	}
	for _, m := range hookCall.FindAllStringIndex(src, -1) {
		in := args(src, m[1], 1)
		if len(in) == 0 {
			continue
		}
		hook, ok := lit.text(in[0])
		if !ok {
			continue
		}
		hm := ajaxHook.FindStringSubmatch(hook)
		if hm == nil {
			continue
		}
		action := hm[2]
		if strings.Contains(action, "{") || action == "" {
			action = "*" // built at run time (a loop over a list): see the file
		}
		r := Route{Methods: []string{"GET", "POST"}, Action: action, Query: map[string]string{"action": action}, Where: lineAt(m[0])}
		switch hm[1] {
		case "wp_ajax_nopriv_":
			r.Path, r.Origin, r.Public = a.path("wp-admin/admin-ajax.php"), "wp_ajax_nopriv", true
		case "wp_ajax_":
			r.Path, r.Origin = a.path("wp-admin/admin-ajax.php"), "wp_ajax"
		case "admin_post_nopriv_":
			r.Path, r.Origin, r.Public = a.path("wp-admin/admin-post.php"), "admin_post", true
		default:
			r.Path, r.Origin = a.path("wp-admin/admin-post.php"), "admin_post"
		}
		s.declared(r)
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

// methodsOf: every method a route's endpoints declare (a route often has
// one endpoint to read and one to change), in a fixed order.
func methodsOf(block string) []string {
	seen := map[string]bool{}
	for _, m := range restMethod.FindAllStringSubmatch(block, -1) {
		if m[2] != "" {
			for _, v := range restMethods[m[2]] {
				seen[v] = true
			}
			continue
		}
		for _, v := range strings.FieldsFunc(strings.ToUpper(m[1]), func(r rune) bool { return r == ',' || r == '|' || r == ' ' }) {
			switch v {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
				seen[v] = true
			}
		}
	}
	var out []string
	for _, v := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if seen[v] {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"GET"} // WordPress's default
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
