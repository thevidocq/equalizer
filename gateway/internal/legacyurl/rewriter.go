// Package legacyurl traduce gli URL del legacy negli URL pubblici del
// gateway. Lo usano la pipeline di resa (link nelle pagine) e la
// traduzione dei redirect.
//
// Fonti dei modelli, in ordine di priorità:
//  1. links delle rotte e compat: "/percorso?chiave={nome}"
//  2. legacy_request delle rotte GET: "/percorso" con query "{{params.x}}"
//
// Un URL del legacy corrisponde a un modello solo se percorso e chiavi di
// query coincidono esattamente: chiavi in più = nessuna corrispondenza.
package legacyurl

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// Rewriter traduce URL del legacy in URL pubblici.
type Rewriter struct {
	base     *url.URL
	basePath string
	patterns []pattern
	assets   []string // prefissi degli asset
}

type pattern struct {
	route    *mapmodel.Route
	segments []part          // segmenti del percorso del legacy
	query    map[string]part // chiavi di query del legacy
}

// part è un valore fisso o una cattura verso params/query della rotta.
type part struct {
	literal string
	ns      string // "params" o "query"; vuoto se fisso
	name    string
}

// New compila i modelli di una mappa.
func New(base *url.URL, m *mapmodel.Model) (*Rewriter, error) {
	r := &Rewriter{base: base, basePath: strings.TrimSuffix(base.Path, "/")}
	for _, a := range m.Assets {
		r.assets = append(r.assets, a.Prefix)
	}
	for i := range m.Routes {
		for _, l := range m.Routes[i].Links {
			if err := r.addBraces(m, l.Legacy, l.Route); err != nil {
				return nil, fmt.Errorf("rotta %q, link %q: %w", m.Routes[i].ID, l.Legacy, err)
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

// Internal dice se u (già risolto in assoluto) punta al legacy.
func (r *Rewriter) Internal(u *url.URL) bool {
	return strings.EqualFold(u.Host, r.base.Host) && (u.Scheme == "http" || u.Scheme == "https")
}

// Public restituisce l'URL pubblico (percorso e query) per un URL interno.
// ok=false se nessun modello corrisponde.
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
			// Gli asset hanno lo stesso percorso pubblico e legacy.
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
	return "", false
}

// Fallback restituisce il percorso relativo di un URL interno non mappato:
// l'host interno del legacy non deve mai comparire nella pagina.
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

// build compone l'URL pubblico della rotta con i valori catturati.
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

// addBraces aggiunge un modello in formato "/percorso?k={nome}".
func (r *Rewriter) addBraces(m *mapmodel.Model, legacy, routeID string) error {
	route, ok := m.RouteByID(routeID)
	if !ok {
		return fmt.Errorf("rotta %q inesistente", routeID)
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
			return part{}, fmt.Errorf("{%s} non è un parametro né una query della rotta", name)
		}
		if strings.ContainsAny(v, "{}") {
			return part{}, fmt.Errorf("segnaposto malformato %q", v)
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

// fromLegacyRequest ricava il modello inverso di legacy_request.
// Valori misti (es. "p-{{params.id}}") non sono invertibili: la rotta si salta.
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
