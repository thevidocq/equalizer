package extract

import (
	"regexp"
	"strings"
)

// A plugin rarely writes a REST namespace or an AJAX action as a plain
// string: it keeps the namespace in a constant or a property
// (self::route_namespace, $this->namespace . '/' . $this->rest_base) and
// builds hook names ('wp_ajax_' . $action). literals collects what one file
// assigns to its constants, properties and variables, and argument turns an
// argument into its text: a part the file does not fix becomes {name}, a
// parameter of unknown type, so the entry point is still reported.

var (
	constLit = regexp.MustCompile(`(?i)\bconst\s+([A-Za-z_]\w*)\s*=\s*['"]([^'"\n]{0,200})['"]`)
	propLit  = regexp.MustCompile(`(?i)\b(?:public|protected|private|var)\s+(?:static\s+)?(?:\??[A-Za-z_\\]+\s+)?\$([A-Za-z_]\w*)\s*=\s*['"]([^'"\n]{0,200})['"]`)
	varLit   = regexp.MustCompile(`\$([A-Za-z_]\w*)\s*=\s*['"]([^'"\n]{0,200})['"]\s*;`)
	thisProp = regexp.MustCompile(`^\$this\s*->\s*([A-Za-z_]\w*)$`)
	staticC  = regexp.MustCompile(`^(?:self|static|[A-Za-z_\\]+)\s*::\s*\$?([A-Za-z_]\w*)$`)
	plainVar = regexp.MustCompile(`^\$([A-Za-z_]\w*)$`)
	interp   = regexp.MustCompile(`\{?\$(?:this->)?([A-Za-z_]\w*)\}?`)
)

type literals struct{ consts, props, vars map[string]string }

func fileLiterals(src string) literals {
	l := literals{map[string]string{}, map[string]string{}, map[string]string{}}
	for _, m := range constLit.FindAllStringSubmatch(src, -1) {
		l.consts[m[1]] = m[2]
	}
	for _, m := range propLit.FindAllStringSubmatch(src, -1) {
		l.props[m[1]] = m[2]
	}
	for _, m := range varLit.FindAllStringSubmatch(src, -1) {
		if _, twice := l.vars[m[1]]; twice {
			l.vars[m[1]] = "\x00" // set more than once: not fixed
		} else {
			l.vars[m[1]] = m[2]
		}
	}
	return l
}

// args splits the call's arguments from src[i:] (just past its "("), at
// most n, each its source text; nested parentheses, brackets and strings
// are skipped. Not found: nil.
func args(src string, i, n int) []string {
	var out []string
	depth, start := 0, i
	for j := i; j < len(src) && j < i+4000; j++ {
		switch c := src[j]; c {
		case '\'', '"':
			for j++; j < len(src) && src[j] != c; j++ {
				if src[j] == '\\' {
					j++
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return append(out, strings.TrimSpace(src[start:j]))
			}
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(src[start:j]))
				if len(out) == n {
					return out
				}
				start = j + 1
			}
		}
	}
	return out
}

// text is an argument's value, its unknown parts as {name}; ok is false
// when no part of it is known.
func (l literals) text(arg string) (string, bool) {
	var b strings.Builder
	known := false
	for _, part := range concatParts(arg) {
		part = strings.TrimSpace(part)
		switch {
		case len(part) >= 2 && part[0] == '\'' && part[len(part)-1] == '\'':
			b.WriteString(part[1 : len(part)-1])
			known = true
		case len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"':
			inner := part[1 : len(part)-1]
			b.WriteString(interp.ReplaceAllStringFunc(inner, func(v string) string {
				name := interp.FindStringSubmatch(v)[1]
				if x, ok := l.vars[name]; ok && x != "\x00" {
					return x
				}
				return "{" + name + "}"
			}))
			known = true
		default:
			v, ok := l.value(part)
			if ok {
				known = true
			}
			b.WriteString(v)
		}
	}
	return b.String(), known
}

func (l literals) value(part string) (string, bool) {
	name, v, ok := "", "", false
	switch {
	case thisProp.MatchString(part):
		name = thisProp.FindStringSubmatch(part)[1]
		v, ok = l.props[name]
	case staticC.MatchString(part):
		name = staticC.FindStringSubmatch(part)[1]
		if v, ok = l.consts[name]; !ok {
			v, ok = l.props[name]
		}
		name = strings.ToLower(name)
	case plainVar.MatchString(part):
		name = plainVar.FindStringSubmatch(part)[1]
		v, ok = l.vars[name]
		ok = ok && v != "\x00"
	default:
		return "{value}", false
	}
	// a value that may be a secret (a key named like one, a token's shape)
	// never leaves the code: the summary keeps its name
	if !ok || secretList(name, []string{v}) || !nameChars.MatchString(v) {
		return "{" + name + "}", false
	}
	return v, true
}

// nameChars: what a namespace, a route (its regex groups included) or an
// action is written with; no space, no quote.
var nameChars = regexp.MustCompile(`^[A-Za-z0-9_./\-()?<>\[\]\\+*^$|{}:=,]{0,200}$`)

// concatParts splits on "." outside strings.
func concatParts(arg string) []string {
	var out []string
	start := 0
	for j := 0; j < len(arg); j++ {
		switch c := arg[j]; c {
		case '\'', '"':
			for j++; j < len(arg) && arg[j] != c; j++ {
				if arg[j] == '\\' {
					j++
				}
			}
		case '.':
			out = append(out, arg[start:j])
			start = j + 1
		}
	}
	return append(out, arg[start:])
}
