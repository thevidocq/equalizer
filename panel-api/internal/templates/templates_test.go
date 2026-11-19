package templates

import (
	"encoding/json"
	"regexp"
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

// Page slugs never take file names: /wp-login.php, /robots.txt and the
// like must stay with the area, as the legacy site serves them.
func TestSlugsAreNotFiles(t *testing.T) {
	for _, tp := range List() {
		var m struct {
			Types map[string]struct{ Pattern string } `json:"types"`
		}
		doc, _ := Render(tp.ID, Fill{BaseURL: "https://x.example", Connector: "cn-1", Address: "127.0.0.1:80"})
		_ = json.Unmarshal(doc, &m)
		p, ok := m.Types["slug"]
		if !ok {
			continue
		}
		re := regexp.MustCompile(p.Pattern)
		for s, want := range map[string]bool{"chi-siamo": true, "città-nuova": true, "wp-login.php": false, "robots.txt": false, "a/b": false} {
			if re.MatchString(s) != want {
				t.Errorf("%s: slug %q: %v", tp.ID, s, !want)
			}
		}
	}
}
