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
	scopeWord     = regexp.MustCompile(`(?i)\b(function|class|trait|interface|enum)\b[^;{]*$`)
)

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
				out.WriteString(" '' ")
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

// library tells the code of others shipped with the site: WordPress's
// core, and the dependencies a plugin or theme bundles. Their entry points,
// inputs and risky calls are left out (known code, versions from
// composer.lock and the core's version.php).
func library(stack, rel string) bool {
	if stack == "wordpress" && (strings.HasPrefix(rel, "wp-admin/") || strings.HasPrefix(rel, "wp-includes/") ||
		rel == "index.php" || rel == "xmlrpc.php" || !strings.Contains(rel, "/") && strings.HasPrefix(rel, "wp-")) {
		return true // a custom file next to them stays the site's own
	}
	for _, d := range []string{"/vendor/", "/vendors/", "/third-party/", "/third_party/", "/libraries/", "/bower_components/"} {
		if strings.Contains("/"+rel, d) {
			return true
		}
	}
	return false
}
