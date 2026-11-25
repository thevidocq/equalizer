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
	// a PHP function, not a method ($pdo->exec(), $db::exec())
	{"exec", regexp.MustCompile("(?i)((^|[^>:$\\w])(exec|system|shell_exec|passthru|popen|proc_open|pcntl_exec)\\s*\\(|`[^`]*\\$_(GET|POST|REQUEST|COOKIE)[^`]*`)")},
	{"unserialize", regexp.MustCompile(`(?i)\b(unserialize|maybe_unserialize)\s*\(`)},
	{"file_write", regexp.MustCompile(`(?i)\b(file_put_contents|fwrite|move_uploaded_file|copy)\s*\(`)},
}

// phpFile reads one PHP file: its inputs and risky calls, tied to route
// when the file itself is an entry point.
func (s *Summary) phpFile(rel, src, route string) {
	lines := strings.Split(src, "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		if !inQuery.MatchString(line) && !strings.Contains(strings.ToLower(line), "eval") || comment(line) {
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
		s.risksOn(line, route, at, "")
	}
	// filter_input reads the request with its own filter
	for i, line := range lines {
		if comment(line) || !strings.Contains(line, "filter_input") {
			continue
		}
		for _, m := range filterInput.FindAllStringSubmatch(line, -1) {
			s.addInput(seen, Input{Name: m[2], Source: sourceOf(m[1]), Type: filterType(m[3]), Route: route, Where: s.at(i + 1)})
		}
	}
	// a request value kept in a variable first, then used in a risky call
	// further down the file: $id = $_GET['id']; … $wpdb->query("… $id")
	tainted := taint(lines)
	if len(tainted) == 0 {
		return
	}
	for i, line := range lines {
		if comment(line) || inQuery.MatchString(line) || strings.Contains(strings.ToLower(line), "eval") {
			continue // seen above
		}
		if v, ok := taintedOn(line, tainted); ok && !assigns(line, v) {
			s.risksOn(line, route, s.at(i+1), tainted[v])
		}
	}
}

// risksOn adds the risky calls of one line. input: the request key a
// variable on the line holds, "" when the line reads the request itself.
func (s *Summary) risksOn(line, route string, at Where, input string) {
	for _, r := range risky {
		if !r.re.MatchString(line) {
			continue
		}
		in := input
		if m := anyKey.FindStringSubmatch(line); m != nil {
			in = m[2]
		}
		if in == "" && r.kind != "eval" && !inQuery.MatchString(line) {
			continue
		}
		if r.kind == "sql" && (strings.Contains(strings.ToLower(line), "prepare(") || queryArgs.MatchString(line)) {
			continue // a prepared query, or WP_Query's array of arguments: no SQL written
		}
		if len(s.Risks) < maxRisks {
			s.Risks = append(s.Risks, Risk{Kind: r.kind, Input: in, Route: route, Where: at})
		}
	}
}

var (
	assignment = regexp.MustCompile(`^\s*\$([A-Za-z_]\w*)\s*=[^=>](.*)$`)
	// what reads the request, besides $_GET and the like
	otherRead = regexp.MustCompile(`(?:->\s*get_param\s*\(|\$request\s*\[|filter_input\s*\(\s*INPUT_(?:GET|POST|REQUEST|COOKIE)\s*,)\s*['"]([A-Za-z0-9_\-]{1,100})['"]`)
	// what leaves nothing harmful: a number, a yes or no, a key, an escaped
	// SQL value
	safeCast = regexp.MustCompile(`(?i)\b(absint|intval|floatval|boolval|sanitize_key|esc_sql|wp_validate_boolean|rest_sanitize_boolean|count|is_numeric|isset|empty)\s*\(|\(\s*(int|integer|float|bool|boolean)\s*\)|FILTER_VALIDATE_(INT|BOOL|FLOAT)`)
)

