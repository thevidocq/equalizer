package templates

import (
	"encoding/json"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/mapschema"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Every template, once filled, is a valid map.
func TestTemplatesAreValidMaps(t *testing.T) {
	list := List()
	if len(list) != 5 || list[len(list)-1].ID != "generic" {
		t.Fatalf("templates: %+v", list)
	}
	for _, tp := range list {
		if tp.Name == "" || tp.Description == "" {
			t.Errorf("%s: name and description required", tp.ID)
		}
		doc, err := Render(tp.ID, Fill{BaseURL: "https://portal.example.org", Connector: "cn-srv-01", Address: "127.0.0.1:8443", MaxUploadMB: 200})
		if err != nil {
			t.Fatal(err)
		}
		doc, _ = store.SetMeta(doc, map[string]any{"tenant": "acme", "app": "portal", "public_domain": "portal.example.org"})
		if err := mapschema.Validate(doc); err != nil {
			t.Errorf("%s: %v", tp.ID, err)
		}
		var m struct {
			Areas []struct {
				MaxBodyKB int `json:"max_body_kb"`
			} `json:"areas"`
		}
		_ = json.Unmarshal(doc, &m)
		if len(m.Areas) == 0 || m.Areas[0].MaxBodyKB != 200*1024 {
			t.Errorf("%s: upload size not set: %+v", tp.ID, m.Areas)
		}
	}
	if _, err := Render("../secret", Fill{}); err != ErrUnknown {
		t.Error("path outside the templates accepted")
	}
}
