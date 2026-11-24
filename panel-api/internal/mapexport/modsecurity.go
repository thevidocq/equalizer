package mapexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ModSecurity writes the version as ModSecurity rules for ModSecurity v3
// and Coraza: a positive model, deny by default. Each entry of the map
// sets tx.eq_entry when the method and the path match; then its query
// keys, form fields and API body are held to the map's checks; a request
// that matched nothing is refused. Rule ids are in the local range from
// FirstID.
func ModSecurity(raw []byte, app string, version int) ([]byte, error) {
	var d doc
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("the map could not be read: %w", err)
	}
	w := &rules{id: FirstID}
	w.header(app, version)
	w.section("Entries: which entry of the map a request is")
	for _, r := range d.list("routes") {
		method := str(r["method"])
		if method == "" {
			method = "GET"
		}
		w.entry("r_"+str(r["id"]), method, d.pathRegex(str(r["path"]), asMap(r["params"])))
	}
	for _, f := range d.list("forms") {
		w.entry("f_"+str(f["id"]), "POST", d.pathRegex(str(f["route"]), asMap(f["params"])))
	}
	for _, a := range d.list("api") {
		w.entry("a_"+str(a["id"]), str(a["method"]), d.pathRegex(str(a["path"]), asMap(a["params"])))
	}
	for _, a := range d.list("assets") {
		w.prefix("asset_"+ident(str(a["prefix"])), []string{"GET"}, str(a["prefix"]))
	}
	for _, a := range d.list("areas") {
		methods := strs(a["methods"])
		if len(methods) == 0 {
			methods = []string{"GET", "POST"}
		}
		w.prefix("area_"+ident(str(a["prefix"])), methods, str(a["prefix"]))
	}
	w.section("Deny by default: what is not in the map does not exist")
	w.rule(`SecRule &TX:EQ_ENTRY "@eq 0" "id:%%d,phase:2,deny,status:404,log,msg:'Equalizer map: not in the map'"`)

	w.section("Checks of each entry")
	for _, r := range d.list("routes") {
		e := "r_" + str(r["id"])
		w.names(e, "ARGS_GET_NAMES", "&ARGS_GET", "query key", d.fields(asMap(r["query"])), "")
		w.values(e, "ARGS_GET", d.fields(asMap(r["query"])), "")
		w.names(e, "ARGS_POST_NAMES", "&ARGS_POST", "body field", nil, "")
	}
	for _, f := range d.list("forms") {
		e := "f_" + str(f["id"])
		var plain, files []field
		for _, x := range d.fields(asMap(f["fields"])) {
			if x.base == "file" {
				files = append(files, x)
			} else {
				plain = append(plain, x)
			}
		}
		w.names(e, "ARGS_GET_NAMES", "&ARGS_GET", "query key", nil, "")
		w.names(e, "ARGS_POST_NAMES", "&ARGS_POST", "form field", plain, "")
		w.values(e, "ARGS_POST", plain, "")
		w.files(e, files)
	}
	for _, a := range d.list("api") {
		e := "a_" + str(a["id"])
		w.names(e, "ARGS_GET_NAMES", "&ARGS_GET", "query key", d.fields(asMap(a["query"])), "")
		w.values(e, "ARGS_GET", d.fields(asMap(a["query"])), "")
		if body := asMap(a["body"]); body != nil {
			// a JSON body: read as JSON (chosen in phase 1, before the body),
			// the engine names its keys json.<key>
			w.rule(`SecRule REQUEST_METHOD "@streq %s" "id:%%d,phase:1,pass,nolog,t:none,chain"
    SecRule REQUEST_FILENAME "@rx %s" "t:none,ctl:requestBodyProcessor=JSON"`, str(a["method"]), quote(d.pathRegex(str(a["path"]), asMap(a["params"]))))
			w.names(e, "ARGS_POST_NAMES", "&ARGS_POST", "body key", d.fields(body), "json.")
			w.values(e, "ARGS_POST", d.fields(body), "json.")
		} else {
			w.names(e, "ARGS_POST_NAMES", "&ARGS_POST", "body field", nil, "")
		}
	}
	for _, a := range d.list("assets") {
		e := "asset_" + ident(str(a["prefix"]))
		if t := strs(a["types"]); len(t) > 0 {
			w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:3,deny,status:403,log,msg:'Equalizer map: a file type the folder does not serve',chain"
    SecRule RESPONSE_HEADERS:Content-Type "!@rx ^(?:%s)\s*(?:;|$)" "t:none,t:lowercase"`, e, alternation(t))
		}
	}
	if w.id > LastID {
		return nil, errors.New("the map needs more rules than the local range holds")
	}
	return []byte(w.b.String()), nil
}

// The local range of rule ids (1-99,999 is for a site's own rules).
const (
	FirstID = 10000
	LastID  = 99999
)

type rules struct {
	b  strings.Builder
	id int
}

func (w *rules) header(app string, version int) {
	fmt.Fprintf(&w.b, `# Equalizer map of %s, version %d, as ModSecurity rules (ModSecurity v3, Coraza).
#
# A positive model, deny by default: a method and path that are not in the
# map are refused, and on each entry only its query keys and fields pass,
# each held to its type, length and rules.
#
# What a WAF keeps of the map: the paths, the methods, the names of the
# keys and fields, their types, lengths, lists of values and patterns, the
# rules no_html, no_template_meta and no_sql_meta (approximated), the file
# types of the folders, the size of uploads.
#
# What a WAF loses: it forwards the request as it came, while Equalizer
# rebuilds it from the map and never forwards a client's bytes; the roles
# and sessions of the map; the checks of ownership (owned_by_user); the
# rate limits; the ranges of numbers (only digits are checked here); the
# rule no_lookalikes; the rebuilding of images; the rewriting of the pages.
# Lengths are counted in bytes here, in characters by the gateway: a text
# with accents reaches its limit sooner.
#
# Needs SecRequestBodyAccess On (form fields and JSON bodies). Rule ids %d
# and up. Try it with SecRuleEngine DetectionOnly first.
`, clean(app), version, FirstID)
}

func (w *rules) section(title string) {
	fmt.Fprintf(&w.b, "\n# --- %s\n\n", title)
}

// rule writes a rule; %d (the first, unescaped) is its id.
func (w *rules) rule(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	text = strings.Replace(text, "%d", fmt.Sprint(w.id), 1)
	w.id++
	w.b.WriteString(text + "\n")
}

func (w *rules) entry(id, method, path string) {
	if path == "" || method == "" {
		return
	}
	w.rule(`SecRule &TX:EQ_ENTRY "@eq 0" "id:%%d,phase:2,pass,nolog,t:none,chain"
    SecRule REQUEST_METHOD "@streq %s" "t:none,chain"
        SecRule REQUEST_FILENAME "@rx %s" "t:none,setvar:'tx.eq_entry=%s'"`, method, quote(path), id)
}

func (w *rules) prefix(id string, methods []string, prefix string) {
	if prefix == "" {
		return
	}
	w.rule(`SecRule &TX:EQ_ENTRY "@eq 0" "id:%%d,phase:2,pass,nolog,t:none,chain"
    SecRule REQUEST_METHOD "@rx ^(?:%s)$" "t:none,chain"
        SecRule REQUEST_FILENAME "@beginsWith %s" "t:none,setvar:'tx.eq_entry=%s'"`, alternation(methods), quote(prefix), id)
}

// names refuses a key the entry does not have (none when fields is empty).
func (w *rules) names(entry, names, count, what string, fields []field, prefix string) {
	if len(fields) == 0 {
		w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'Equalizer map: %s takes no %s',chain"
    SecRule %s "@gt 0" "t:none"`, entry, entry, what, count)
		return
	}
	var allowed []string
	for _, f := range fields {
		allowed = append(allowed, prefix+f.name)
	}
	w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'Equalizer map: a %s that %s does not take',chain"
    SecRule %s "!@rx ^(?:%s)$" "t:none"`, entry, what, entry, names, alternation(allowed))
}

// values holds each field to its check.
func (w *rules) values(entry, coll string, fields []field, prefix string) {
	for _, f := range fields {
		v := coll + ":" + prefix + f.name
		msg := fmt.Sprintf("Equalizer map: %s on %s", clean(f.name), entry)
		if f.required {
			w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'%s is missing',chain"
    SecRule &%s "@eq 0" "t:none"`, entry, msg, v)
		}
		if re := valueRegex(f); re != "" {
			w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'%s does not fit its check (%s)',chain"
    SecRule %s "!@rx %s" "t:none"`, entry, msg, f.base, v, quote(re))
		}
		if f.base == "int" || f.base == "enum" || f.base == "bool" || f.base == "date" {
			continue
		}
		w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'%s is longer than %d',chain"
    SecRule %s "@gt %d" "t:none,t:length"`, entry, msg, f.maxLength, v, f.maxLength)
		for _, r := range f.rules {
			if re, ok := ruleRegex[r]; ok {
				w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'%s breaks %s',chain"
    SecRule %s "@rx %s" "t:none"`, entry, msg, r, v, quote(re))
			}
		}
		if len(f.deny) > 0 {
			w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'%s is a refused value',chain"
    SecRule %s "@rx ^(?:%s)$" "t:none,t:lowercase"`, entry, msg, v, alternation(lower(f.deny)))
		}
	}
}

