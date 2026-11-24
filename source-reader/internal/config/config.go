// Package config reads the source reader's settings from the environment.
package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// Config is how the reader reaches the panel and where it keeps its state.
type Config struct {
	PanelURL string // the panel's address, e.g. https://panel.example.com
	Token    string // the site's source token, from the panel's Source tab
	State    string // a private folder: the reader's ssh key and known hosts
	Work     string // where files fetched over sftp or smb are read, then deleted
	Site     string // the site's folder when it is mounted in the reader (method local)
	Instance string
	LogLevel string
}

// Load reads EQ_* variables.
func Load() (Config, error) {
	c := Config{
		// the panel's address; one ending in /v1, the API's, works too
		PanelURL: strings.TrimSuffix(strings.TrimRight(os.Getenv("EQ_PANEL_URL"), "/"), "/v1"),
		Token:    os.Getenv("EQ_SOURCE_TOKEN"),
		State:    env("EQ_SOURCE_STATE", "/var/lib/equalizer-source"),
		Work:     env("EQ_SOURCE_WORK", "/tmp/equalizer-source"),
		Site:     env("EQ_SOURCE_SITE", "/site"),
		Instance: env("HOSTNAME", "source-reader"),
		LogLevel: env("EQ_LOG_LEVEL", "info"),
	}
	u, err := url.Parse(c.PanelURL)
	if err != nil || u.Scheme != "https" && !(u.Scheme == "http" && os.Getenv("EQ_SOURCE_INSECURE_PANEL") == "true") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" {
		return c, errors.New("EQ_PANEL_URL: the panel's address, https://…")
	}
	if len(c.Token) < 20 {
		return c, errors.New("EQ_SOURCE_TOKEN: the token from the site's Source tab in the panel")
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