// taint: the variables a file sets from a request value it did not make
// safe (a number, a key), each with the key it holds; a variable set from
// one of them inherits its key (the first such variable in the line). Each
// line's variables are looked up in a map: linear in the file.
func taint(lines []string) map[string]string {
	out := map[string]string{}
	for pass := 0; pass < 2; pass++ {
		for _, line := range lines {
			m := assignment.FindStringSubmatch(line)
			if m == nil || comment(line) || safeCast.MatchString(m[2]) {
				continue
			}
			if k := anyKey.FindStringSubmatch(m[2]); k != nil {
				out[m[1]] = k[2]
				continue
			}
			if k := otherRead.FindStringSubmatch(m[2]); k != nil {
				out[m[1]] = k[1]
				continue
			}
			for _, v := range varName.FindAllStringSubmatch(m[2], -1) {
				if key, ok := out[v[1]]; ok && v[1] != m[1] {
					out[m[1]] = key
					break
				}
			}
		}
	}
	return out
}

var varName = regexp.MustCompile(`\$([A-Za-z_]\w*)`)

// taintedOn: the first variable of the line that holds a request value.
func taintedOn(line string, tainted map[string]string) (string, bool) {
	for _, v := range varName.FindAllStringSubmatch(line, -1) {
		if _, ok := tainted[v[1]]; ok {
			return v[1], true
		}
	}
	return "", false
}

// assigns: the line only sets v (its own risky calls are seen through the
// value it reads)
func assigns(line, v string) bool {
	m := assignment.FindStringSubmatch(line)
	return m != nil && m[1] == v && !strings.Contains(m[2], "$"+v)
}

func (s *Summary) addInput(seen map[string]bool, in Input) {
	if in.Type == "enum" && secretList(in.Name, in.Values) {
		// a key compared with a token written in the code (a backdoor, a
		// shared secret): the summary never carries the value
		in.Type, in.Values = "unknown", nil
	}
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

var (
	secretKey   = regexp.MustCompile(`(?i)(pass|pwd|secret|token|key|auth|sig|hash|nonce|salt|otp|code|cred)`)
	secretShape = regexp.MustCompile(`^[A-Za-z0-9+/=_.\-]{16,}$`)
	hasDigit    = regexp.MustCompile(`[0-9]`)
	hasLetter   = regexp.MustCompile(`[A-Za-z]`)
)

// secretList: values a key is compared with that may be a secret: a value
// of 8 characters or more for a key named like a secret (token, pass, key…),
// or a value that looks like a token (16 characters or more, letters and
// digits, no space). A country_code of IT and FR stays a list.
func secretList(name string, values []string) bool {
	for _, v := range values {
		if secretKey.MatchString(name) && len(v) >= 8 ||
			secretShape.MatchString(v) && hasDigit.MatchString(v) && hasLetter.MatchString(v) {
			return true
		}
	}
	return false
}

// comment: a line that is only a comment (a docblock's "* @param …
// `$_POST['x']`", a "// eval(…)" note): nothing it names runs.
func comment(line string) bool {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "/*") {
		// code after a comment closed on the same line still runs
		end := strings.Index(t, "*/")
		return end < 0 || strings.TrimSpace(t[end+2:]) == ""
	}
	return strings.HasPrefix(t, "*") && !strings.HasPrefix(t, "*/") || strings.HasPrefix(t, "//") ||
		strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "#[")
}

var queryArgs = regexp.MustCompile(`->\s*query\s*\(\s*\$\w*args\s*\)`)

var filterInput = regexp.MustCompile(`filter_input\s*\(\s*INPUT_(GET|POST|REQUEST)\s*,\s*['"]([A-Za-z0-9_\-]{1,100})['"]\s*(?:,\s*FILTER_([A-Z_]+))?`)

// filterType: what a PHP filter makes of a value.
func filterType(f string) string {
	switch f {
	// a FILTER_SANITIZE_ strips characters and refuses nothing: a phone
	// number read as SANITIZE_NUMBER_INT is not an int to the site
	case "VALIDATE_INT":
		return "int"
	case "VALIDATE_EMAIL":
		return "email"
	case "VALIDATE_URL":
		return "url"
	case "VALIDATE_BOOLEAN", "VALIDATE_BOOL":
		return "bool"
	}
	return "unknown"
}
