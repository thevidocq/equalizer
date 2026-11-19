package certfeed

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefresh(t *testing.T) {
	up := true
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || !up {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"app":"portal","domains":["www.acme.test","Acme.test"],"chain_pem":"CHAIN\n","key_pem":"KEY"}]}`)
	}))
	defer panel.Close()
	s := &Source{URL: panel.URL, Token: "tok", Client: panel.Client()}
	if s.Bundle("www.acme.test") != nil || s.Loaded() {
		t.Fatal("nothing before the first read")
	}
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b := string(s.Bundle("ACME.test.")); b != "CHAIN\nKEY\n" || !s.Loaded() {
		t.Errorf("bundle: %q", b)
	}
	// The panel down: the certificates stay, sites keep their HTTPS.
	up = false
	if err := s.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("refresh with the panel down: %v", err)
	}
	if s.Bundle("www.acme.test") == nil {
		t.Error("certificates dropped when the panel did not answer")
	}
}
