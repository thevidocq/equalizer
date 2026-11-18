// Package router trova la voce della mappa corrispondente a una richiesta.
// Deny by default: ciò che non corrisponde a una voce non esiste.
package router

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Kind distingue rotte di lettura, form e API.
type Kind string

const (
	KindRoute Kind = "route"
	KindForm  Kind = "form"
	KindAPI   Kind = "api"
	KindAsset Kind = "asset"
)

// Entry è una voce della mappa da instradare.
type Entry struct {
	ID      string
	Kind    Kind
	Method  string
	Pattern string // es. /pratiche/{id}; per KindAsset un prefisso, es. /css/
}

// Match è il risultato di un instradamento riuscito.
type Match struct {
	Entry  Entry
	Params map[string]string // valori grezzi: la validazione dei tipi è a valle
}

// Errori di instradamento; il chiamante li traduce in 404 e 405.
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
	param   string // non vuoto se il segmento è {param}
}

// Router contiene le voci compilate. È immutabile dopo New.
type Router struct {
	entries []compiled
	assets  []Entry // prefissi, dal più lungo al più corto
}

// New compila le voci. Rifiuta pattern malformati o duplicati.
func New(entries []Entry) (*Router, error) {
	r := &Router{}
	seen := map[string]string{}
	for _, e := range entries {
		if e.Kind == KindAsset {
			if !strings.HasPrefix(e.Pattern, "/") || !strings.HasSuffix(e.Pattern, "/") || strings.Contains(e.Pattern, "..") {
				return nil, fmt.Errorf("prefisso asset non valido %q", e.Pattern)
			}
			r.assets = append(r.assets, e)
			continue
		}
		segs, params, err := compile(e.Pattern)
		if err != nil {
			return nil, fmt.Errorf("voce %q: %w", e.ID, err)
		}
		key := e.Method + " " + shape(segs)
		if other, dup := seen[key]; dup {
			return nil, fmt.Errorf("voci %q e %q in conflitto su %s %s", other, e.ID, e.Method, e.Pattern)
		}
		seen[key] = e.ID
		r.entries = append(r.entries, compiled{entry: e, segments: segs, params: params})
	}
	sort.Slice(r.assets, func(i, j int) bool { return len(r.assets[i].Pattern) > len(r.assets[j].Pattern) })
	return r, nil
}

// Match trova la voce più specifica (meno parametri) per metodo e percorso.
// HEAD usa le voci GET.
func (r *Router) Match(method, path string) (Match, error) {
	if method == http.MethodHead {
		method = http.MethodGet
	}
	parts := split(path)
	var (
		best      *compiled
		bestVals  map[string]string
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
		if best == nil || c.params < best.params {
			best, bestVals = c, vals
		}
	}
	switch {
	case best != nil:
		return Match{Entry: best.entry, Params: bestVals}, nil
	case pathFound:
		return Match{}, ErrMethodNotAllowed
	}
	for _, a := range r.assets {
		if strings.HasPrefix(path, a.Pattern) && len(path) > len(a.Pattern) {
			if method != http.MethodGet {
				return Match{}, ErrMethodNotAllowed
			}
			return Match{Entry: a, Params: map[string]string{"file": path[len(a.Pattern):]}}, nil
		}
	}
	return Match{}, ErrNotMapped
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
		return nil, 0, errors.New("il percorso deve iniziare con /")
	}
	var segs []segment
	params := 0
	for _, p := range split(pattern) {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			name := p[1 : len(p)-1]
			if name == "" {
				return nil, 0, errors.New("parametro senza nome")
			}
			segs = append(segs, segment{param: name})
			params++
			continue
		}
		if strings.ContainsAny(p, "{}") {
			return nil, 0, fmt.Errorf("segmento non valido %q", p)
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

// split divide il percorso in segmenti; "/" diventa zero segmenti.
func split(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
