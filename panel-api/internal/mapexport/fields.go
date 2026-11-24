// Package mapexport writes a map version for other tools: an OpenAPI 3
// description of what the map accepts, and ModSecurity rules (ModSecurity
// v3, Coraza) that refuse what it does not hold. Both are translations: a
// WAF checks a request and forwards it, the gateway rebuilds it, so each
// file says in its comments what it keeps and what it loses.
package mapexport

import (
	"sort"
	"strings"
)

// field is a field's check with its named type and the map's defaults
// applied, as the gateway resolves it (gateway/internal/validate).
type field struct {
	name      string
	base      string
	required  bool
	min, max  *float64
	values    []string
	deny      []string
	pattern   string
	charset   string
	maxLength int
	rules     []string
	mime      []string
	maxSizeKB int
}

const fallbackMaxLength = 1000

type doc map[string]any

func (d doc) list(key string) []map[string]any {
	var out []map[string]any
	for _, x := range asList(d[key]) {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func strs(v any) []string {
	var out []string
	for _, x := range asList(v) {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func isBase(t string) bool {
	switch t {
	case "int", "enum", "bool", "date", "string", "email", "text", "password", "file":
		return true
	}
	return false
}

// fields resolves a field map, sorted by name.
func (d doc) fields(m map[string]any) []field {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]field, 0, len(names))
	for _, n := range names {
		out = append(out, d.resolve(n, asMap(m[n])))
	}
	return out
}

func (d doc) resolve(name string, s map[string]any) field {
	f := field{name: name, base: str(s["type"])}
	if !isBase(f.base) {
		t := asMap(asMap(d["types"])[f.base])
		f = field{name: name, base: str(t["base"]), min: num(t["min"]), max: num(t["max"]), values: strs(t["values"]),
			deny: strs(t["deny"]), pattern: str(t["pattern"]), charset: str(t["charset"]), rules: strs(t["rules"])}
		if n := num(t["max_length"]); n != nil {
			f.maxLength = int(*n)
		}
	}
	req, ok := s["required"].(bool)
	f.required = !ok || req
	if v := num(s["min"]); v != nil {
		f.min = v
	}
	if v := num(s["max"]); v != nil {
		f.max = v
	}
	if v := strs(s["values"]); len(v) > 0 {
		f.values = v
	}
	f.deny = append(f.deny, strs(s["deny"])...)
	if v := str(s["pattern"]); v != "" {
		f.pattern = v
	}
	if v := str(s["charset"]); v != "" {
		f.charset = v
	}
	if v := num(s["max_length"]); v != nil {
		f.maxLength = int(*v)
	}
	f.rules = append(f.rules, strs(s["rules"])...)
	f.mime = strs(s["mime"])
	if v := num(s["max_size_kb"]); v != nil {
		f.maxSizeKB = int(*v)
	}
	defaults := asMap(asMap(d["defaults"])["string"])
	if f.base == "string" {
		if f.charset == "" && f.pattern == "" {
			f.charset = str(defaults["charset"])
		}
		if f.maxLength == 0 {
			if v := num(defaults["max_length"]); v != nil {
				f.maxLength = int(*v)
			}
		}
	}
	switch {
	case f.maxLength > 0:
	case f.base == "email":
		f.maxLength = 254
	case f.base == "password":
		f.maxLength = 128
	default:
		f.maxLength = fallbackMaxLength
	}
	return f
}

// pathParams are the {name} of a public path, in order.
func pathParams(p string) []string {
	var out []string
	for {
		i := strings.IndexByte(p, '{')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(p[i:], '}')
		if j < 0 {
			return out
		}
		out = append(out, p[i+1:i+j])
		p = p[i+j+1:]
	}
}