func (w *rules) files(entry string, files []field) {
	var names []string
	for _, f := range files {
		names = append(names, f.name)
		if f.maxSizeKB > 0 {
			w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'Equalizer map: %s on %s is larger than %d KB',chain"
    SecRule FILES_SIZES:%s "@gt %d" "t:none"`, entry, clean(f.name), entry, f.maxSizeKB, f.name, f.maxSizeKB*1024)
		}
	}
	if len(names) == 0 {
		w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'Equalizer map: %s takes no file',chain"
    SecRule &FILES "@gt 0" "t:none"`, entry, entry)
		return
	}
	w.rule(`SecRule TX:EQ_ENTRY "@streq %s" "id:%%d,phase:2,deny,status:403,log,msg:'Equalizer map: a file that %s does not take',chain"
    SecRule FILES_NAMES "!@rx ^(?:%s)$" "t:none"`, entry, entry, alternation(names))
}

// ruleRegex: what each of the map's rules refuses. no_sql_meta allows an
// apostrophe between two ASCII letters only (the gateway, between any two
// letters); no_lookalikes has no rule here.
var ruleRegex = map[string]string{
	"no_html":          `[<>]|&#`,
	"no_template_meta": `<\?|\?>|<%|%>|\$\{|#\{|\{\{|\}\}`,
	"no_sql_meta":      `--|/\*|\*/|;|\\|"|\x00|` + "`" + `|(?:^|[^A-Za-z])'|'(?:[^A-Za-z]|$)`,
}

