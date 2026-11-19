// Package legacyurl translates legacy URLs into the gateway's public URLs.
// It is used by the rendering pipeline (links in pages) and by redirect
// translation.
//
// Pattern sources, in priority order:
//  1. route links and compat: "/path?key={name}"
//  2. legacy_request of GET routes: "/path" with query "{{params.x}}"
//
// A legacy URL matches a pattern only if path and query keys are exactly the
// same: extra keys mean no match.
package legacyurl

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// Rewriter translates legacy URLs into public URLs.
type Rewriter struct {
	base     *url.URL
	basePath string
	patterns []pattern
	assets   []string // asset prefixes
	areas    []string // area prefixes: same public and legacy path
}

type pattern struct {
	route    *mapmodel.Route
	segments []part          // legacy path segments
	query    map[string]part // legacy query keys
}

// part is a fixed value or a capture into the route's params/query.
type part struct {
	literal string
	ns      string // "params" or "query"; empty when fixed
	name    string
}

// New compiles the patterns of a map.
func New(base *url.URL, m *mapmodel.Model) (*Rewriter, error) {
	r := &Rewriter{base: base, basePath: strings.TrimSuffix(base.Path, "/")}
	for _, a := range m.Assets {
		r.assets = append(r.assets, a.Prefix)
	}
	for _, a := range m.Areas {
		r.areas = append(r.areas, a.Prefix)
	}
	for i := range m.Routes {
		for _, l := range m.Routes[i].Links {
			if err := r.addBraces(m, l.Legacy, l.Route); err != nil {
				return nil, fmt.Errorf("route %q, link %q: %w", m.Routes[i].ID, l.Legacy, err)
			}
		}
	}
	for _, c := range m.Compat {
		if err := r.addBraces(m, c.Legacy, c.Route); err != nil {
			return nil, fmt.Errorf("compat %q: %w", c.Legacy, err)
		}
	}
	for i := range m.Routes {
		if p, ok := fromLegacyRequest(&m.Routes[i]); ok {
			r.patterns = append(r.patterns, p)
		}
	}
	return r, nil
}

// Internal reports whether u (already resolved to absolute) points to the legacy application.
func (r *Rewriter) Internal(u *url.URL) bool {
	return strings.EqualFold(u.Host, r.base.Host) && (u.Scheme == "http" || u.Scheme == "https")
}

// Public returns the public URL (path and query) for an internal URL.
// ok=false when no pattern matches.
func (r *Rewriter) Public(u *url.URL) (string, bool) {
	path := u.Path
	if r.basePath != "" {
		if !strings.HasPrefix(path, r.basePath+"/") && path != r.basePath {
			return "", false
		}
		path = strings.TrimPrefix(path, r.basePath)
	}
	if path == "" {
		path = "/"
	}
	for _, prefix := range r.assets {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			// Assets have the same public and legacy path.
			out := (&url.URL{Path: path, RawQuery: u.RawQuery, Fragment: u.Fragment}).String()
			return out, true
		}
	}
	segs := split(path)
	q := u.Query()
	for _, p := range r.patterns {
		if out, ok := p.apply(segs, q); ok {
			if u.Fragment != "" {
				out += "#" + u.Fragment
			}
			return out, true
		}
	}
	for _, prefix := range r.areas {
		if strings.HasPrefix(path, prefix) || path+"/" == prefix {
			// Areas are served as they are: the same path, the same query.
			return (&url.URL{Path: path, RawQuery: u.RawQuery, Fragment: u.Fragment}).String(), true
		}
	}
	return "", false
}

// Fallback returns the relative path of an unmapped internal URL: the
// legacy's internal host must never appear in the page.
func (r *Rewriter) Fallback(u *url.URL) string {
	out := u.EscapedPath()
	if out == "" {
		out = "/"
	}
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}

