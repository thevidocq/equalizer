package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("EQ_SOURCE_TOKEN", "0123456789abcdefghijklmnop")
	for in, want := range map[string]string{
		"https://panel.example.com":     "https://panel.example.com",
		"https://panel.example.com/":    "https://panel.example.com",
		"https://panel.example.com/v1/": "https://panel.example.com",
	} {
		t.Setenv("EQ_PANEL_URL", in)
		c, err := Load()
		if err != nil || c.PanelURL != want {
			t.Errorf("%s: %q %v", in, c.PanelURL, err)
		}
	}
	for _, bad := range []string{"", "http://panel.example.com", "ftp://x", "https://user:pw@panel.example.com"} {
		t.Setenv("EQ_PANEL_URL", bad)
		if _, err := Load(); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	t.Setenv("EQ_SOURCE_INSECURE_PANEL", "true")
	t.Setenv("EQ_PANEL_URL", "http://panel.lan")
	if _, err := Load(); err != nil {
		t.Error("http with EQ_SOURCE_INSECURE_PANEL")
	}
	t.Setenv("EQ_SOURCE_TOKEN", "short")
	if _, err := Load(); err == nil {
		t.Error("a short token")
	}
}
