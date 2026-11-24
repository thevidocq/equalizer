package extract

import (
	"regexp"
	"strings"
)

// A file is an entry point when it does something as soon as it is opened:
// outside its functions and classes it reads the request or makes a risky
// call. A file that only declares functions or classes, or a template that
// prints what it is given, does nothing on its own. This reads the code's
// structure by its braces, strings and comments, not with a PHP parser: an
// approximation, which can miss an unusual file.

var (
	actsOnRequest = regexp.MustCompile(`\$_(GET|POST|REQUEST|COOKIE|FILES|SERVER)\s*\[|php://input`)
	// A named function, or a class-like: their bodies run only when called.
	// A closure (function followed by its parentheses) is not one: handed to
	// a hook such as add_action('init', ...), it runs on every request.
	// Foo::class and $o->class are names, not declarations.
	scopeWord = regexp.MustCompile(`(?i)(^|[^:>$\w])(function\s*&?\s*[A-Za-z_]\w*|(class|trait|interface|enum)\b)[^;{]*$`)
	heredoc   = regexp.MustCompile(`^<<<[ \t]*(["']?)([A-Za-z_][A-Za-z0-9_]*)(["']?)\r?\n`)
)

// heredocEnd is the index just past a heredoc or nowdoc that starts at
// src[0:], or -1 when src does not start one.
func heredocEnd(src string) int {
	m := heredoc.FindStringSubmatchIndex(src)
	if m == nil || src[m[2]:m[3]] != src[m[6]:m[7]] {
		return -1
	}
	id := src[m[4]:m[5]]
	for i := m[1]; i < len(src); {
		line := src[i:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		t := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(t, id) && (len(t) == len(id) || !isWord(t[len(id)])) {
			return i + (len(line) - len(t)) + len(id)
		}
		i += len(line) + 1
	}
	return len(src)
}

// blank stands for a string: empty, but a double-quoted string (or a
// heredoc) that reads the request keeps that, for PHP reads it there too.
func blank(quote byte, body string) string {
	if quote == '"' {
		if m := actsOnRequest.FindString(body); m != "" {
			return " " + m + " "
		}
	}
	return " '' "
}

func isWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// attributeEnd is the index just past a #[...] attribute at src[0:].
func attributeEnd(src string) int {
	depth := 0
	for j := 1; j < len(src); j++ {
		switch src[j] {
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				return j + 1
			}
		case '\'', '"':
			q := src[j]
			for j++; j < len(src) && src[j] != q; j++ {
				if src[j] == '\\' {
					j++
				}
			}
		}
	}
	return len(src)
}

// topLevel returns the PHP code of src outside functions and classes,
// without strings, comments or the HTML around it.
func topLevel(src string) string {
	var out, pending strings.Builder
	inPHP := false
	var scopes []bool // per open brace: true when it opens a function or class
	inside := func() bool {
		for _, f := range scopes {
			if f {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(src); i++ {
		if !inPHP {
			if strings.HasPrefix(src[i:], "<?php") || strings.HasPrefix(src[i:], "<?=") {
				inPHP = true
				i += 2
			}
			continue
		}
		c := src[i]
		switch {
		case strings.HasPrefix(src[i:], "?>"):
			inPHP = false
			i++
		case c == '#' && i+1 < len(src) && src[i+1] == '[':
			i += attributeEnd(src[i:]) - 1
		case c == '<' && strings.HasPrefix(src[i:], "<<<") && heredocEnd(src[i:]) > 0:
			end := heredocEnd(src[i:])
			if !inside() {
				q := byte('"')
				if strings.HasPrefix(strings.TrimLeft(src[i+3:], " \t"), "'") {
					q = '\'' // a nowdoc: PHP reads nothing in it
				}
				out.WriteString(blank(q, src[i:i+end]))
			}
			i += end - 1
		case c == '/' && i+1 < len(src) && src[i+1] == '/', c == '#':
			for i < len(src) && src[i] != '\n' && !strings.HasPrefix(src[i:], "?>") {
				i++
			}
			i--
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return out.String()
			}
			i += end + 3
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(src) && src[j] != c {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if !inside() {
				out.WriteString(blank(c, src[i+1:min(j, len(src))]))
			}
			i = j
		case c == '{':
			scopes = append(scopes, scopeWord.MatchString(pending.String()))
			pending.Reset()
		case c == '}':
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}
			pending.Reset()
		case c == ';':
			pending.Reset()
			if !inside() {
				out.WriteByte(c)
			}
		default:
			pending.WriteByte(c)
			if !inside() {
				out.WriteByte(c)
			}
		}
	}
	return out.String()
}

// acts tells whether a PHP file does something on its own when opened.
func acts(src string) bool {
	top := topLevel(src)
	if actsOnRequest.MatchString(top) {
		return true
	}
	for _, r := range risky {
		if r.re.MatchString(top) {
			return true
		}
	}
	return false
}

// library tells the code of others a site ships, in any application: the
// dependencies a package manager or a plugin bundles. Their entry points,
// inputs and risky calls are left out (known code, versions from
// composer.lock). Each application's own core is its kind's.
func library(rel string) bool {
	for _, d := range []string{"/vendor/", "/vendors/", "/third-party/", "/third_party/", "/bower_components/"} {
		if strings.Contains("/"+rel, d) {
			return true
		}
	}
	return false
}
