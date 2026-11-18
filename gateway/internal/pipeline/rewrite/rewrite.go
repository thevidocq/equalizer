// Package rewrite riscrive gli URL della pagina verso il gateway: link,
// risorse, form e srcset. L'host interno del legacy non compare mai.
package rewrite

import (
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// Step è il passo di riscrittura degli URL.
type Step struct{}

// Name restituisce il nome del passo.
func (Step) Name() string { return "rewrite" }

// Attributi che contengono un URL (object usa "data", gestito a parte).
var urlAttrs = map[string]bool{
	"href": true, "src": true, "action": true, "formaction": true,
	"poster": true, "background": true, "cite": true, "longdesc": true,
}

// Schemi che non puntano al legacy e restano invariati.
var keepSchemes = []string{"javascript:", "mailto:", "tel:", "data:", "about:", "blob:", "sms:"}

// Apply riscrive tutti gli URL interni.
func (Step) Apply(doc *html.Node, ctx *pipeline.Context) error {
	base := ctx.PageURL
	// <base href> cambia la risoluzione: lo si applica e poi lo si toglie,
	// perché dopo la riscrittura tutti gli URL sono percorsi assoluti.
	var bases []*html.Node
	pipeline.Walk(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.DataAtom == atom.Base {
			if href, ok := attr(n, "href"); ok && len(bases) == 0 {
				if u, err := url.Parse(strings.TrimSpace(href)); err == nil {
					base = base.ResolveReference(u)
				}
			}
			bases = append(bases, n)
		}
		return true
	})
	for _, n := range bases {
		n.Parent.RemoveChild(n)
		ctx.Stats.Removed++
	}

	pipeline.Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		for i, a := range n.Attr {
			key := strings.ToLower(a.Key)
			switch {
			case urlAttrs[key] || (key == "data" && n.DataAtom == atom.Object):
				n.Attr[i].Val = rewriteOne(a.Val, base, ctx)
			case key == "srcset" || key == "imagesrcset":
				n.Attr[i].Val = rewriteSrcset(a.Val, base, ctx)
			}
		}
		return true
	})
	return nil
}

func rewriteOne(raw string, base *url.URL, ctx *pipeline.Context) string {
	v := strings.TrimSpace(raw)
	if v == "" || strings.HasPrefix(v, "#") {
		return raw
	}
	lower := strings.ToLower(v)
	for _, s := range keepSchemes {
		if strings.HasPrefix(lower, s) {
			return raw
		}
	}
	u, err := url.Parse(v)
	if err != nil {
		return raw
	}
	abs := base.ResolveReference(u)
	if !ctx.URLs.Internal(abs) {
		return raw // esterno: resta com'è
	}
	if public, ok := ctx.URLs.Public(abs); ok {
		ctx.Stats.Rewritten++
		return public
	}
	ctx.Stats.NoteUnmapped(abs)
	return ctx.URLs.Fallback(abs)
}

// rewriteSrcset riscrive "url descrittore, url descrittore".
func rewriteSrcset(raw string, base *url.URL, ctx *pipeline.Context) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		fields[0] = rewriteOne(fields[0], base, ctx)
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", ")
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}
