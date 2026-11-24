// Package securityscan follows a site's security scans and reads their
// findings against the site's published map.
package securityscan

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Verdicts, as in the API contract (SecurityMapVerdict).
const (
	Refused  = "refused"   // the gateway refuses the request
	Precise  = "precise"   // a precise entry serves it and checks the input by its type
	FreeText = "free_text" // a precise entry serves it; the input is free text
	Area     = "area"      // only the generic area serves it
	Asset    = "asset"     // a static file
	Server   = "server"    // not about an address (ports, TLS, headers)
)

// Verdict is how the map treats a finding's address.
type Verdict struct {
	Verdict   string  `json:"verdict"`
	Entry     *string `json:"entry"`
	FieldType *string `json:"field_type"`
	list      string  // the document's list of the entry: routes, forms or api
}

type field struct {
	Type    string   `json:"type"`
	Pattern string   `json:"pattern"`
	Values  []string `json:"values"`
}

type typeDef struct {
	Base    string   `json:"base"`
	Pattern string   `json:"pattern"`
	Values  []string `json:"values"`
}

type entry struct {
	ID     string           `json:"id"`
	Path   string           `json:"path"`
	Method string           `json:"method"`
	Route  string           `json:"route"` // forms: the path they post to
	Params map[string]field `json:"params"`
	Query  map[string]field `json:"query"`
	Fields map[string]field `json:"fields"` // forms
	Body   map[string]field `json:"body"`   // api
	Authz  json.RawMessage  `json:"authz"`
}

func (e *entry) guarded() bool { return len(e.Authz) > 0 && string(e.Authz) != "null" }

type area struct {
	ID      string   `json:"id"`
	Prefix  string   `json:"prefix"`
	Methods []string `json:"methods"`
}

type asset struct {
	Prefix string `json:"prefix"`
}

// Map is the part of a published map the verdict reads.
type Map struct {
	Types  map[string]typeDef `json:"types"`
	Routes []entry            `json:"routes"`
	Forms  []entry            `json:"forms"`
	API    []entry            `json:"api"`
	Areas  []area             `json:"areas"`
	Assets []asset            `json:"assets"`
}

// ParseMap reads a map document.
func ParseMap(doc json.RawMessage) (*Map, error) {
	var m Map
	if err := json.Unmarshal(doc, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Judge says how the gateway treats a request to rawURL with method, for
// the parameter the scanner names (empty: none). It follows the gateway's
// order (gateway/internal/server/public.go): the precise entries the path
// has the shape of, as legacy servers read it; the first whose path values
// fit; the longest area for what they do not serve, for path values that
// do not fit and for a query a route does not declare. Values are not
// checked beyond the path: the verdict says which entry checks the
// parameter and how, not whether one value passes. A nil map refuses all.
func (m *Map) Judge(rawURL, method, parameter string) Verdict {
	if rawURL == "" {
		return Verdict{Verdict: Server}
	}
	u, err := url.Parse(rawURL)
	if m == nil || err != nil {
		return Verdict{Verdict: Refused}
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	key := pathKey(path)
	a := m.areaFor(key)
	cands := m.candidates(method, key)
	if len(cands) == 0 { // not mapped, or not for this method: the area's, if any
		if a != nil && allows(a.Methods, method) {
			return areaVerdict(a)
		}
		return Verdict{Verdict: Refused}
	}
	c, fits := m.firstFit(cands)
	if a != nil && (!fits || !c.queryFits(u.RawQuery)) {
		if !fits && unsafeValues(c.vals) {
			return refusedBy(c.e.ID)
		}
		if !allows(a.Methods, method) {
			return Verdict{Verdict: Refused}
		}
		return areaVerdict(a)
	}
	if c.kind == kAsset {
		return Verdict{Verdict: Asset}
	}
	if !fits || c.e.guarded() && !m.valuesFit(c) {
		return refusedBy(c.e.ID) // the entry's own check refuses the value
	}
	return m.input(c, parameter)
}

func areaVerdict(a *area) Verdict {
	id := a.ID
	return Verdict{Verdict: Area, Entry: &id}
}

func refusedBy(id string) Verdict {
	return Verdict{Verdict: Refused, Entry: &id}
}

// input finds the parameter among the entry's inputs. One the entry does
// not declare is refused (deny by default).
func (m *Map) input(c candidate, parameter string) Verdict {
	id := c.e.ID
	if parameter == "" {
		return Verdict{Verdict: Precise, Entry: &id}
	}
	for _, g := range []map[string]field{c.e.Params, c.e.Query, c.e.Fields, c.e.Body} {
		f, ok := g[parameter]
		if !ok {
			continue
		}
		t := f.Type
		if t == "" {
			t = "string"
		}
		v := Verdict{Verdict: Precise, Entry: &id, FieldType: &t, list: c.kind.list()}
		if m.free(f) {
			v.Verdict = FreeText
		}
		return v
	}
	return refusedBy(id)
}

// resolve gives a field's base, pattern and values, with its named type's.
func (m *Map) resolve(f field) (string, string, []string) {
	base, pattern, values := f.Type, f.Pattern, f.Values
	if def, ok := m.Types[f.Type]; ok {
		base = def.Base
		if pattern == "" {
			pattern = def.Pattern
		}
		if len(values) == 0 {
			values = def.Values
		}
	}
	if base == "" {
		base = "string"
	}
	return base, pattern, values
}

// free reports whether a field is text without a pattern or list of values:
// the gateway checks its length, markup and rules, not its shape.
func (m *Map) free(f field) bool {
	base, pattern, values := m.resolve(f)
	return (base == "string" || base == "text") && pattern == "" && len(values) == 0
}

// areaFor is the area with the longest prefix of key ("/x" falls in "/x/").
func (m *Map) areaFor(key string) *area {
	var best *area
	for i := range m.Areas {
		a := &m.Areas[i]
		if strings.HasPrefix(key, a.Prefix) || strings.HasPrefix(key+"/", a.Prefix) && len(key)+1 == len(a.Prefix) {
			if best == nil || len(a.Prefix) > len(best.Prefix) {
				best = a
			}
		}
	}
	return best
}

func allows(methods []string, method string) bool {
	if len(methods) == 0 {
		return method == "GET" || method == "HEAD" || method == "POST"
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) || strings.EqualFold(m, "GET") && method == "HEAD" {
			return true
		}
	}
	return false
}
