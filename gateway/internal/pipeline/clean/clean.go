// Package clean toglie dalla pagina ciò che non si vede ma informa chi
// attacca: commenti HTML e meta che rivelano il software del legacy.
package clean

import (
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// Step è il passo di pulizia invisibile.
type Step struct{}

// Name restituisce il nome del passo.
func (Step) Name() string { return "clean" }

// meta con questi nomi rivelano prodotto e versione del legacy.
var revealing = map[string]bool{"generator": true, "application-name": true}

// Apply rimuove commenti e meta rivelatori.
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
