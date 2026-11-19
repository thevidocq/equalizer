package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminMapCheck(t *testing.T) {
	h := (&Admin{}).Handler()
	ok := `{"meta":{"schema":"gateway-map/1.0","tenant":"t","app":"a","version":1,"status":"draft"},
	 "legacy":{"base_url":"http://10.0.0.1:8080","reach":{"mode":"direct","allow":["10.0.0.1:8080"]}},
	 "roles":{"anonymous":"guest","list":{"guest":{}}},
	 "routes":[{"id":"home","path":"/","method":"GET","roles":["guest"],"legacy_request":{"method":"GET","path":"/"}}]}`
	for name, c := range map[string]struct {
		body string
		code int
	}{
		"consistent":   {ok, 200},
		"unknown role": {strings.Replace(ok, `"roles":["guest"]`, `"roles":["staff"]`, 1), 422},
		"not JSON":     {"{", 422},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/maps/check", strings.NewReader(c.body)))
		if rec.Code != c.code {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}
