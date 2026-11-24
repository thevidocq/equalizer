package securityscan

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// The gateway's routing, as far as a verdict needs it
// (gateway/internal/router, server/strict.go, server/pathfit.go,
// pathkey). panel-api cannot import the gateway's packages (one service per
// folder): tests pin the cases that matter.

type kind int

const (
	kRoute kind = iota
	kForm
	kAPI
	kAsset
)

type candidate struct {
	e    *entry
	kind kind
	vals map[string]string
}

// candidates are the entries the path has the shape of for method, fewest
// parameters first (map order kept), then the longest asset prefix for a
// GET. None when the path is not mapped or not for this method.
func (m *Map) candidates(method, key string) []candidate {
	if method == "HEAD" {
		method = "GET"
	}
	type shaped struct {
		c      candidate
		params int
	}
	var out []shaped
	add := func(e *entry, k kind, pattern, entryMethod string) {
		vals, n, ok := match(pattern, key)
		if ok && strings.EqualFold(entryMethod, method) {
			out = append(out, shaped{candidate{e, k, vals}, n})
		}
	}
	for i := range m.Routes {
		add(&m.Routes[i], kRoute, m.Routes[i].Path, "GET")
	}
	for i := range m.Forms {
		add(&m.Forms[i], kForm, m.Forms[i].Route, "POST")
	}
	for i := range m.API {
		add(&m.API[i], kAPI, m.API[i].Path, m.API[i].Method)
	}
	for i := 1; i < len(out); i++ { // insertion sort: stable
		for j := i; j > 0 && out[j].params < out[j-1].params; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	cands := make([]candidate, 0, len(out)+1)
	for _, s := range out {
		cands = append(cands, s.c)
	}
	if method != "GET" {
		return cands
	}
	var best *asset
	for i := range m.Assets {
		p := m.Assets[i].Prefix
		if strings.HasPrefix(key, p) && len(key) > len(p) && (best == nil || len(p) > len(best.Prefix)) {
			best = &m.Assets[i]
		}
	}
	if best != nil {
		cands = append(cands, candidate{&entry{ID: "asset:" + best.Prefix}, kAsset, nil})
	}
	return cands
}

// firstFit keeps the first candidate whose path values fit their types; an
// entry that decides who opens which object always fits (never bypassed).
func (m *Map) firstFit(cands []candidate) (candidate, bool) {
	pages := cands[0].kind != kAsset
	for i, c := range cands {
		if i > 0 && pages && c.kind == kAsset {
			break
		}
		if c.kind == kAsset || c.e.guarded() || m.valuesFit(c) {
			return c, true
		}
	}
	return cands[0], false
}

// valuesFit reports whether every path value fits its parameter's type.
func (m *Map) valuesFit(c candidate) bool {
	for name, v := range c.vals {
		if !m.fits(c.e.Params[name], v) {
			return false
		}
	}
	return true
}

// queryFits: a route serves a query only when it declares every key.
func (c candidate) queryFits(rawQuery string) bool {
	if c.kind != kRoute || rawQuery == "" || c.e.guarded() {
		return true
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return true
	}
	for k := range q {
		if _, ok := c.e.Query[k]; !ok {
			return false
		}
	}
	return true
}

// fits is the shape check of a path value: numbers, lists and patterns. It
// is what decides between a precise entry and the area; lengths and rules
// are the gateway's.
func (m *Map) fits(f field, raw string) bool {
	v, err := url.PathUnescape(raw)
	if err != nil {
		return false
	}
	base, pattern, values := m.resolve(f)
	switch base {
	case "int", "id":
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return false
		}
	case "enum":
		ok := false
		for _, x := range values {
			ok = ok || x == v
		}
		if !ok {
			return false
		}
	}
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil || !re.MatchString(v) {
			return false
		}
	}
	return true
}

// unsafeValues: path values the gateway refuses even where the area serves
// the path (server/strict.go unsafeInPath).
func unsafeValues(vals map[string]string) bool {
	for _, raw := range vals {
		v, err := url.PathUnescape(raw)
		if err != nil {
			return true
		}
		if strings.ContainsAny(v, "\"<>`{}|\\^") || strings.IndexFunc(v, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			return true
		}
		rs := []rune(v)
		for i, r := range rs {
			if r == '\'' && (i == 0 || i == len(rs)-1 || !unicode.IsLetter(rs[i-1]) || !unicode.IsLetter(rs[i+1])) {
				return true
			}
		}
	}
	return false
}

// match compares a path with a map template: {name} is one non-empty
// segment. It returns the values and the number of parameters.
func match(template, path string) (map[string]string, int, bool) {
	if !strings.HasPrefix(template, "/") {
		return nil, 0, false
	}
	t, p := split(template), split(path)
	if len(t) != len(p) {
		return nil, 0, false
	}
	var vals map[string]string
	n := 0
	for i := range t {
		if strings.HasPrefix(t[i], "{") && strings.HasSuffix(t[i], "}") {
			if p[i] == "" {
				return nil, 0, false
			}
			if vals == nil {
				vals = map[string]string{}
			}
			vals[t[i][1:len(t[i])-1]] = p[i]
			n++
			continue
		}
		if t[i] != p[i] {
			return nil, 0, false
		}
	}
	return vals, n, true
}

// split cuts a path into segments as the gateway's router does: a final
// slash does not count.
func split(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// pathKey is the path as legacy servers read it (gateway/internal/pathkey):
// "//" merged, ";parameters" and trailing dots and spaces dropped.
func pathKey(path string) string {
	segs := strings.Split(path, "/")
	out := make([]string, 0, len(segs))
	for i, s := range segs {
		s, _, _ = strings.Cut(s, ";")
		if t := strings.TrimRight(s, ". "); t != "" {
			s = t
		}
		if s == "" && i != 0 && i != len(segs)-1 {
			continue
		}
		out = append(out, s)
	}
	key := strings.Join(out, "/")
	if strings.HasSuffix(key, "//") {
		key = strings.TrimRight(key, "/") + "/"
	}
	return key
}
