// Package config reads the connector configuration from environment variables.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/thevidocq/equalizer/connector/internal/allowlist"
)

// Config is the complete connector configuration.
type Config struct {
	RelayURL   *url.URL
	ID         string
	Token      string
	Allow      *allowlist.List
	RelayTLS   *tls.Config // TLS towards the relay
	LegacyTLS  *tls.Config // TLS towards https legacy destinations
	LogLevel   string
	Instance   string
	StatusFile string // touched while the tunnel is open (Docker healthcheck)
}

// FromEnv builds the configuration.
func FromEnv() (Config, error) {
	host, _ := os.Hostname()
	cfg := Config{
		ID:         os.Getenv("EQ_CONNECTOR_ID"),
		LogLevel:   env("EQ_LOG_LEVEL", "info"),
		Instance:   env("EQ_INSTANCE", host),
		StatusFile: env("EQ_STATUS_FILE", "/tmp/eq-tunnel-up"),
	}
	raw := os.Getenv("EQ_RELAY_URL")
	u, err := url.Parse(raw)
	if raw == "" || err != nil || u.Host == "" {
		return Config{}, fmt.Errorf("EQ_RELAY_URL is required, e.g. https://relay.example.com")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && os.Getenv("EQ_RELAY_INSECURE_HTTP") == "true") {
		return Config{}, fmt.Errorf("EQ_RELAY_URL must use https (plain http only with EQ_RELAY_INSECURE_HTTP=true, for local tests)")
	}
	cfg.RelayURL = u
	if cfg.ID == "" {
		return Config{}, fmt.Errorf("EQ_CONNECTOR_ID is required")
	}
	if cfg.Token, err = secret("EQ_CONNECTOR_TOKEN"); err != nil {
		return Config{}, err
	}
	if cfg.Token == "" {
		return Config{}, fmt.Errorf("EQ_CONNECTOR_TOKEN or EQ_CONNECTOR_TOKEN_FILE is required")
	}
	if cfg.Allow, err = allowlist.Parse(os.Getenv("EQ_ALLOW")); err != nil {
		return Config{}, fmt.Errorf("EQ_ALLOW: %w", err)
	}
	if cfg.RelayTLS, err = tlsConfig(os.Getenv("EQ_RELAY_CA_FILE"), false); err != nil {
		return Config{}, fmt.Errorf("EQ_RELAY_CA_FILE: %w", err)
	}
	// Old legacy servers often have self-signed certificates: trust a
	// specific CA, or (last resort) skip verification on the local network.
	insecure := os.Getenv("EQ_LEGACY_INSECURE_TLS") == "true"
	if cfg.LegacyTLS, err = tlsConfig(os.Getenv("EQ_LEGACY_CA_FILE"), insecure); err != nil {
		return Config{}, fmt.Errorf("EQ_LEGACY_CA_FILE: %w", err)
	}
	return cfg, nil
}

func tlsConfig(caFile string, insecure bool) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecure} //nolint:gosec // opt-in, legacy only
	if caFile == "" {
		return cfg, nil
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificate found in %s", caFile)
	}
	cfg.RootCAs = pool
	return cfg, nil
}

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