func (p pattern) apply(segs []string, q url.Values) (string, bool) {
	if len(segs) != len(p.segments) {
		return "", false
	}
	captured := map[string]map[string]string{"params": {}, "query": {}}
	for i, s := range p.segments {
		if s.ns == "" {
			if segs[i] != s.literal {
				return "", false
			}
			continue
		}
		captured[s.ns][s.name] = segs[i]
	}
	for key, vals := range q {
		spec, ok := p.query[key]
		if !ok || len(vals) != 1 {
			return "", false
		}
		if spec.ns == "" {
			if vals[0] != spec.literal {
				return "", false
			}
			continue
		}
		captured[spec.ns][spec.name] = vals[0]
	}
	for key, spec := range p.query {
		if spec.ns == "" && !q.Has(key) {
			return "", false
		}
	}
	return build(p.route, captured)
}

// build composes the route's public URL from the captured values.
func build(route *mapmodel.Route, captured map[string]map[string]string) (string, bool) {
	var b strings.Builder
	for _, seg := range split(route.Path) {
		b.WriteByte('/')
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			v, ok := captured["params"][seg[1:len(seg)-1]]
			if !ok || v == "" {
				return "", false
			}
			b.WriteString(url.PathEscape(v))
			continue
		}
		b.WriteString(seg)
	}
	out := b.String()
	if out == "" {
		out = "/"
	}
	if len(captured["query"]) > 0 {
		qs := url.Values{}
		keys := make([]string, 0, len(captured["query"]))
		for k := range captured["query"] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			qs.Set(k, captured["query"][k])
		}
		out += "?" + qs.Encode()
	}
	return out, true
}

// addBraces adds a pattern in the "/path?k={name}" format.
func (r *Rewriter) addBraces(m *mapmodel.Model, legacy, routeID string) error {
	route, ok := m.RouteByID(routeID)
	if !ok {
		return fmt.Errorf("route %q does not exist", routeID)
	}
	path, rawQuery, _ := strings.Cut(legacy, "?")
	target := func(v string) (part, error) {
		if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
			name := v[1 : len(v)-1]
			if _, isParam := route.Params[name]; isParam {
				return part{ns: "params", name: name}, nil
			}
			if _, isQuery := route.Query[name]; isQuery {
				return part{ns: "query", name: name}, nil
			}
			return part{}, fmt.Errorf("{%s} is neither a parameter nor a query of the route", name)
		}
		if strings.ContainsAny(v, "{}") {
			return part{}, fmt.Errorf("malformed placeholder %q", v)
		}
		return part{literal: v}, nil
	}
	p := pattern{route: route, query: map[string]part{}}
	for _, s := range split(path) {
		t, err := target(s)
		if err != nil {
			return err
		}
		p.segments = append(p.segments, t)
	}
	if rawQuery != "" {
		for _, kv := range strings.Split(rawQuery, "&") {
			k, v, _ := strings.Cut(kv, "=")
			t, err := target(v)
			if err != nil {
				return err
			}
			p.query[k] = t
		}
	}
	r.patterns = append(r.patterns, p)
	return nil
}

// fromLegacyRequest derives the inverse pattern of legacy_request.
// Mixed values (e.g. "p-{{params.id}}") cannot be inverted: the route is skipped.
func fromLegacyRequest(route *mapmodel.Route) (pattern, bool) {
	lr := route.LegacyRequest
	if !strings.EqualFold(lr.Method, "GET") {
		return pattern{}, false
	}
	toPart := func(v string) (part, bool) {
		if !strings.Contains(v, "{{") {
			return part{literal: v}, true
		}
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, "{{"), "}}"))
		if !strings.HasPrefix(v, "{{") || !strings.HasSuffix(v, "}}") || strings.Contains(inner, "{") {
			return part{}, false
		}
		ns, name, ok := strings.Cut(inner, ".")
		if !ok || (ns != "params" && ns != "query") {
			return part{}, false
		}
		return part{ns: ns, name: name}, true
	}
	p := pattern{route: route, query: map[string]part{}}
	for _, s := range split(lr.Path) {
		t, ok := toPart(s)
		if !ok {
			return pattern{}, false
		}
		p.segments = append(p.segments, t)
	}
	for k, v := range lr.Query {
		t, ok := toPart(v)
		if !ok {
			return pattern{}, false
		}
		p.query[k] = t
	}
	return p, true
}

func split(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
