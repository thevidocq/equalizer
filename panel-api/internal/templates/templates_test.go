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

// Assets drop the query string: generated files that need it (Drupal's
// aggregated CSS and JS, image styles) belong to the area. Favicons come as
// image/vnd.microsoft.icon from PHP and Apache.
func TestAssetsServeWhatSitesSend(t *testing.T) {
	for _, tp := range List() {
		var m struct {
			Assets []struct {
				Prefix string   `json:"prefix"`
				Types  []string `json:"types"`
			} `json:"assets"`
		}
		doc, _ := Render(tp.ID, Fill{BaseURL: "https://x.example", Connector: "cn-1", Address: "127.0.0.1:80"})
		_ = json.Unmarshal(doc, &m)
		for _, a := range m.Assets {
			if a.Prefix == "/sites/default/files/" {
				t.Errorf("%s: %s is generated on request with its query", tp.ID, a.Prefix)
			}
			has := map[string]bool{}
			for _, ty := range a.Types {
				has[ty] = true
			}
			if has["image/x-icon"] && !has["image/vnd.microsoft.icon"] {
				t.Errorf("%s %s: image/vnd.microsoft.icon missing", tp.ID, a.Prefix)
			}
		}
	}
}
