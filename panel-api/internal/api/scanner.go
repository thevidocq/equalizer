package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/cveradar"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The tenant's vulnerability scanner: its API address and key, tried before
// they are saved; the key is sealed and never returned.

func scannerLabel(tenant string) string { return "scanner:" + tenant }

type scannerView struct {
	Endpoint  *string    `json:"endpoint"`
	HasAPIKey bool       `json:"has_api_key"`
	KeyHint   *string    `json:"key_hint"`
	Status    string     `json:"status"`
	CheckedAt *time.Time `json:"checked_at"`
}

func (s *Server) getScanner(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	c, err := s.Store.TenantScanner(r.Context(), tenant)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, scannerView{Status: "not_connected"})
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	status := "connected"
	if c.KeyRefused {
		status = "key_refused"
	}
	writeJSON(w, http.StatusOK, scannerView{Endpoint: &c.Endpoint, HasAPIKey: len(c.Key) > 0, KeyHint: &c.KeyHint, Status: status,
		CheckedAt: c.CheckedAt})
}

func (s *Server) putScanner(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	var in struct {
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	endpoint := strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	key := strings.TrimSpace(in.APIKey)
	if !cveradar.ValidEndpoint(endpoint) || s.ScannerHTTP == nil && !cveradar.OnHTTPSPort(endpoint) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "endpoint: the https address (port 443) of the scanner's API, e.g. https://api.example.com/api/public/v1")
		return
	}
	if len(key) > 300 || strings.ContainsAny(key, "\r\n \t") {
		fail(w, http.StatusUnprocessableEntity, "invalid", "api_key: at most 300 characters, no spaces")
		return
	}
	if key == "" {
		old, err := s.Store.TenantScanner(r.Context(), tenant)
		if err != nil {
			fail(w, http.StatusUnprocessableEntity, "invalid", "api_key is required the first time")
			return
		}
		if old.Endpoint != endpoint {
			// The saved key goes only where it was saved for.
			fail(w, http.StatusUnprocessableEntity, "invalid", "a new address needs the key again")
			return
		}
		k, err := s.MFA.OpenFor(scannerLabel(tenant), old.Key)
		if err != nil {
			fail(w, http.StatusUnprocessableEntity, "invalid", "the saved key cannot be read any more: enter it again")
			return
		}
		key = string(k)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.scanner(endpoint, key).Check(ctx); err != nil {
		fail(w, http.StatusFailedDependency, "scanner_failed", "the scanner refused this address or key: "+err.Error())
		return
	}
	c := store.Scanner{Endpoint: endpoint}
	if in.APIKey != "" {
		c.Key = s.MFA.SealFor(scannerLabel(tenant), []byte(key))
		c.KeyHint = hint(key)
	}
	if err := s.Store.SetTenantScanner(r.Context(), tenant, c); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "scanner.set", tenant, map[string]any{"endpoint": endpoint, "key_changed": in.APIKey != ""})
	s.getScanner(w, r, u)
}

func (s *Server) deleteScanner(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	if err := s.Store.DeleteTenantScanner(r.Context(), tenant); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "scanner.delete", tenant, nil)
	w.WriteHeader(http.StatusNoContent)
}

// hint is the end of a key, to recognise it: never more than 4 characters,
// none for a key too short to spare them.
func hint(key string) string {
	if len(key) < 16 {
		return ""
	}
	return key[len(key)-4:]
}

func (s *Server) scanner(endpoint, key string) *cveradar.Client {
	return &cveradar.Client{Endpoint: endpoint, Key: key, HTTP: s.ScannerHTTP}
}

// ScannerFor opens a tenant's scanner; store.ErrNotFound without one.
func (s *Server) ScannerFor(ctx context.Context, tenant string) (*cveradar.Client, error) {
	c, err := s.Store.TenantScanner(ctx, tenant)
	if err != nil {
		return nil, err
	}
	key, err := s.MFA.OpenFor(scannerLabel(tenant), c.Key)
	if err != nil {
		return nil, err
	}
	return s.scanner(c.Endpoint, string(key)), nil
}
