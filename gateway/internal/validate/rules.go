package validate

import (
	"strings"
	"unicode"
)

// rules are specific checks, not generic signatures: each one accepts or
// rejects a precise class of characters or sequences.
var rules = map[string]func(string) bool{
	// no_html: no HTML tags or numeric entities.
	"no_html": func(s string) bool {
		return !strings.ContainsAny(s, "<>") && !strings.Contains(s, "&#")
	},
	// no_sql_meta: no SQL metacharacters. The apostrophe is allowed only
	// between two letters (e.g. "O'Brien"), never as a delimiter.
	"no_sql_meta": func(s string) bool {
		for _, seq := range []string{"--", "/*", "*/", ";", "\\", "\"", "\x00", "`"} {
			if strings.Contains(s, seq) {
				return false
			}
		}
		runes := []rune(s)
		for i, c := range runes {
			if c != '\'' {
				continue
			}
			if i == 0 || i == len(runes)-1 || !unicode.IsLetter(runes[i-1]) || !unicode.IsLetter(runes[i+1]) {
				return false
			}
		}
		return true
	},
	// no_template_meta: no template or server-side code sequences.
	"no_template_meta": func(s string) bool {
		for _, seq := range []string{"<?", "?>", "<%", "%>", "${", "#{", "{{", "}}"} {
			if strings.Contains(s, seq) {
				return false
			}
		}
		return true
	},
}