// valueRegex is the anchored expression a value must match, "" when only
// its length and rules are checked.
func valueRegex(f field) string {
	if p := part(f); p != "" {
		return "^" + p + "$"
	}
	return ""
}

// part is a field's check as an expression without anchors, for a value or
// a path segment; "" for free text.
func part(f field) string {
	switch f.base {
	case "int":
		return `-?[0-9]{1,18}`
	case "enum":
		return "(?:" + alternation(f.values) + ")"
	case "bool":
		return `(?:true|false|1|0)`
	case "date":
		return `[0-9]{4}-[0-9]{2}-[0-9]{2}`
	case "email":
		return `[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`
	}
	switch {
	case f.pattern != "":
		return "(?:" + strings.TrimSuffix(strings.TrimPrefix(f.pattern, "^"), "$") + ")"
	case f.charset != "":
		return "(?:" + f.charset + ")*"
	}
	return ""
}

// pathRegex is a public path as an anchored expression, each {param} held
// to its check (any segment for free text).
func (d doc) pathRegex(p string, params map[string]any) string {
	if p == "" {
		return ""
	}
	byName := map[string]field{}
	for _, f := range d.fields(params) {
		byName[f.name] = f
	}
	var b strings.Builder
	b.WriteString("^")
	for p != "" {
		i := strings.IndexByte(p, '{')
		j := strings.IndexByte(p, '}')
		if i < 0 || j < i {
			b.WriteString(regexp.QuoteMeta(p))
			break
		}
		b.WriteString(regexp.QuoteMeta(p[:i]))
		f, ok := byName[p[i+1:j]]
		if x := part(f); ok && x != "" {
			b.WriteString(x)
		} else {
			b.WriteString(`[^/]+`)
		}
		p = p[j+1:]
	}
	return b.String() + "$"
}

func alternation(values []string) string {
	q := make([]string, len(values))
	for i, v := range values {
		q[i] = regexp.QuoteMeta(v)
	}
	return strings.Join(q, "|")
}

func lower(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.ToLower(v)
	}
	return out
}

// quote makes an operator's argument safe inside its double quotes.
func quote(s string) string {
	return strings.NewReplacer(`"`, `\"`, "\n", "", "\r", "").Replace(s)
}

// clean keeps a name for a message: no quote or newline can end it.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\'' || r == '"' || r == '\n' || r == '\r' || r == '\\' {
			return '_'
		}
		return r
	}, s)
}
