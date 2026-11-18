package site

import (
	"errors"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func baseModel() *mapmodel.Model {
	return &mapmodel.Model{
		Legacy: mapmodel.Legacy{BaseURL: "http://10.0.0.1:8080", Reach: mapmodel.Reach{Mode: "direct", Allow: []string{"10.0.0.1:8080"}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Routes: []mapmodel.Route{{ID: "home", Path: "/", Method: "GET", Roles: []string{"guest"},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}}},
	}
}

func TestBuild(t *testing.T) {
	if _, err := Build(baseModel()); err != nil {
		t.Fatalf("valid map rejected: %v", err)
	}
	cases := map[string]func(*mapmodel.Model){
		"unknown anonymous role": func(m *mapmodel.Model) { m.Roles.Anonymous = "visitor" },
		"unknown mode":           func(m *mapmodel.Model) { m.Legacy.Reach.Mode = "vpn" },
		"empty allowlist":        func(m *mapmodel.Model) { m.Legacy.Reach.Allow = nil },
		"invalid placeholder": func(m *mapmodel.Model) {
			m.Routes[0].LegacyRequest.Query = map[string]string{"x": "{{env.HOME}}"}
		},
		"invalid pattern": func(m *mapmodel.Model) {
			m.Routes[0].Query = map[string]mapmodel.FieldSpec{"q": {Type: "string", Pattern: "(["}}
		},
	}
	for name, mutate := range cases {
		m := baseModel()
		mutate(m)
		if _, err := Build(m); err == nil {
			t.Errorf("%s: map wrongly accepted", name)
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
	if _, err := st.Legacy.Do(req); !errors.Is(err, ErrConnectorUnavailable) {
		t.Fatalf("expected the connector error, got %v", err)
	}
}
