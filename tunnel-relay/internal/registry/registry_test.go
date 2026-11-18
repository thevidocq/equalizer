package registry

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connectors.json")
	write := func(body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"connectors":[{"id":"cn-1","tenant":"t","app":"a","token_sha256":"` + Hash("secret") + `"}]}`)
	r := New(path)
	if changed, err := r.Reload(); err != nil || !changed {
		t.Fatalf("reload: %v %v", changed, err)
	}
	if c, ok := r.Authenticate("cn-1", "secret"); !ok || c.Tenant != "t" {
		t.Fatal("valid token rejected")
	}
	for _, c := range [][2]string{{"cn-1", "wrong"}, {"cn-1", ""}, {"cn-2", "secret"}} {
		if _, ok := r.Authenticate(c[0], c[1]); ok {
			t.Errorf("%v accepted", c)
		}
	}
	if changed, _ := r.Reload(); changed {
		t.Error("unchanged file reported as changed")
	}

	// A broken file keeps the previous set.
	time.Sleep(10 * time.Millisecond)
	write(`{"connectors":[{"id":"BAD ID","token_sha256":"00"}]}`)
	if _, err := r.Reload(); err == nil {
		t.Fatal("invalid file accepted")
	}
	if _, ok := r.Authenticate("cn-1", "secret"); !ok {
		t.Fatal("previous set lost after a bad reload")
	}
}

func TestParseRejects(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`{"connectors":[{"id":"cn-1","token_sha256":"abc"}]}`,
		`{"connectors":[{"id":"cn-1","token_sha256":"` + Hash("a") + `"},{"id":"cn-1","token_sha256":"` + Hash("b") + `"}]}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("accepted: %s", body)
		}
	}
}

func TestRegistryFromPanel(t *testing.T) {
	body := `{"connectors":[{"id":"cn-1","tenant":"t","app":"a","token_sha256":"` + Hash("secret") + `"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer panel-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	r := NewFromPanel(srv.URL, "panel-tok")
	if changed, err := r.Reload(); err != nil || !changed {
		t.Fatalf("first reload: %v %v", changed, err)
	}
	if _, ok := r.Authenticate("cn-1", "secret"); !ok {
		t.Fatal("connector from the panel not accepted")
	}
	if changed, _ := r.Reload(); changed {
		t.Error("same document reported as changed")
	}
	body = `{"connectors":[]}`
	if changed, _ := r.Reload(); !changed {
		t.Error("new document not seen")
	}
	if _, ok := r.Authenticate("cn-1", "secret"); ok {
		t.Error("removed connector still accepted")
	}
	if _, err := NewFromPanel(srv.URL, "wrong").Reload(); err == nil {
		t.Error("wrong token accepted")
	}
}
