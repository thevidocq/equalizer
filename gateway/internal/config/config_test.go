package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormKey(t *testing.T) {
	t.Setenv("EQ_MAP_PUBKEY", "x")
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))

	t.Setenv("EQ_FORM_KEY", "")
	t.Setenv("EQ_FORM_KEY_FILE", "")
	cfg, err := FromEnv()
	if err != nil || cfg.FormKey != nil {
		t.Fatalf("no key: %v %v", cfg.FormKey, err)
	}

	t.Setenv("EQ_FORM_KEY", key)
	if cfg, err = FromEnv(); err != nil || len(cfg.FormKey) != 32 {
		t.Fatalf("env key: %v", err)
	}

	t.Setenv("EQ_FORM_KEY", "")
	path := filepath.Join(t.TempDir(), "form.key")
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EQ_FORM_KEY_FILE", path)
	if cfg, err = FromEnv(); err != nil || len(cfg.FormKey) != 32 {
		t.Fatalf("file key: %v", err)
	}

	t.Setenv("EQ_FORM_KEY", base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("short key: %v", err)
	}
}
