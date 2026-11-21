package store

import (
	"encoding/json"
	"testing"
)

func TestSetBaseHost(t *testing.T) {
	for in, want := range map[string]string{
		"https://metreta.example.com":       "https://www.metreta.com",
		"https://metreta.example.com:8443/": "https://www.metreta.com:8443/",
	} {
		doc, _ := json.Marshal(map[string]any{"legacy": map[string]any{"base_url": in, "reach": map[string]any{"mode": "direct"}}})
		out, err := SetBaseHost(doc, "www.metreta.com")
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Legacy struct {
				BaseURL string         `json:"base_url"`
				Reach   map[string]any `json:"reach"`
			} `json:"legacy"`
		}
		_ = json.Unmarshal(out, &m)
		if m.Legacy.BaseURL != want || m.Legacy.Reach["mode"] != "direct" {
			t.Errorf("%s -> %s (reach %v), want %s", in, m.Legacy.BaseURL, m.Legacy.Reach, want)
		}
	}
}
