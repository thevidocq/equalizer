package extract

import (
	"regexp"
	"strings"
)

// PHP, whatever the framework: request keys and what the code makes of
// them, and the calls where a request value may do harm.

const key = `\$_(GET|POST|REQUEST)\s*\[\s*['"]([A-Za-z0-9_\-\[\]]{1,100})['"]\s*\]`

// sanitizers say the type an input becomes; wp_unslash and trim are seen
// through.
var sanitizers = []struct {
	re  *regexp.Regexp
	typ string
}{
	{regexp.MustCompile(`(?i)\b(absint|intval)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "int"},
	{regexp.MustCompile(`\(\s*int\s*\)\s*` + key), "int"},
	{regexp.MustCompile(`(?i)\b(rest_sanitize_boolean|boolval|wp_validate_boolean)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "bool"},
	{regexp.MustCompile(`\(\s*bool\s*\)\s*` + key), "bool"},
	{regexp.MustCompile(`(?i)\b(sanitize_email|is_email)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "email"},
	{regexp.MustCompile(`(?i)\b(esc_url_raw|sanitize_url)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "url"},
	{regexp.MustCompile(`(?i)\bsanitize_key\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "key"},
	{regexp.MustCompile(`(?i)\bsanitize_title(?:_with_dashes)?\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "slug"},
	{regexp.MustCompile(`(?i)\b(sanitize_text_field|sanitize_textarea_field)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "text"},
	{regexp.MustCompile(`(?i)\b(wp_kses_post|wp_kses)\s*\(\s*(?:wp_unslash\s*\(\s*)?` + key), "html"},
	{regexp.MustCompile(`(?i)filter_var\s*\(\s*` + key + `\s*,\s*FILTER_VALIDATE_INT`), "int"},
	{regexp.MustCompile(`(?i)filter_var\s*\(\s*` + key + `\s*,\s*FILTER_VALIDATE_EMAIL`), "email"},
	{regexp.MustCompile(`(?i)filter_var\s*\(\s*` + key + `\s*,\s*FILTER_VALIDATE_URL`), "url"},
	{regexp.MustCompile(`(?i)filter_var\s*\(\s*` + key + `\s*,\s*FILTER_VALIDATE_BOOL`), "bool"},
}

var (
	anyKey = regexp.MustCompile(key)
	// the whole list on the line, closed, of quoted values only
	inArray = regexp.MustCompile(`(?i)(!\s*)?in_array\s*\(\s*(?:\w+\s*\(\s*)*` + key + `\s*\)*\s*,\s*(?:array\s*\(|\[)((?:\s*['"][A-Za-z0-9_\-.]{1,100}['"]\s*,?)+)\s*[)\]]`)
	// a list that refuses its values: the code accepts everything else
	refusal = regexp.MustCompile(`(?i)\b(die|exit|wp_die|throw|return|header|wp_redirect|wp_send_json_error)\b`)
	quoted  = regexp.MustCompile(`['"]([A-Za-z0-9_\-.]{1,100})['"]`)
	inQuery = regexp.MustCompile(`\$_(GET|POST|REQUEST)\s*\[`)
)

// risky calls, by kind. sql and the others count only when a request value
// is on the same line; eval always does.
var risky = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"sql", regexp.MustCompile(`(?i)(\$wpdb\s*->\s*(query|get_results|get_row|get_var|get_col)|\b(mysqli_query|mysql_query|pg_query)|->\s*query)\s*\(`)},
	{"eval", regexp.MustCompile(`(?i)\b(eval|assert|create_function)\s*\(`)},
	{"include", regexp.MustCompile(`(?i)\b(include|include_once|require|require_once)\b[\s(]*[^;]*\$_(GET|POST|REQUEST)`)},
	// a backtick runs a command only around a request value: in a query it
	// quotes a table or column name
	{"exec", regexp.MustCompile("(?i)(\\b(exec|system|shell_exec|passthru|popen|proc_open|pcntl_exec)\\s*\\(|`[^`]*\\$_(GET|POST|REQUEST|COOKIE)[^`]*`)")},
	{"unserialize", regexp.MustCompile(`(?i)\b(unserialize|maybe_unserialize)\s*\(`)},
	{"file_write", regexp.MustCompile(`(?i)\b(file_put_contents|fwrite|move_uploaded_file|copy)\s*\(`)},
}

// phpFile reads one PHP file: its inputs and risky calls, tied to route
// when the file itself is an entry point.
func (s *Summary) phpFile(rel, src, route string) {
	lines := strings.Split(src, "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		if !inQuery.MatchString(line) && !strings.Contains(strings.ToLower(line), "eval") {
			continue
		}
		at := s.at(i + 1)
		typed := map[string]bool{}
		for _, sz := range sanitizers {
			for _, m := range sz.re.FindAllStringSubmatch(line, -1) {
				source, name := m[len(m)-2], m[len(m)-1]
				s.addInput(seen, Input{Name: name, Source: sourceOf(source), Type: sz.typ, Route: route, Where: at})
				typed[name] = true
			}
		}
		for _, m := range inArray.FindAllStringSubmatch(line, -1) {
			if m[1] == "" && refusal.MatchString(line) {
				// in_array(x, list) then die: a list the code refuses, not the
				// values it accepts (!in_array … return is the list it accepts)
				continue
			}
			var values []string
			for _, q := range quoted.FindAllStringSubmatch(m[4], 50) {
				values = append(values, q[1])
			}
			if len(values) > 0 {
				s.addInput(seen, Input{Name: m[3], Source: sourceOf(m[2]), Type: "enum", Values: values, Route: route, Where: at})
				typed[m[3]] = true
			}
		}
		for _, m := range anyKey.FindAllStringSubmatch(line, -1) {
			if !typed[m[2]] {
				s.addInput(seen, Input{Name: m[2], Source: sourceOf(m[1]), Type: "unknown", Route: route, Where: at})
			}
		}
		for _, r := range risky {
			if !r.re.MatchString(line) {
				continue
			}
			input := ""
			if m := anyKey.FindStringSubmatch(line); m != nil {
				input = m[2]
			}
			if input == "" && r.kind != "eval" && !inQuery.MatchString(line) {
				continue
			}
			if r.kind == "sql" && strings.Contains(strings.ToLower(line), "prepare(") {
				continue // a prepared query: the value is bound, not written into it
			}
			if len(s.Risks) < maxRisks {
				s.Risks = append(s.Risks, Risk{Kind: r.kind, Input: input, Route: route, Where: at})
			}
		}
	}
}

func (s *Summary) addInput(seen map[string]bool, in Input) {
	k := in.Name + "|" + in.Source + "|" + in.Type + "|" + in.Route
	if seen[k] || len(s.Inputs) >= maxInputs {
		return
	}
	seen[k] = true
	s.Inputs = append(s.Inputs, in)
}

func sourceOf(super string) string {
	switch super {
	case "GET":
		return "query"
	case "POST":
		return "body"
	}
	return "request"
}
