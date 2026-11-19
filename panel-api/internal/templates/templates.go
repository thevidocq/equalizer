// Package templates holds starting maps for common CMS and generic sites.
// A template is a normal map with four markers: __BASE_URL__,
// __CONNECTOR__ and __ADDRESS__ are filled in for the site; _template
// carries its name and description and is removed. The upload size is set
// on every area and on the asset folders that have a size limit.
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

// DefaultUploadMB is the upload size when none is given.
const DefaultUploadMB = 50

// Fill holds the site's values for a template.
type Fill struct {
	BaseURL, Connector, Address string
	MaxUploadMB                 int // 1–1024; 0: DefaultUploadMB
}

// Render fills a template for a site. The values must already be valid:
// they are placed in JSON strings, never interpreted.
func Render(id string, f Fill) (json.RawMessage, error) {
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
	fill(doc, map[string]string{"__BASE_URL__": f.BaseURL, "__CONNECTOR__": f.Connector, "__ADDRESS__": f.Address})
	mb := f.MaxUploadMB
	if mb <= 0 {
		mb = DefaultUploadMB
	}
	setUpload(doc, min(mb, 1024)*1024)
	return json.Marshal(doc)
}

// setUpload sets the largest request body of every area and raises the
// download limit of asset folders (uploaded files are served from there).
func setUpload(doc map[string]any, kb int) {
	areas, _ := doc["areas"].([]any)
	for _, a := range areas {
		if m, ok := a.(map[string]any); ok {
			m["max_body_kb"] = kb
		}
	}
	assets, _ := doc["assets"].([]any)
	for _, a := range assets {
		m, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if cur, ok := m["max_size_kb"].(float64); ok && int(cur) < kb {
			m["max_size_kb"] = kb
		}
	}
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
