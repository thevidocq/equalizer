package mapstore

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

func withDomain(app, domain string) string {
	return strings.Replace(strings.Replace(testMap, `"app":"a"`, `"app":"`+app+`"`, 1),
		`"status":"published"`, `"status":"published","public_domain":"`+domain+`"`, 1)
}

func list(t *testing.T, docs ...[]byte) []byte {
	t.Helper()
	items := make([]json.RawMessage, len(docs))
	for i, d := range docs {
		items[i] = d
	}
	b, _ := json.Marshal(map[string]any{"items": items})
	return b
}

func TestManySites(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "maps.json")
	s := NewStore(path, pub, "", site.Options{})
	portal := sign(t, priv, withDomain("portal", "portal.example.org"), 3, "k1")
	taxes := sign(t, priv, withDomain("taxes", "taxes.example.org"), 3, "k1")

	_ = os.WriteFile(path, list(t, portal, taxes), 0o600)
	if _, err := s.Reload(); err != nil {
		t.Fatal(err)
	}
	for host, app := range map[string]string{"portal.example.org": "portal", "TAXES.example.org:443": "taxes", "taxes.example.org.": "taxes"} {
		if st := s.For(host); st == nil || st.Model.Meta.App != app {
			t.Errorf("%s: %v", host, st)
		}
	}
	if s.For("blog.example.org") != nil || s.For("") != nil {
		t.Error("a host without map must have no site")
	}
	if !s.Serves("portal.example.org") || s.Serves("evil.example.org") || !s.Ready() {
		t.Error("Serves/Ready")
	}

	// A new taxes map signed by another key: taxes keeps its previous map,
	// portal is not affected, a removed site disappears.
	badTaxes := sign(t, otherPriv, withDomain("taxes", "taxes.example.org"), 4, "k1")
	_ = os.WriteFile(path, list(t, badTaxes), 0o600)
	s.Source().(*FileSource).Forget()
	rep, err := s.Reload()
	if err == nil || len(rep.Failed) != 1 {
		t.Fatalf("refused map not reported: %v %+v", err, rep)
	}
	if st := s.For("taxes.example.org"); st == nil || st.Model.Meta.Version != 3 {
		t.Error("taxes must keep its previous map")
	}
	if s.For("portal.example.org") != nil {
		t.Error("portal was removed from the panel and must not be served")
	}

	// Two maps for one domain, or a map without domain among others: refused.
	dup := sign(t, priv, withDomain("copy", "portal.example.org"), 3, "k1")
	bare := sign(t, priv, strings.Replace(testMap, `"app":"a"`, `"app":"bare"`, 1), 3, "k1")
	_ = os.WriteFile(path, list(t, portal, dup, bare), 0o600)
	s.Source().(*FileSource).Forget()
	rep, _ = s.Reload()
	if len(rep.Failed) != 2 || s.For("portal.example.org").Model.Meta.App != "portal" || s.For("x.example.org") != nil {
		t.Fatalf("duplicates: %+v", rep.Failed)
	}
}

func TestAliases(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "maps.json")
	s := NewStore(path, pub, "", site.Options{})
	withAliases := func(app, domain string, aliases ...string) string {
		b, _ := json.Marshal(aliases)
		return strings.Replace(withDomain(app, domain), `"public_domain":"`+domain+`"`, `"public_domain":"`+domain+`","aliases":`+string(b), 1)
	}
	portal := sign(t, priv, withAliases("portal", "www.example.org", "example.org", "old-portal.example.net"), 3, "k1")
	thief := sign(t, priv, withAliases("thief", "thief.example.org", "example.org"), 3, "k1")
	_ = os.WriteFile(path, list(t, portal, thief), 0o600)
	rep, _ := s.Reload()
	if len(rep.Failed) != 1 {
		t.Fatalf("a domain taken twice must be refused: %+v", rep.Failed)
	}
	if st := s.AliasOf("EXAMPLE.org:443"); st == nil || st.Model.Meta.App != "portal" {
		t.Errorf("alias: %v", st)
	}
	if s.For("example.org") != nil || s.AliasOf("www.example.org") != nil {
		t.Error("an alias is not the site, the site is not an alias")
	}
	if !s.Serves("old-portal.example.net") || !s.Serves("www.example.org") {
		t.Error("aliases need certificates too")
	}
}
