// Package templates holds starting maps for common CMS and generic sites.
// A template is a normal map with four markers: __BASE_URL__,
// __CONNECTOR__ and __ADDRESS__ are filled in for the site; _template
// carries its name and description and is removed.
package templates

import (
	"embed"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

//go:embed maps/*.json
var files embed.FS

// Template describes one starting map.
type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ErrUnknown reports a template that does not exist.
var ErrUnknown = errors.New("unknown template")

// List returns the templates, generic last.
func List() []Template {
	entries, _ := files.ReadDir("maps")
	out := make([]Template, 0, len(entries))
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".json")
		raw, _ := files.ReadFile("maps/" + e.Name())
		var doc struct {
			T Template `json:"_template"`
		}
		_ = json.Unmarshal(raw, &doc)
		doc.T.ID = id
		out = append(out, doc.T)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].ID == "generic") != (out[j].ID == "generic") {
			return out[j].ID == "generic"
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Render fills a template for a site. The values must already be valid:
// they are placed in JSON strings, never interpreted.
func Render(id, baseURL, connector, address string) (json.RawMessage, error) {
	if strings.ContainsAny(id, "/.\\") {
		return nil, ErrUnknown
	}
	raw, err := files.ReadFile("maps/" + id + ".json")
	if err != nil {
		return nil, ErrUnknown
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	delete(doc, "_template")
	fill(doc, map[string]string{"__BASE_URL__": baseURL, "__CONNECTOR__": connector, "__ADDRESS__": address})
	return json.Marshal(doc)
}

func fill(v any, with map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = fill(e, with)
		}
	case []any:
		for i, e := range x {
			x[i] = fill(e, with)
		}
	case string:
		if r, ok := with[x]; ok {
			return r
		}
	}
	return v
}
