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
	PublicAddr      string         // public traffic address (behind the edge proxy)
	AdminAddr       string         // address of /healthz, /readyz (control network only)
	Instance        string         // instance name in logs
	LogLevel        string         // debug, info, warn, error
	MapFile         string         // SignedMap file (JSON)
	MapPublicKey    string         // Ed25519 public key, base64
	MapPublicKeyID  string         // expected key identifier
	TrustedProxies  []netip.Prefix // networks allowed to set X-Forwarded-For
	FormKey         []byte         // 32-byte key sealing form tokens; nil: random per instance
	ShutdownTimeout time.Duration
}

// FromEnv builds the configuration from environment variables with safe defaults.
func FromEnv() (Config, error) {
	host, _ := os.Hostname()
	cfg := Config{
		PublicAddr:      env("EQ_PUBLIC_ADDR", ":8080"),
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

	proxies, err := parsePrefixes(env("EQ_TRUSTED_PROXIES", ""))
	if err != nil {
		return Config{}, fmt.Errorf("EQ_TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies

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
