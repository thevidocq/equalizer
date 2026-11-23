// Package review compares a map version with the published one and flags
// risky changes with fixed rules. It is the deterministic half of the
// approver's aid: it always runs, needs no AI, and never decides anything.
package review

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// Change is one entry added, removed or changed.
type Change struct {
	Section string `json:"section"`
	Entry   string `json:"entry"`
	Change  string `json:"change"`
	Detail  string `json:"detail,omitempty"`
}

// Finding is a risk the approver should look at.
type Finding struct {
	Severity string `json:"severity"`
	Entry    string `json:"entry"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Result of a comparison.
type Result struct {
	Changes  []Change  `json:"changes"`
	Findings []Finding `json:"findings"`
}

type obj = map[string]any

var (
	sensitive = regexp.MustCompile(`(?i)(admin|backup|export|dump|debug|config|install|setup|phpinfo|console|\.sql|\.zip|\.bak|\.env)`)
	personal  = regexp.MustCompile(`(?i)(user|account|profile|my|order|invoice|document|certificate|pratic|richiest|fattur|profilo|utent|cittadin|domand)`)
	narrow    = map[string]bool{"int": true, "bool": true, "date": true, "enum": true}
)

// Compare returns what changes from base (the published map, nil when none)
// to next, and the findings.
func Compare(base, next json.RawMessage) Result {
	var b, n obj
	_ = json.Unmarshal(base, &b)
	_ = json.Unmarshal(next, &n)
	if b == nil {
		b = obj{}
	}
	c := &cmp{types: typesOf(n), oldTypes: typesOf(b), anon: str(path(n, "roles", "anonymous"))}
	c.legacy(asObj(b["legacy"]), asObj(n["legacy"]))
	c.roles(asObj(b["roles"]), asObj(n["roles"]))
	c.defaults(asObj(b["defaults"]), asObj(n["defaults"]))
	c.typeDefs(asObj(b["types"]), asObj(n["types"]))
	for _, sec := range []string{"routes", "forms", "api"} {
		c.entries(sec, byID(b[sec], "id"), byID(n[sec], "id"))
	}
	c.assets(byID(b["assets"], "prefix"), byID(n["assets"], "prefix"))
	c.areas(byID(b["areas"], "prefix"), byID(n["areas"], "prefix"))
	if len(asList(b["behavior"])) > len(asList(n["behavior"])) {
		c.add(Change{"behavior", "behavior", "removed", "fewer behaviour rules"})
		c.find("medium", "behavior", "behavior_removed", "Behaviour rules were removed: scanners and bursts are blocked less.")
	}
	sort.SliceStable(c.r.Findings, func(i, j int) bool { return rank[c.r.Findings[i].Severity] < rank[c.r.Findings[j].Severity] })
	if c.r.Changes == nil {
		c.r.Changes = []Change{}
	}
	if c.r.Findings == nil {
		c.r.Findings = []Finding{}
	}
	return c.r
}

var rank = map[string]int{"high": 0, "medium": 1, "low": 2}

type cmp struct {
	r        Result
	types    map[string]obj
	oldTypes map[string]obj
	anon     string
}

func (c *cmp) add(ch Change) { c.r.Changes = append(c.r.Changes, ch) }
func (c *cmp) find(sev, entry, rule, msg string) {
	c.r.Findings = append(c.r.Findings, Finding{Severity: sev, Entry: entry, Rule: rule, Message: msg})
}

func (c *cmp) legacy(b, n obj) {
	for _, k := range []string{"base_url", "timeout_ms", "user_agent"} {
		if !reflect.DeepEqual(b[k], n[k]) && b != nil && len(b) > 0 {
			c.add(Change{"legacy", "legacy." + k, "changed", fmt.Sprintf("%v → %v", b[k], n[k])})
		}
	}
	br, nr := asObj(b["reach"]), asObj(n["reach"])
	if len(b) > 0 && !reflect.DeepEqual(br, nr) {
		c.add(Change{"legacy", "legacy.reach", "changed", fmt.Sprintf("%v → %v", br, nr)})
		c.find("medium", "legacy.reach", "reach_changed", "The gateway will connect somewhere else: check connector, address and allowed destinations.")
	}
	// direct mode is a normal choice (a site on a public address that accepts
	// only Equalizer's): worth a look only when this version switches to it
	if str(nr["mode"]) == "direct" && str(br["mode"]) != "direct" {
		c.find("medium", "legacy.reach", "direct_mode", "This version switches to direct mode: the gateway reaches the site at its address, without a connector. Check that the site accepts only Equalizer's addresses (Isolation tab).")
	}
	if len(asList(nr["allow"])) > 1 && len(asList(nr["allow"])) > len(asList(br["allow"])) {
		c.find("medium", "legacy.reach.allow", "allow_widened", "More destinations allowed: each one is reachable through this map.")
	}
}

func (c *cmp) roles(b, n obj) {
	c.login(b, n)
	if len(b) > 0 && !reflect.DeepEqual(b, n) {
		c.add(Change{"roles", "roles", "changed", ""})
		if str(b["anonymous"]) != str(n["anonymous"]) {
			c.find("medium", "roles.anonymous", "anonymous_changed", "The role of visitors who are not logged in changes: every entry open to it changes audience.")
		}
	}
}

func (c *cmp) defaults(b, n obj) {
	if len(b) == 0 || reflect.DeepEqual(b, n) {
		return
	}
	c.add(Change{"defaults", "defaults", "changed", ""})
	if limitLooser(asObj(b["rate_limit"]), asObj(n["rate_limit"])) {
		c.find("medium", "defaults.rate_limit", "rate_limit_raised", "Default rate limits are higher or gone.")
	}
	if num(path(n, "string", "max_length")) > num(path(b, "string", "max_length")) && num(path(b, "string", "max_length")) > 0 {
		c.find("low", "defaults.string", "string_longer", "Default text length is longer.")
	}
}

func (c *cmp) typeDefs(b, n obj) {
	for name, nv := range n {
		bv, ok := b[name]
		switch {
		case !ok:
			if len(b) > 0 {
				c.add(Change{"types", name, "added", ""})
			}
		case !reflect.DeepEqual(bv, nv):
			c.add(Change{"types", name, "changed", ""})
			c.field("types."+name, asObj(bv), asObj(nv), false)
		}
	}
	for name := range b {
		if _, ok := n[name]; !ok {
			c.add(Change{"types", name, "removed", ""})
		}
	}
}

func (c *cmp) entries(sec string, b, n map[string]obj) {
	for _, id := range keys(n) {
		e := n[id]
		label := sec + "." + id
		old, ok := b[id]
		if !ok {
			c.add(Change{sec, id, "added", where(e)})
			c.newEntry(sec, label, e)
			continue
		}
		if reflect.DeepEqual(old, e) {
			continue
		}
		c.add(Change{sec, id, "changed", where(e)})
		if where(old) != where(e) {
			c.find("medium", label, "path_changed", fmt.Sprintf("Its public path changes from %s to %s.", where(old), where(e)))
		}
		if opens(asList(old["roles"]), asList(e["roles"]), c.anon) {
			c.find("medium", label, "opened_to_anonymous", "Now open to visitors who are not logged in.")
		}
		for _, part := range []string{"params", "query", "fields", "body"} {
			bo, no := asObj(old[part]), asObj(e[part])
			for f, nv := range no {
				if bv, ok := bo[f]; ok {
					c.field(label+"."+part+"."+f, asObj(bv), asObj(nv), true)
				} else {
					c.newField(label+"."+part+"."+f, asObj(nv))
				}
			}
		}
		if rf := asObj(path(e, "response", "fields")); len(rf) > len(asObj(path(old, "response", "fields"))) {
			c.find("low", label, "response_wider", "The API returns more fields: check none is personal or internal data.")
		}
		if ni := asObj(path(e, "response", "issues")); len(ni) > len(asObj(path(old, "response", "issues"))) {
			c.find("medium", label, "issues_added",
				"The answer now grants access to more objects (response.issues): check it lists only the user's own.")
		}
		if old["authz"] != nil && e["authz"] == nil {
			c.find("high", label, "authz_removed", "Its authorisation rule was removed: anyone with the role can open any object.")
		}
		if limitLooser(asObj(old["rate_limit"]), asObj(e["rate_limit"])) {
			c.find("medium", label, "rate_limit_raised", "Its rate limit is higher or gone.")
		}
	}
	for _, id := range keys(b) {
		if _, ok := n[id]; !ok {
			c.add(Change{sec, id, "removed", where(b[id])})
			c.find("low", sec+"."+id, "removed", fmt.Sprintf("Removed: %s will answer 404.", where(b[id])))
		}
	}
}

func (c *cmp) newEntry(sec, label string, e obj) {
	p := where(e)
	if sensitive.MatchString(p) {
		c.find("high", label, "sensitive_path", fmt.Sprintf("%s looks like an admin, backup, export or debug path.", p))
	}
	if strings.Contains(p, "{") && personal.MatchString(p) && e["authz"] == nil {
		c.find("medium", label, "personal_without_authz",
			fmt.Sprintf("%s may show one person's data: an authorisation rule (owned_by_user) stops users opening others' objects.", p))
	}
	for _, part := range []string{"params", "query", "fields", "body"} {
		for f, spec := range asObj(e[part]) {
			c.newField(label+"."+part+"."+f, asObj(spec))
		}
	}
	if sec == "forms" || (sec == "api" && str(e["method"]) != "GET") {
		if e["rate_limit"] == nil {
			c.find("low", label, "no_rate_limit", "A write without its own rate limit: only the default applies.")
		}
	}
	for _, spec := range asObj(e["fields"]) {
		if str(asObj(spec)["type"]) == "password" {
			c.find("medium", label, "login_form", "A login form: keep a strict rate limit against password guessing.")
			break
		}
	}
	if sec == "api" {
		c.find("low", label, "new_api", "A new API: check that its response fields contain no personal or internal data.")
	}
}

func (c *cmp) newField(label string, spec obj) {
	t := c.base(spec)
	if t == "string" && str(spec["pattern"]) == "" && str(spec["charset"]) == "" && c.typeOf(spec, "pattern") == "" && num(spec["max_length"]) > 500 {
		c.find("low", label, "free_text", "Long free text without a pattern.")
	}
}

// field compares two specs of the same field and flags any widening.
func (c *cmp) field(label string, b, n obj, isField bool) {
	bt, nt := c.baseWith(b, c.oldTypes), c.base(n)
	switch {
	case narrow[bt] && !narrow[nt]:
		c.find("high", label, "type_widened", fmt.Sprintf("Type widened from %s to %s: values that were refused will pass.", bt, nt))
	case bt == "enum" && nt == "enum" && len(asList(n["values"])) > len(asList(b["values"])):
		c.find("low", label, "enum_widened", "More values allowed.")
	}
	if num(n["max_length"]) > 2*num(b["max_length"]) && num(b["max_length"]) > 0 {
		c.find("medium", label, "longer", fmt.Sprintf("Maximum length from %v to %v.", b["max_length"], n["max_length"]))
	}
	for _, k := range []string{"pattern", "charset"} {
		if str(b[k]) != "" && str(n[k]) == "" {
			c.find("medium", label, k+"_removed", "Its "+k+" was removed.")
		}
	}
	if len(asList(b["rules"])) > len(asList(n["rules"])) {
		c.find("high", label, "rules_removed", "Text rules were removed (for example no_html).")
	}
	if len(asList(b["deny"])) > len(asList(n["deny"])) {
		c.find("high", label, "deny_removed", "Refused values were removed: admin names may pass again.")
	}
	if num(n["max"]) > num(b["max"]) && b["max"] != nil {
		c.find("low", label, "range_wider", "A larger maximum.")
	}
	_ = isField
}

func (c *cmp) assets(b, n map[string]obj) {
	for _, p := range keys(n) {
		e := n[p]
		old, ok := b[p]
		if ok && reflect.DeepEqual(old, e) {
			continue
		}
		change := "added"
		if ok {
			change = "changed"
		}
		c.add(Change{"assets", p, change, strings.Join(strs(e["types"]), ", ")})
		for _, t := range strs(e["types"]) {
			if t == "text/html" || strings.Contains(t, "php") || strings.HasSuffix(t, "/*") || t == "application/octet-stream" {
				c.find("high", "assets."+p, "asset_type", fmt.Sprintf("Files of type %s would be served as they are, without rebuilding.", t))
			}
		}
		if sensitive.MatchString(p) {
			c.find("high", "assets."+p, "sensitive_path", p+" looks like an admin or backup folder.")
		}
	}
	for _, p := range keys(b) {
		if _, ok := n[p]; !ok {
			c.add(Change{"assets", p, "removed", ""})
		}
	}
}

func (c *cmp) base(spec obj) string { return c.baseWith(spec, c.types) }

func (c *cmp) baseWith(spec obj, types map[string]obj) string {
	t := str(spec["type"])
	if t == "" {
		t = str(spec["base"]) // a type definition
	}
	if def, ok := types[t]; ok {
		return str(def["base"])
	}
	return t
}

func (c *cmp) typeOf(spec obj, key string) string {
	if def, ok := c.types[str(spec["type"])]; ok {
		return str(def[key])
	}
	return ""
}
