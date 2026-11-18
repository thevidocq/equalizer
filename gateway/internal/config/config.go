// Package config legge la configurazione del gateway dalle variabili d'ambiente.
package config

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Config è la configurazione completa del gateway.
type Config struct {
	PublicAddr      string         // indirizzo del traffico pubblico (dietro il proxy edge)
	AdminAddr       string         // indirizzo di /healthz, /readyz (solo rete control)
	Instance        string         // nome dell'istanza nei log
	LogLevel        string         // debug, info, warn, error
	MapFile         string         // file SignedMap (JSON)
	MapPublicKey    string         // chiave pubblica Ed25519 in base64
	MapPublicKeyID  string         // identificativo atteso della chiave
	TrustedProxies  []netip.Prefix // reti da cui accettare X-Forwarded-For
	ShutdownTimeout time.Duration
}

// FromEnv costruisce la configurazione da variabili d'ambiente con valori predefiniti sicuri.
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

	proxies, err := parsePrefixes(env("EQ_TRUSTED_PROXIES", ""))
	if err != nil {
		return Config{}, fmt.Errorf("EQ_TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("EQ_LOG_LEVEL non valido: %q", cfg.LogLevel)
	}
	return cfg, nil
}

// publicKey legge la chiave da EQ_MAP_PUBKEY o dal file in EQ_MAP_PUBKEY_FILE.
func publicKey() (string, error) {
	if k := strings.TrimSpace(os.Getenv("EQ_MAP_PUBKEY")); k != "" {
		return k, nil
	}
	path := os.Getenv("EQ_MAP_PUBKEY_FILE")
	if path == "" {
		return "", fmt.Errorf("serve EQ_MAP_PUBKEY o EQ_MAP_PUBKEY_FILE")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("lettura chiave pubblica: %w", err)
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
