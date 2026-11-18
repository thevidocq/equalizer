// Package config reads the panel-api configuration from environment variables.
package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is the complete panel-api configuration.
type Config struct {
	Addr            string // API and (later) the panel UI; control network or localhost only
	AdminAddr       string // /healthz, /readyz
	Instance        string
	LogLevel        string
	DatabaseURL     string // postgres://user@host/db?sslmode=disable, password from EQ_DATABASE_PASSWORD[_FILE]
	SigningKey      ed25519.PrivateKey
	SigningKeyID    string
	InternalToken   string // presented by gateway and tunnel-relay on /v1/internal
	MFAKey          []byte // encrypts authenticator secrets (32 bytes)
	MFARequired     bool   // every panel user must use an authenticator app (default true)
	PanelURL        string // public address of the panel, for links in emails
	MailMode        string // smtp, log (development: links in the logs) or off
	SMTPAddr        string
	SMTPUser        string
	SMTPPassword    string
	MailFrom        string
	SessionTTL      time.Duration
	UIDir           string // static files of panel-ui; empty: API only
	ShutdownTimeout time.Duration
}

// FromEnv builds the configuration.
func FromEnv() (Config, error) {
	host, _ := os.Hostname()
	cfg := Config{
		Addr:            env("EQ_ADDR", ":8080"),
		AdminAddr:       env("EQ_ADMIN_ADDR", ":9092"),
		Instance:        env("EQ_INSTANCE", host),
		LogLevel:        env("EQ_LOG_LEVEL", "info"),
		SigningKeyID:    env("EQ_MAP_KEY_ID", "k1"),
		SessionTTL:      8 * time.Hour,
		UIDir:           os.Getenv("EQ_UI_DIR"),
		ShutdownTimeout: 10 * time.Second,
	}
	dsn, err := DatabaseURL()
	if err != nil {
		return Config{}, err
	}
	cfg.DatabaseURL = dsn

	seed, err := Secret("EQ_MAP_SIGNING_KEY")
	if err != nil {
		return Config{}, err
	}
	raw, err := base64.StdEncoding.DecodeString(seed)
	if err != nil || len(raw) != ed25519.SeedSize {
		return Config{}, fmt.Errorf("EQ_MAP_SIGNING_KEY or EQ_MAP_SIGNING_KEY_FILE is required (eqtool keygen)")
	}
	cfg.SigningKey = ed25519.NewKeyFromSeed(raw)

	if cfg.InternalToken, err = Secret("EQ_INTERNAL_TOKEN"); err != nil {
		return Config{}, err
	}
	if len(cfg.InternalToken) < 32 {
		return Config{}, fmt.Errorf("EQ_INTERNAL_TOKEN or EQ_INTERNAL_TOKEN_FILE is required (at least 32 characters)")
	}
	mfaKey, err := Secret("EQ_MFA_KEY")
	if err != nil {
		return Config{}, err
	}
	if cfg.MFAKey, err = base64.StdEncoding.DecodeString(mfaKey); err != nil || len(cfg.MFAKey) != 32 {
		return Config{}, fmt.Errorf("EQ_MFA_KEY or EQ_MFA_KEY_FILE is required: 32 bytes in base64 (eqtool formkey)")
	}
	cfg.MFARequired = os.Getenv("EQ_MFA_REQUIRED") != "false"

	cfg.PanelURL = env("EQ_PANEL_URL", "http://localhost:8088")
	cfg.SMTPAddr = os.Getenv("EQ_SMTP_ADDR")
	cfg.SMTPUser = os.Getenv("EQ_SMTP_USER")
	if cfg.SMTPPassword, err = Secret("EQ_SMTP_PASSWORD"); err != nil {
		return Config{}, err
	}
	cfg.MailFrom = env("EQ_MAIL_FROM", "equalizer@localhost")
	cfg.MailMode = os.Getenv("EQ_MAIL_MODE")
	if cfg.MailMode == "" {
		cfg.MailMode = "off"
		if cfg.SMTPAddr != "" {
			cfg.MailMode = "smtp"
		}
	}
	switch cfg.MailMode {
	case "smtp", "log", "off":
	default:
		return Config{}, fmt.Errorf("EQ_MAIL_MODE must be smtp, log or off")
	}
	if cfg.MailMode == "smtp" && cfg.SMTPAddr == "" {
		return Config{}, fmt.Errorf("EQ_MAIL_MODE=smtp needs EQ_SMTP_ADDR (host:587)")
	}
	return cfg, nil
}

// DatabaseURL reads EQ_DATABASE_URL and adds the password from
// EQ_DATABASE_PASSWORD[_FILE], so the password never sits in the URL.
func DatabaseURL() (string, error) {
	raw := os.Getenv("EQ_DATABASE_URL")
	if raw == "" {
		return "", fmt.Errorf("EQ_DATABASE_URL is required, e.g. postgres://equalizer@postgres/equalizer?sslmode=disable")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", fmt.Errorf("invalid EQ_DATABASE_URL")
	}
	pw, err := Secret("EQ_DATABASE_PASSWORD")
	if err != nil {
		return "", err
	}
	if pw != "" && u.User != nil {
		u.User = url.UserPassword(u.User.Username(), pw)
	}
	return u.String(), nil
}

// Secret reads NAME, or the file named by NAME_FILE.
func Secret(name string) (string, error) {
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
