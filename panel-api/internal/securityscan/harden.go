package securityscan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Fix is a change of a map draft that closes one finding.
type Fix struct {
	Finding string // the scanner's id
	Change  string // in a few words, for the Security tab and the draft's note
}

// What each text rule closes, by the weakness the scanner names (CWE) or the
// words of its title. A rule is added only to a free-text input a finding
// names: never to a typed one, never to an input no finding names.
var ruleFor = []struct {
	rule  string
	cwe   []string
	words []string
}{
	{"no_html", []string{"CWE-79", "CWE-80", "CWE-83", "CWE-87"}, []string{"xss", "cross-site scripting", "cross site scripting", "html injection"}},
	{"no_sql_meta", []string{"CWE-89", "CWE-564"}, []string{"sql injection", "sqli", "sql error"}},
	{"no_template_meta", []string{"CWE-94", "CWE-95", "CWE-917", "CWE-1336"}, []string{"template injection", "ssti", "code injection", "expression language"}},
}

// rules are the text rules that close a finding, from its CWE and title.
func rules(f store.SecurityFinding) []string {
	title := strings.ToLower(f.Title)
	var out []string
	for _, r := range ruleFor {
		hit := false
		for _, c := range f.CWE {
			for _, w := range r.cwe {
				hit = hit || strings.EqualFold(strings.TrimSpace(c), w)
			}
		}
		for _, w := range r.words {
			hit = hit || strings.Contains(title, w)
		}
		if hit {
			out = append(out, r.rule)
		}
	}
	return out
}

// Harden adds to doc (a map document) the text rules that close the
// findings the map serves as free text, and returns what it changed. The
// verdict is read on doc itself, so a draft that already closes a finding
// is left alone. Findings it cannot close (the area's, the server's, a
// weakness no rule covers) are not touched: the Security tab says so.
func Harden(doc map[string]any, m *Map, findings []store.SecurityFinding) []Fix {
	var fixes []Fix
	for _, f := range findings {
		if f.Parameter == nil || f.URL == nil {
			continue
		}
		v := m.Judge(*f.URL, deref(f.HTTPMethod), *f.Parameter)
		if v.Verdict != FreeText || v.Entry == nil {
			continue
		}
		want := rules(f)
		if len(want) == 0 {
			continue
		}
		spec := fieldSpec(doc, *v.Entry, *f.Parameter)
		if spec == nil {
			continue
		}
		added := addRules(spec, want)
		if len(added) == 0 {
			continue // the draft has them already: the finding is for the gateway's rules to judge
		}
		fixes = append(fixes, Fix{Finding: f.RemoteID,
			Change: fmt.Sprintf("%s.%s refuses %s", *v.Entry, *f.Parameter, strings.Join(names(added), ", "))})
	}
	return fixes
}

// fieldSpec finds an input of an entry in the document: route parameters
// and query, form fields, API parameters, query and body.
func fieldSpec(doc map[string]any, entry, name string) map[string]any {
	groups := map[string][]string{"routes": {"params", "query"}, "forms": {"fields"}, "api": {"params", "query", "body"}}
	for list, keys := range groups {
		entries, _ := doc[list].([]any)
		for _, e := range entries {
			em, _ := e.(map[string]any)
			if em == nil || em["id"] != entry {
				continue
			}
			for _, k := range keys {
				fields, _ := em[k].(map[string]any)
				if spec, ok := fields[name].(map[string]any); ok {
					return spec
				}
			}
		}
	}
	return nil
}

// addRules adds the rules spec lacks and returns them, sorted.
func addRules(spec map[string]any, want []string) []string {
	have := map[string]bool{}
	list, _ := spec["rules"].([]any)
	for _, r := range list {
		if s, ok := r.(string); ok {
			have[s] = true
		}
	}
	var added []string
	for _, r := range want {
		if !have[r] {
			have[r] = true
			list = append(list, r)
			added = append(added, r)
		}
	}
	if len(added) > 0 {
		spec["rules"] = list
	}
	sort.Strings(added)
	return added
}

var ruleNames = map[string]string{"no_html": "markup", "no_sql_meta": "SQL characters", "no_template_meta": "template sequences"}

func names(rules []string) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = ruleNames[r]
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
