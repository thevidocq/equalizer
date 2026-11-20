// Package formmap recognises, in a legacy page, the <form> elements that
// post to a form declared in the map.
//
// A legacy form is identified by its action URL (path and query) and by its
// hidden inputs: the legacy_request of the declared form is inverted, so
// "/permits/supplement.do" with body {permitId: "{{params.id}}"} matches a
// form posting to that path with a hidden input permitId=42, and gives the
// public action "/permits/42/supplement".
package formmap

import (
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
)

// Match is a legacy form recognised as a declared form.
type Match struct {
	FormID string
	Action string            // public action URL
	Params map[string]string // path parameters of the public action
	Hidden map[string]string // hidden values the map keeps ({{hidden.x}})
	Rename map[string]string // legacy input name → public field name
}

// Map holds the compiled forms of one map version. It is immutable.
type Map struct {
	base     *url.URL
	basePath string
	forms    []compiled
}

type compiled struct {
	form     *mapmodel.Form
	segments []part
	query    map[string]part
	body     map[string]part // legacy body key → part
	public   []string        // public path segments
}

// part is a fixed value or a capture into a namespace (params, hidden, fields).
type part struct {
	literal string
	ns      string
	name    string
}

// New compiles the forms of a map. Forms whose legacy_request cannot be
// inverted (mixed values in the path, for instance) are skipped: they are
// not recognised in pages.
func New(base *url.URL, m *mapmodel.Model) *Map {
	fm := &Map{base: base, basePath: strings.TrimSuffix(base.Path, "/")}
	for i := range m.Forms {
		if c, ok := compile(&m.Forms[i]); ok {
			fm.forms = append(fm.forms, c)
		}
	}
	return fm
}

func compile(f *mapmodel.Form) (compiled, bool) {
	toPart := func(tpl string) (part, bool) {
		if !strings.Contains(tpl, "{{") {
			return part{literal: tpl}, true
		}
		ns, name, ok := placeholder.Single(tpl)
		if !ok {
			return part{}, false
		}
		return part{ns: ns, name: name}, true
	}
	c := compiled{form: f, query: map[string]part{}, body: map[string]part{}, public: split(f.Route)}
	for _, s := range split(f.LegacyRequest.Path) {
		p, ok := toPart(s)
		if !ok || (p.ns != "" && p.ns != "params") {
			return compiled{}, false
		}
		c.segments = append(c.segments, p)
	}
	for k, v := range f.LegacyRequest.Query {
		// params: part of the record the form acts on; hidden: a value the
		// page put in the action (phpBB's sid), kept server-side like a
		// hidden input and sent back as it was.
		p, ok := toPart(v)
		if !ok || (p.ns != "" && p.ns != "params" && p.ns != "hidden") {
			return compiled{}, false
		}
		c.query[k] = p
	}
	for k, v := range f.LegacyRequest.Form {
		if p, ok := toPart(v); ok {
			c.body[k] = p
		}
	}
	return c, true
}

// Match recognises a legacy form from its absolute action URL and its hidden
// inputs (name → value).
func (m *Map) Match(action *url.URL, hidden map[string]string) (Match, bool) {
	if !strings.EqualFold(action.Host, m.base.Host) {
		return Match{}, false
	}
	path := action.Path
	if m.basePath != "" {
		if !strings.HasPrefix(path, m.basePath+"/") {
			return Match{}, false
		}
		path = strings.TrimPrefix(path, m.basePath)
	}
	segs := split(path)
	query := action.Query()
	for _, c := range m.forms {
		if out, ok := c.match(segs, query, hidden); ok {
			return out, true
		}
	}
	return Match{}, false
}

func (c compiled) match(segs []string, query url.Values, hidden map[string]string) (Match, bool) {
	if len(segs) != len(c.segments) {
		return Match{}, false
	}
	out := Match{FormID: c.form.ID, Params: map[string]string{}, Hidden: map[string]string{}, Rename: map[string]string{}}
	for i, p := range c.segments {
		if p.ns == "" {
			if segs[i] != p.literal {
				return Match{}, false
			}
			continue
		}
		out.Params[p.name] = segs[i]
	}
	for k, vals := range query {
		p, ok := c.query[k]
		if !ok || len(vals) != 1 {
			return Match{}, false
		}
		switch p.ns {
		case "":
			if vals[0] != p.literal {
				return Match{}, false
			}
			continue
		case "hidden":
			out.Hidden[p.name] = vals[0]
			continue
		}
		out.Params[p.name] = vals[0]
	}
	for k, p := range c.query {
		if p.ns == "" && !query.Has(k) {
			return Match{}, false
		}
	}
	for key, p := range c.body {
		switch p.ns {
		case "params":
			v, ok := hidden[key]
			if !ok {
				return Match{}, false
			}
			out.Params[p.name] = v
		case "hidden":
			if v, ok := hidden[p.name]; ok {
				out.Hidden[p.name] = v
			}
		case "fields":
			if key != p.name {
				out.Rename[key] = p.name
			}
		}
	}
	action, ok := c.publicAction(out.Params)
	if !ok {
		return Match{}, false
	}
	out.Action = action
	return out, true
}

func (c compiled) publicAction(params map[string]string) (string, bool) {
	for name := range c.form.Params {
		if params[name] == "" {
			return "", false
		}
	}
	var b strings.Builder
	for _, seg := range c.public {
		b.WriteByte('/')
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			v := params[seg[1:len(seg)-1]]
			if v == "" {
				return "", false
			}
			b.WriteString(url.PathEscape(v))
			continue
		}
		b.WriteString(seg)
	}
	if b.Len() == 0 {
		return "/", true
	}
	return b.String(), true
}

func split(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
