// Package forms binds the page's POST forms to the forms declared in the map.
//
// For every <form method="post">:
//   - all hidden inputs are removed: the values the map needs ({{hidden.x}}
//     and path parameters) are kept server-side, inside the form token;
//   - if the form matches a declared form, its action becomes the public
//     action, its inputs take the public field names and a single hidden
//     input carries the sealed token;
//   - otherwise the action is left to the rewrite step, which makes it a
//     relative path the gateway rejects.
//
// GET forms (searches) are left to the rewrite step: they behave as links.
// The step must run before rewrite, because it needs the legacy action URL.
package forms

import (
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// Step is the form binding step.
type Step struct{}

// Name returns the step name.
func (Step) Name() string { return "forms" }

// Apply binds every POST form of the page.
func (Step) Apply(doc *html.Node, ctx *pipeline.Context) error {
	base := pipeline.BaseURL(doc, ctx.PageURL)
	var forms []*html.Node
	pipeline.Walk(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.DataAtom == atom.Form {
			forms = append(forms, n)
		}
		return true
	})
	for _, f := range forms {
		if err := bind(f, base, ctx); err != nil {
			return err
		}
	}
	return nil
}

func bind(form *html.Node, base *url.URL, ctx *pipeline.Context) error {
	method, _ := pipeline.Attr(form, "method")
	if !strings.EqualFold(strings.TrimSpace(method), "post") {
		return nil
	}
	// Hidden values are taken away only from forms the gateway binds (it
	// keeps them server-side); any other form, e.g. one served by an area,
	// keeps them and works as on the legacy site.
	hidden, hiddenNodes := collectHidden(form)

	rawAction, _ := pipeline.Attr(form, "action")
	action, err := url.Parse(strings.TrimSpace(rawAction))
	if err != nil || ctx.Forms == nil {
		ctx.Stats.FormsUnmapped++
		return nil
	}
	abs := base.ResolveReference(action)
	m, ok := ctx.Forms.Match(abs, hidden)
	if !ok {
		if _, served := ctx.URLs.Public(abs); served && ctx.URLs.Internal(abs) {
			return nil // posts into an area: served as it is, hidden values included
		}
		for _, n := range hiddenNodes { // posts nowhere: nothing to keep in the page
			n.Parent.RemoveChild(n)
			ctx.Stats.Removed++
		}
		ctx.Stats.FormsUnmapped++
		if len(ctx.Stats.UnmappedPaths) < 20 {
			ctx.Stats.UnmappedPaths = append(ctx.Stats.UnmappedPaths, abs.Path)
		}
		return nil
	}
	token, err := ctx.Forms.Token(m, ctx.Stats.Charset)
	if err != nil {
		return err
	}
	for _, n := range hiddenNodes {
		n.Parent.RemoveChild(n)
		ctx.Stats.Removed++
	}
	pipeline.SetAttr(form, "action", m.Action)
	// The form is re-encoded by the gateway: the browser always submits UTF-8.
	pipeline.DelAttr(form, "accept-charset")
	ctx.MarkFinal(form)
	pipeline.Walk(form, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		// A button's formaction would send the form somewhere else.
		pipeline.DelAttr(n, "formaction")
		if name, ok := pipeline.Attr(n, "name"); ok {
			if public, rename := m.Rename[name]; rename && isControl(n) {
				pipeline.SetAttr(n, "name", public)
			}
		}
		return true
	})
	form.AppendChild(&html.Node{
		Type: html.ElementNode, Data: "input", DataAtom: atom.Input,
		Attr: []html.Attribute{{Key: "type", Val: "hidden"}, {Key: "name", Val: formtoken.Field}, {Key: "value", Val: token}},
	})
	ctx.Stats.Forms++
	return nil
}

// collectHidden returns the form's hidden inputs and their values (the first
// value wins when a name repeats).
func collectHidden(form *html.Node) (map[string]string, []*html.Node) {
	values := map[string]string{}
	var remove []*html.Node
	pipeline.Walk(form, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.DataAtom != atom.Input {
			return true
		}
		typ, _ := pipeline.Attr(n, "type")
		if !strings.EqualFold(strings.TrimSpace(typ), "hidden") {
			return true
		}
		name, _ := pipeline.Attr(n, "name")
		if _, seen := values[name]; name != "" && !seen {
			values[name], _ = pipeline.Attr(n, "value")
		}
		remove = append(remove, n)
		return false
	})
	return values, remove
}

func isControl(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Input, atom.Textarea, atom.Select, atom.Button:
		return true
	}
	return false
}
