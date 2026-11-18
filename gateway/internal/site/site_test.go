package site

import (
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func baseModel() *mapmodel.Model {
	return &mapmodel.Model{
		Legacy: mapmodel.Legacy{BaseURL: "http://10.0.0.1:8080", Reach: mapmodel.Reach{Mode: "direct", Allow: []string{"10.0.0.1:8080"}}},
		Roles:  mapmodel.Roles{Anonymous: "anonimo", List: map[string]any{"anonimo": nil}},
		Routes: []mapmodel.Route{{ID: "home", Path: "/", Method: "GET", Roles: []string{"anonimo"},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}}},
	}
}

func TestBuild(t *testing.T) {
	if _, err := Build(baseModel()); err != nil {
		t.Fatalf("mappa valida rifiutata: %v", err)
	}
	cases := map[string]func(*mapmodel.Model){
		"ruolo anonimo inesistente": func(m *mapmodel.Model) { m.Roles.Anonymous = "ospite" },
		"modalità sconosciuta":      func(m *mapmodel.Model) { m.Legacy.Reach.Mode = "vpn" },
		"allowlist vuota":           func(m *mapmodel.Model) { m.Legacy.Reach.Allow = nil },
		"segnaposto non valido": func(m *mapmodel.Model) {
			m.Routes[0].LegacyRequest.Query = map[string]string{"x": "{{env.HOME}}"}
		},
		"pattern non valido": func(m *mapmodel.Model) {
			m.Routes[0].Query = map[string]mapmodel.FieldSpec{"q": {Type: "string", Pattern: "(["}}
		},
	}
	for name, mutate := range cases {
		m := baseModel()
		mutate(m)
		if _, err := Build(m); err == nil {
			t.Errorf("%s: mappa accettata a torto", name)
		}
	}
}

func TestConnectorModeFailsSafely(t *testing.T) {
	m := baseModel()
	m.Legacy.Reach = mapmodel.Reach{Mode: "connector", ConnectorID: "cn-1", Allow: []string{"10.0.0.1:8080"}}
	st, err := Build(m)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := st.Legacy.Build(t.Context(), m.Routes[0].LegacyRequest, nil)
	if _, err := st.Legacy.Do(req); err == nil || !strings.Contains(err.Error(), "connettore") {
		t.Fatalf("atteso errore del connettore, ottenuto %v", err)
	}
}
