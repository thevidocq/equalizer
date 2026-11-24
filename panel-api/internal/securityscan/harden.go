package securityscan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Fix is a change of a map draft that hardens one finding's input.
type Fix struct {
	Finding string // the scanner's id
	Change  string // in a few words, for the Security tab and the draft's note
}

// What each text rule refuses, by the weakness the scanner names (CWE) or
// the words of its title. A rule narrows an input, it does not prove the
// attack impossible (markup only, not attribute or script contexts; quotes
// and comment marks, not a bare numeric injection), so the panel says
// "hardened", never "closed". Code injection (eval) has no text rule.
var ruleFor = []struct {
	rule  string
	cwe   []string
	words *regexp.Regexp
}{
	{"no_html", []string{"CWE-79", "CWE-80", "CWE-83", "CWE-87"},
		regexp.MustCompile(`\b(xss|cross[- ]site scripting|html injection)\b`)},
	{"no_sql_meta", []string{"CWE-89", "CWE-564"},
		regexp.MustCompile(`\b(sqli|sql injection|sql error)\b`)},
	{"no_template_meta", []string{"CWE-917", "CWE-1336"},
		regexp.MustCompile(`\b(template injection|ssti|expression language)\b`)},
}

// rules are the text rules for a finding, from its CWE and title.
func rules(f store.SecurityFinding) []string {
	title := strings.ToLower(f.Title)
	var out []string
	for _, r := range ruleFor {
		hit := r.words.MatchString(title)
		for _, c := range f.CWE {
			for _, w := range r.cwe {
				hit = hit || strings.EqualFold(strings.TrimSpace(c), w)
			}
		}
		if hit {
			out = append(out, r.rule)
		}
	}
	return out
}

// Hardened is what Harden found.
type Hardened struct {
	Added   []Fix // rules this call added to the document
	Already []Fix // findings whose input has its rules already (an earlier draft)
}

// Harden adds to doc (a map document) the text rules for the findings the
// published map serves as free text (judged with published), and returns
// what it added and which findings doc hardens already. Findings no rule
// covers (the area's, the server's, other weaknesses) are left alone: the
// Security tab says so.
func Harden(doc map[string]any, published *Map, findings []store.SecurityFinding) Hardened {
	var h Hardened
	touched := map[string]bool{} // inputs this call changed: later findings on them are added too
	for _, f := range findings {
		if f.Parameter == nil || f.URL == nil {
			continue
		}
		v := published.Judge(*f.URL, deref(f.HTTPMethod), *f.Parameter)
		if v.Verdict != FreeText || v.Entry == nil {
			continue
		}
		want := rules(f)
		if len(want) == 0 {
			continue
		}
		spec := fieldSpec(doc, v.list, *v.Entry, *f.Parameter)
		if spec == nil {
			continue
		}
		key := v.list + "/" + *v.Entry + "/" + *f.Parameter
		added := addRules(spec, want)
		fix := Fix{Finding: f.RemoteID, Change: fmt.Sprintf("%s.%s refuses %s", *v.Entry, *f.Parameter, strings.Join(names(want), ", "))}
		switch {
		case len(added) > 0:
			touched[key] = true
			h.Added = append(h.Added, fix)
		case touched[key]:
			h.Added = append(h.Added, fix) // a second finding on an input this call hardened
		default:
			h.Already = append(h.Already, fix)
		}
	}
	return h
}

// fieldSpec finds an input of an entry of a list (routes, forms, api).
func fieldSpec(doc map[string]any, list, entry, name string) map[string]any {
	keys := map[string][]string{"routes": {"params", "query"}, "forms": {"fields"}, "api": {"params", "query", "body"}}[list]
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
	sort.Strings(out)
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
