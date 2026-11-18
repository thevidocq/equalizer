// Package clean removes from the page what is invisible but informs an
// attacker: HTML comments and meta tags that reveal the legacy software.
package clean

import (
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// Step is the invisible clean-up step.
type Step struct{}

// Name returns the step name.
func (Step) Name() string { return "clean" }

// meta tags with these names reveal the legacy product and version.
var revealing = map[string]bool{"generator": true, "application-name": true}

// Apply removes comments and revealing meta tags.
func (Step) Apply(doc *html.Node, ctx *pipeline.Context) error {
	var drop []*html.Node
	pipeline.Walk(doc, func(n *html.Node) bool {
		switch {
		case n.Type == html.CommentNode:
			drop = append(drop, n)
		case n.Type == html.ElementNode && n.DataAtom == atom.Meta:
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "name") && revealing[strings.ToLower(strings.TrimSpace(a.Val))] {
					drop = append(drop, n)
				}
			}
		}
		return true
	})
	for _, n := range drop {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
			ctx.Stats.Removed++
		}
	}
	return nil
}
