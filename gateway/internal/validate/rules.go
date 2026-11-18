package validate

import (
	"strings"
	"unicode"
)

// rules sono controlli specifici, non firme generiche: ognuno ammette o
// rifiuta una classe precisa di caratteri o sequenze.
var rules = map[string]func(string) bool{
	// no_html: nessun tag o entità HTML.
	"no_html": func(s string) bool {
		return !strings.ContainsAny(s, "<>") && !strings.Contains(s, "&#")
	},
	// no_sql_meta: nessun metacarattere SQL. L'apostrofo è ammesso solo tra
	// due lettere (es. "D'Angelo"), mai come delimitatore.
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
	// no_template_meta: nessuna sequenza di template o codice lato server.
	"no_template_meta": func(s string) bool {
		for _, seq := range []string{"<?", "?>", "<%", "%>", "${", "#{", "{{", "}}"} {
			if strings.Contains(s, seq) {
				return false
			}
		}
		return true
	},
}
