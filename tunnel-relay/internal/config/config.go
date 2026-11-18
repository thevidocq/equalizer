// Package config reads the relay configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the complete relay configuration.
type Config struct {
	TunnelAddr      string // connectors (behind the edge proxy, which terminates TLS)
	InternalAddr    string // gateway requests (control network only)
	AdminAddr       string // /healthz, /readyz, /v1/tunnels (control network only)
	Peers           string // DNS name of the replicas (EQ_PEERS); empty: a single relay
	Instance        string
	LogLevel        string
	ConnectorsFile  string
	ConnectorsURL   string // panel-api /v1/internal/connectors; takes precedence over the file
	PanelToken      string // token for ConnectorsURL
	GatewayToken    string
	ShutdownTimeout time.Duration
}

// FromEnv builds the configuration.
func FromEnv() (Config, error) {
	host, _ := os.Hostname()
	cfg := Config{
		TunnelAddr:      env("EQ_TUNNEL_ADDR", ":8081"),
		InternalAddr:    env("EQ_INTERNAL_ADDR", ":8082"),
		AdminAddr:       env("EQ_ADMIN_ADDR", ":9091"),
		Instance:        env("EQ_INSTANCE", host),
		LogLevel:        env("EQ_LOG_LEVEL", "info"),
		ConnectorsFile:  env("EQ_CONNECTORS_FILE", "/secrets/connectors.json"),
		ShutdownTimeout: 10 * time.Second,
	}
	token, err := secret("EQ_GATEWAY_TOKEN")
	if err != nil {
		return Config{}, err
	}
	if len(token) < 32 {
		return Config{}, fmt.Errorf("EQ_GATEWAY_TOKEN or EQ_GATEWAY_TOKEN_FILE is required (at least 32 characters)")
	}
	cfg.GatewayToken = token
	cfg.ConnectorsURL = os.Getenv("EQ_CONNECTORS_URL")
	cfg.Peers = os.Getenv("EQ_PEERS")
	if cfg.PanelToken, err = secret("EQ_PANEL_TOKEN"); err != nil {
		return Config{}, err
	}
	if cfg.ConnectorsURL != "" && cfg.PanelToken == "" {
		return Config{}, fmt.Errorf("EQ_CONNECTORS_URL needs EQ_PANEL_TOKEN or EQ_PANEL_TOKEN_FILE")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("invalid EQ_LOG_LEVEL: %q", cfg.LogLevel)
	}
	return cfg, nil
}

// secret reads NAME, or the file named by NAME_FILE.
func secret(name string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v, nil
	}
	path := os.Getenv(name + "_FILE")
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s_FILE: %w", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
