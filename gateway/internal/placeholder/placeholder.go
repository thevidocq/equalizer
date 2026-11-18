// Package placeholder resolves the {{namespace.name}} placeholders of
// legacy_request.
//
// Allowed namespaces: params (path), query, fields (form), hidden (legacy
// hidden fields kept server-side), account (legacy account of the role),
// user (authenticated user). Values arrive already
// validated: this package only substitutes them.
package placeholder

import (
	"fmt"
	"strings"
)

// Values holds values by namespace: Values["params"]["id"].
type Values map[string]map[string]string

var namespaces = map[string]bool{"params": true, "query": true, "fields": true, "hidden": true, "account": true, "user": true}

// Resolve substitutes the placeholders in tpl.
//
// When tpl is a single placeholder and its value is missing, it returns
// absent=true: the request builder omits that parameter. In a mixed
// template a missing value is an error, so partial values are never built.
func Resolve(tpl string, vals Values) (out string, absent bool, err error) {
	if !strings.Contains(tpl, "{{") {
		if strings.Contains(tpl, "}}") {
			return "", false, fmt.Errorf("malformed placeholder in %q", tpl)
		}
		return tpl, false, nil
	}
	var b strings.Builder
	rest := tpl
	single := strings.HasPrefix(tpl, "{{") && strings.HasSuffix(tpl, "}}") && strings.Count(tpl, "{{") == 1
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			if strings.Contains(rest, "}}") {
				return "", false, fmt.Errorf("malformed placeholder in %q", tpl)
			}
			b.WriteString(rest)
			break
		}
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			return "", false, fmt.Errorf("unclosed placeholder in %q", tpl)
		}
		b.WriteString(rest[:start])
		key := strings.TrimSpace(rest[start+2 : start+end])
		ns, name, ok := strings.Cut(key, ".")
		if !ok || !namespaces[ns] || name == "" {
			return "", false, fmt.Errorf("invalid placeholder %q", key)
		}
		v, found := vals[ns][name]
		if !found {
			if single {
				return "", true, nil
			}
			return "", false, fmt.Errorf("%w %q", errMissing, key)
		}
		b.WriteString(v)
		rest = rest[start+end+2:]
	}
	return b.String(), false, nil
}

var errMissing = fmt.Errorf("missing value for")

// Single returns namespace and name when tpl is exactly one placeholder,
// e.g. "{{params.id}}"; ok is false for fixed or mixed values.
func Single(tpl string) (ns, name string, ok bool) {
	t := strings.TrimSpace(tpl)
	if !strings.HasPrefix(t, "{{") || !strings.HasSuffix(t, "}}") || strings.Count(t, "{{") != 1 {
		return "", "", false
	}
	ns, name, ok = strings.Cut(strings.TrimSpace(t[2:len(t)-2]), ".")
	if !ok || !namespaces[ns] || name == "" {
		return "", "", false
	}
	return ns, name, true
}

// Check verifies a template's syntax without values.
func Check(tpl string) error {
	probe := Values{}
	for ns := range namespaces {
		probe[ns] = map[string]string{}
	}
	_, _, err := Resolve(tpl, probe)
	if err != nil && strings.HasPrefix(err.Error(), errMissing.Error()) {
		return nil
	}
	return err
}
