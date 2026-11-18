// Package pipeline applica la resa fedele alle pagine HTML del legacy:
// converte in UTF-8, analizza l'HTML5, applica una serie di passi
// indipendenti e riserializza la pagina. A video la pagina resta identica.
package pipeline

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/charset"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// URLMapper traduce gli URL del legacy in URL pubblici (legacyurl.Rewriter).
type URLMapper interface {
	Internal(u *url.URL) bool
	Public(u *url.URL) (string, bool)
	Fallback(u *url.URL) string
}

// Context è ciò che un passo conosce della pagina in elaborazione.
type Context struct {
	PageURL *url.URL // URL del legacy da cui viene la pagina
	URLs    URLMapper
	Stats   Stats
}

// Stats riassume cosa ha fatto la pipeline, per log e traccia richiesta.
type Stats struct {
	Charset       string
	Rewritten     int
	Unmapped      int
	Removed       int
	UnmappedPaths []string // al massimo 20, senza query
}

// NoteUnmapped registra un URL interno senza rotta pubblica.
func (s *Stats) NoteUnmapped(u *url.URL) {
	s.Unmapped++
	if len(s.UnmappedPaths) < 20 {
		s.UnmappedPaths = append(s.UnmappedPaths, u.Path)
	}
}

// Step è un passo della resa fedele. Ogni passo modifica l'albero e non
// conosce gli altri.
type Step interface {
	Name() string
	Apply(doc *html.Node, ctx *Context) error
}

// Pipeline è una sequenza configurata di passi.
type Pipeline struct {
	steps []Step
}

// New crea una pipeline con i passi nell'ordine dato.
func New(steps ...Step) *Pipeline {
	return &Pipeline{steps: steps}
}

// ContentType è il tipo dichiarato al client per le pagine elaborate.
const ContentType = "text/html; charset=utf-8"

// Render elabora una pagina HTML e la restituisce in UTF-8.
func (p *Pipeline) Render(body []byte, contentType string, ctx *Context) ([]byte, error) {
	cs, err := charset.Detect(contentType, body)
	if err != nil {
		return nil, err
	}
	ctx.Stats.Charset = cs
	utf8Body, err := charset.ToUTF8(cs, body)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(bytes.NewReader(utf8Body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}
	declareUTF8(doc)
	for _, s := range p.steps {
		if err := s.Apply(doc, ctx); err != nil {
			return nil, fmt.Errorf("passo %s: %w", s.Name(), err)
		}
	}
	var out bytes.Buffer
	if err := html.Render(&out, doc); err != nil {
		return nil, fmt.Errorf("serializzazione: %w", err)
	}
	return out.Bytes(), nil
}

// IsHTML dice se un Content-Type va elaborato dalla pipeline.
func IsHTML(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/xhtml+xml")
}

// Walk visita tutti i nodi in profondità. fn restituisce false per non
// scendere nei figli.
func Walk(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling // fn può rimuovere c
		Walk(c, fn)
		c = next
	}
}

// declareUTF8 allinea le dichiarazioni di codifica alla pagina convertita.
func declareUTF8(doc *html.Node) {
	Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.DataAtom != atom.Meta {
			return true
		}
		for i, a := range n.Attr {
			switch strings.ToLower(a.Key) {
			case "charset":
				n.Attr[i].Val = "utf-8"
			case "content":
				if isContentTypeMeta(n) {
					n.Attr[i].Val = ContentType
				}
			}
		}
		return false
	})
}

func isContentTypeMeta(n *html.Node) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "http-equiv") && strings.EqualFold(strings.TrimSpace(a.Val), "content-type") {
			return true
		}
	}
	return false
}
