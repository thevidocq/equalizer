package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// The panel's CMS templates must give maps the gateway loads.
func TestPanelTemplatesLoad(t *testing.T) {
	files, _ := filepath.Glob("../../../panel-api/internal/templates/maps/*.json")
	if len(files) == 0 {
		t.Skip("templates not found (module built outside the repository)")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.NewReplacer(`"__BASE_URL__"`, `"https://site.example"`, `"__CONNECTOR__"`, `"cn-1"`,
			`"__ADDRESS__"`, `"10.0.0.1:443"`).Replace(string(raw))
		var m mapmodel.Model
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		m.Meta.Tenant, m.Meta.App, m.Meta.Version = "t", "a", 1
		if _, err := Build(&m); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
		}
	}
}
