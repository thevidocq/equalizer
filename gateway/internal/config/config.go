// Package config reads the gateway configuration from environment variables.
package config

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Config is the complete gateway configuration.
type Config struct {
	PublicAddr       string         // public traffic address (behind the edge proxy)
	AdminAddr        string         // address of /healthz, /readyz (control network only)
	Instance         string         // instance name in logs
	LogLevel         string         // debug, info, warn, error
	MapFile          string         // SignedMap file (JSON), used when MapURL is empty
	MapURL           string         // panel-api internal URL of the published map
	MapToken         string         // token for MapURL
	UnmappedURL      string         // panel endpoint for refused paths; default next to MapURL
	OIDCSecretsURL   string         // panel endpoint of OIDC client secrets; default next to MapURL
	CertificatesURL  string         // panel endpoint of the sites' certificates; default next to MapURL
	PublicHTTPAddr   string         // plain http traffic from Caddy (http sites; https sites are redirected)
	MapPublicKey     string         // Ed25519 public key, base64
	MapPublicKeyID   string         // expected key identifier
	TrustedProxies   []netip.Prefix // networks allowed to set X-Forwarded-For
	BehindCloudflare bool           // honour CF-Connecting-IP (EQ_BEHIND_CLOUDFLARE)
	FormKey          []byte         // 32-byte key sealing form tokens; nil: random per instance
	RelayURL         string         // tunnel-relay internal address; empty: no connector mode
	RelayToken       string         // token presented to the relay
	RedisURL         string         // session store; empty: in-process store (single replica)
	ShutdownTimeout  time.Duration
}

// FromEnv builds the configuration from environment variables with safe defaults.
func FromEnv() (Config, error) {
	host, _ := os.Hostname()
	cfg := Config{
		PublicAddr:      env("EQ_PUBLIC_ADDR", ":8080"),
		PublicHTTPAddr:  env("EQ_PUBLIC_HTTP_ADDR", ":8081"),
		AdminAddr:       env("EQ_ADMIN_ADDR", ":9090"),
		Instance:        env("EQ_INSTANCE", host),
		LogLevel:        env("EQ_LOG_LEVEL", "info"),
		MapFile:         env("EQ_MAP_FILE", "/maps/current.json"),
		MapPublicKeyID:  env("EQ_MAP_KEY_ID", ""),
		ShutdownTimeout: 10 * time.Second,
	}

	key, err := publicKey()
	if err != nil {
		return Config{}, err
	}
	cfg.MapPublicKey = key

	formKey, err := formKey()
	if err != nil {
		return Config{}, err
	}
	cfg.FormKey = formKey

	cfg.MapURL = os.Getenv("EQ_MAP_URL")
	if cfg.MapToken, err = secretValue("EQ_MAP_TOKEN"); err != nil {
		return Config{}, err
	}
	cfg.UnmappedURL = os.Getenv("EQ_UNMAPPED_URL")
	if cfg.UnmappedURL == "" && strings.HasSuffix(cfg.MapURL, "/v1/internal/maps") {
		cfg.UnmappedURL = strings.TrimSuffix(cfg.MapURL, "/maps") + "/unmapped"
	}
	cfg.CertificatesURL = os.Getenv("EQ_CERTIFICATES_URL")
	if cfg.CertificatesURL == "" && strings.HasSuffix(cfg.MapURL, "/v1/internal/maps") {
		cfg.CertificatesURL = strings.TrimSuffix(cfg.MapURL, "/maps") + "/certificates"
	}
	cfg.OIDCSecretsURL = os.Getenv("EQ_OIDC_SECRETS_URL")
	if cfg.OIDCSecretsURL == "" && strings.HasSuffix(cfg.MapURL, "/v1/internal/maps") {
		cfg.OIDCSecretsURL = strings.TrimSuffix(cfg.MapURL, "/maps") + "/oidc"
	}
	if cfg.MapURL != "" && cfg.MapToken == "" {
		return Config{}, fmt.Errorf("EQ_MAP_URL needs EQ_MAP_TOKEN or EQ_MAP_TOKEN_FILE")
	}
	cfg.RelayURL = os.Getenv("EQ_RELAY_URL")
	cfg.RedisURL = os.Getenv("EQ_REDIS_URL")
	if cfg.RelayToken, err = secretValue("EQ_RELAY_TOKEN"); err != nil {
		return Config{}, err
	}

	proxies, err := parsePrefixes(env("EQ_TRUSTED_PROXIES", ""))
	if err != nil {
		return Config{}, fmt.Errorf("EQ_TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies
	cfg.BehindCloudflare = os.Getenv("EQ_BEHIND_CLOUDFLARE") == "true"

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("invalid EQ_LOG_LEVEL: %q", cfg.LogLevel)
	}
	return cfg, nil
}

// publicKey reads the key from EQ_MAP_PUBKEY or from the file in EQ_MAP_PUBKEY_FILE.
func publicKey() (string, error) {
	if k := strings.TrimSpace(os.Getenv("EQ_MAP_PUBKEY")); k != "" {
		return k, nil
	}
	path := os.Getenv("EQ_MAP_PUBKEY_FILE")
	if path == "" {
		return "", fmt.Errorf("EQ_MAP_PUBKEY or EQ_MAP_PUBKEY_FILE is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading public key: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// formKey reads the form-token key (base64, 32 bytes) from EQ_FORM_KEY or
// from the file in EQ_FORM_KEY_FILE. It is optional: without it each
// instance uses a random key, which works with a single replica only.
func formKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("EQ_FORM_KEY"))
	if raw == "" {
		path := os.Getenv("EQ_FORM_KEY_FILE")
		if path == "" {
			return nil, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading form key: %w", err)
		}
		raw = strings.TrimSpace(string(b))
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("the form key must be 32 bytes in base64")
	}
	return key, nil
}

// secretValue reads NAME, or the file named by NAME_FILE.
func secretValue(name string) (string, error) {
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

func parsePrefixes(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
