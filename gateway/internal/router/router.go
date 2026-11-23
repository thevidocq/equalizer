// Package router finds the map entry matching a request.
// Deny by default: anything that matches no entry does not exist.
package router

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Kind distinguishes read routes, forms, APIs and asset prefixes.
type Kind string

const (
	KindRoute Kind = "route"
	KindForm  Kind = "form"
	KindAPI   Kind = "api"
	KindAsset Kind = "asset"
)

// Entry is a map entry to route.
type Entry struct {
	ID      string
	Kind    Kind
	Method  string
	Pattern string // e.g. /permits/{id}; for KindAsset a prefix, e.g. /css/
}

// Match is the result of a successful routing.
type Match struct {
	Entry  Entry
	Params map[string]string // raw values: type validation happens downstream
}

// Routing errors; the caller turns them into 404 and 405.
var (
	ErrNotMapped        = errors.New("route_not_mapped")
	ErrMethodNotAllowed = errors.New("method_not_allowed")
)

type compiled struct {
	entry    Entry
	segments []segment
	params   int
}

type segment struct {
	literal string
	param   string // set when the segment is {param}
}

// Router holds the compiled entries. It is immutable after New.
type Router struct {
	entries []compiled
	assets  []Entry // prefixes, longest first
}

// New compiles the entries. It rejects malformed or duplicate patterns.
func New(entries []Entry) (*Router, error) {
	r := &Router{}
	seen := map[string]string{}
	for _, e := range entries {
		if e.Kind == KindAsset {
			if !strings.HasPrefix(e.Pattern, "/") || !strings.HasSuffix(e.Pattern, "/") || strings.Contains(e.Pattern, "..") {
				return nil, fmt.Errorf("invalid asset prefix %q", e.Pattern)
			}
			r.assets = append(r.assets, e)
			continue
		}
		segs, params, err := compile(e.Pattern)
		if err != nil {
			return nil, fmt.Errorf("entry %q: %w", e.ID, err)
		}
		key := e.Method + " " + shape(segs)
		if other, dup := seen[key]; dup {
			return nil, fmt.Errorf("entries %q and %q conflict on %s %s", other, e.ID, e.Method, e.Pattern)
		}
		seen[key] = e.ID
		r.entries = append(r.entries, compiled{entry: e, segments: segs, params: params})
	}
	sort.Slice(r.assets, func(i, j int) bool { return len(r.assets[i].Pattern) > len(r.assets[j].Pattern) })
	// fewest parameters first, map order kept: Candidates needs no sort per request
	sort.SliceStable(r.entries, func(i, j int) bool { return r.entries[i].params < r.entries[j].params })
	return r, nil
}

// Match finds the most specific entry (fewest parameters) for method and
// path. HEAD uses GET entries. Asset prefixes are tried after routes.
func (r *Router) Match(method, path string) (Match, error) {
	c, err := r.Candidates(method, path)
	if err != nil {
		return Match{}, err
	}
	return c[0], nil
}

// Candidates returns every entry the path has the shape of for method, the
// most specific first (fewest parameters), then the asset prefix, if any:
// /wp-includes/js/x.js has the shape of /{year}/{month}/{slug}/ and is a
// file of /wp-includes/. The caller keeps the first whose values fit.
func (r *Router) Candidates(method, path string) ([]Match, error) {
	if method == http.MethodHead {
		method = http.MethodGet
	}
	parts := split(path)
	var (
		out       []Match
		pathFound bool
	)
	for i := range r.entries {
		c := &r.entries[i]
		vals, ok := c.match(parts)
		if !ok {
			continue
		}
		pathFound = true
		if c.entry.Method != method {
			continue
		}
		out = append(out, Match{Entry: c.entry, Params: vals})
	}
	if len(out) == 0 && pathFound {
		return nil, ErrMethodNotAllowed
	}
	for _, a := range r.assets {
		if strings.HasPrefix(path, a.Pattern) && len(path) > len(a.Pattern) {
			if method != http.MethodGet {
				if len(out) == 0 {
					return nil, ErrMethodNotAllowed
				}
				break
			}
			out = append(out, Match{Entry: a, Params: map[string]string{"file": path[len(a.Pattern):]}})
			break
		}
	}
	if len(out) == 0 {
		return nil, ErrNotMapped
	}
	return out, nil
}

func (c *compiled) match(parts []string) (map[string]string, bool) {
	if len(parts) != len(c.segments) {
		return nil, false
	}
	var vals map[string]string
	for i, s := range c.segments {
		if s.param == "" {
			if parts[i] != s.literal {
				return nil, false
			}
			continue
		}
		if parts[i] == "" {
			return nil, false
		}
		if vals == nil {
			vals = map[string]string{}
		}
		vals[s.param] = parts[i]
	}
	return vals, true
}

func compile(pattern string) ([]segment, int, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, 0, errors.New("path must start with /")
	}
	var segs []segment
	params := 0
	for _, p := range split(pattern) {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			name := p[1 : len(p)-1]
			if name == "" {
				return nil, 0, errors.New("unnamed parameter")
			}
			segs = append(segs, segment{param: name})
			params++
			continue
		}
		if strings.ContainsAny(p, "{}") {
			return nil, 0, fmt.Errorf("invalid segment %q", p)
		}
		segs = append(segs, segment{literal: p})
	}
	return segs, params, nil
}

func shape(segs []segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteByte('/')
		if s.param != "" {
			b.WriteString("{}")
		} else {
			b.WriteString(s.literal)
		}
	}
	return b.String()
}

// split splits the path into segments; "/" yields zero segments.
func split(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
