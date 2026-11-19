// Package certfeed keeps the certificates of the protected sites (each the
// legacy site's own, uploaded in the panel), read from the panel every few
// seconds, to give Caddy at TLS handshake. Keys stay in memory only and are
// never logged.
package certfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Source reads the certificates from panel-api (/v1/internal/certificates).
type Source struct {
	URL, Token string
	Client     *http.Client
	Log        *slog.Logger

	mu     sync.RWMutex
	bundle map[string][]byte // domain → PEM chain then key, as Caddy takes it
	loaded bool
}

// Bundle returns the PEM bundle for a domain, or nil.
func (s *Source) Bundle(domain string) []byte {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bundle[strings.ToLower(strings.TrimSuffix(domain, "."))]
}

// Loaded reports whether the certificates were read at least once.
func (s *Source) Loaded() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

// Refresh reads the certificates once; on error the previous ones stay, so
// a panel restart never takes a site's HTTPS down.
func (s *Source) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel answered %d", resp.StatusCode)
	}
	var body struct {
		Items []struct {
			Domains  []string `json:"domains"`
			ChainPEM string   `json:"chain_pem"`
			KeyPEM   string   `json:"key_pem"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&body); err != nil {
		return err
	}
	next := map[string][]byte{}
	for _, it := range body.Items {
		b := []byte(strings.TrimSpace(it.ChainPEM) + "\n" + strings.TrimSpace(it.KeyPEM) + "\n")
		for _, d := range it.Domains {
			next[strings.ToLower(d)] = b
		}
	}
	s.mu.Lock()
	s.bundle, s.loaded = next, true
	s.mu.Unlock()
	return nil
}

// Run refreshes every interval until ctx ends.
func (s *Source) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := s.Refresh(ctx); err != nil && s.Log != nil {
			s.Log.WarnContext(ctx, "site certificates not refreshed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
