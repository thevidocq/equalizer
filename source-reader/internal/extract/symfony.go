package extract

import (
	"regexp"
	"strings"
)

// Symfony-style routing files (Drupal's *.routing.yml, phpBB's
// routing.yml): a route per top-level key, with its path, methods and
// requirements. Read line by line: the files are flat and regular.

type ymlRoute struct {
	name, path string
	methods    []string
	req        map[string]string // requirements: _access, _permission, a parameter's pattern
	line       int
}

var (
	ymlKey   = regexp.MustCompile(`^([A-Za-z0-9_.\-]+):\s*(#.*)?$`)
	ymlField = regexp.MustCompile(`^\s+([A-Za-z0-9_]+):\s*(.*?)\s*$`)
	ymlParam = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)
)

func unquote(v string) string {
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.Trim(strings.TrimSpace(v), `'"`)
}

// symfonyRoutes reads the routes of a routing file.
func symfonyRoutes(src string) []ymlRoute {
	var out []ymlRoute
	var cur *ymlRoute
	section := ""
	for i, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if m := ymlKey.FindStringSubmatch(line); m != nil {
			out = append(out, ymlRoute{name: m[1], req: map[string]string{}, line: i + 1})
			cur, section = &out[len(out)-1], ""
			continue
		}
		if cur == nil {
			continue
		}
		m := ymlField.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		switch {
		case m[1] == "path" && indent <= 4 && section != "requirements" && section != "defaults":
			cur.path = unquote(m[2])
			section = ""
		case m[1] == "methods" && indent <= 4:
			cur.methods = methodList(m[2])
			section = ""
		case m[1] == "requirements" && m[2] == "", m[1] == "defaults" && m[2] == "", m[1] == "options" && m[2] == "":
			section = m[1]
		case m[1] == "requirements" && strings.HasPrefix(m[2], "{"):
			for _, kv := range strings.Split(strings.Trim(m[2], "{}"), ",") {
				if k, v, ok := strings.Cut(kv, ":"); ok {
					cur.req[strings.TrimSpace(k)] = unquote(v)
				}
			}
		case section == "requirements" && indent > 2:
			cur.req[m[1]] = unquote(m[2])
		case indent <= 2:
			section = m[1]
		}
	}
	kept := out[:0]
	for _, r := range out {
		if r.path != "" {
			kept = append(kept, r)
		}
	}
	return kept
}

func methodList(v string) []string {
	var out []string
	for _, m := range strings.FieldsFunc(strings.ToUpper(v), func(r rune) bool { return strings.ContainsRune("[], '\"|", r) }) {
		switch m {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			out = append(out, m)
		}
	}
	return out
}

// params types a route's parameters by their requirement.
func (r ymlRoute) params() map[string]string {
	out := map[string]string{}
	for _, m := range ymlParam.FindAllStringSubmatch(r.path, -1) {
		switch p := r.req[m[1]]; {
		case p == `\d+` || p == `[0-9]+` || p == `\d*`:
			out[m[1]] = "int"
		case p == "":
			out[m[1]] = "unknown"
		case strings.Trim(p, `[]a-zA-Z0-9_\-+`) == "":
			out[m[1]] = "slug"
		default:
			out[m[1]] = "unknown"
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
