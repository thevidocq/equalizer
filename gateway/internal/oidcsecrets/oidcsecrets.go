// Package oidcsecrets keeps the client secrets of the sites that log in with
// OIDC, read from the panel every few seconds. They stay in memory only and
// are never logged.
package oidcsecrets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Source reads the secrets from panel-api (/v1/internal/oidc).
type Source struct {
	URL, Token string
	Client     *http.Client
	Log        *slog.Logger

	mu      sync.RWMutex
	secrets map[string]string
}

// OIDCSecret implements server.Secrets.
func (s *Source) OIDCSecret(app string) string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.secrets[app]
}

// Refresh reads the secrets once; on error the previous ones stay.
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
			App          string `json:"app"`
			ClientSecret string `json:"client_secret"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return err
	}
	next := make(map[string]string, len(body.Items))
	for _, it := range body.Items {
		next[it.App] = it.ClientSecret
	}
	s.mu.Lock()
	s.secrets = next
	s.mu.Unlock()
	return nil
}

// Run refreshes every interval until ctx ends.
func (s *Source) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := s.Refresh(ctx); err != nil && s.Log != nil {
			s.Log.WarnContext(ctx, "OIDC secrets not refreshed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
