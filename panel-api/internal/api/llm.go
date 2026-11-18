package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The tenant's own AI model. The platform has no AI settings: each tenant
// brings its provider and key, and nobody else reads the key.

type llmView struct {
	Provider  *string `json:"provider"`
	Model     *string `json:"model"`
	Endpoint  *string `json:"endpoint"`
	HasAPIKey bool    `json:"has_api_key"`
}

func llmLabel(tenant string) string { return "llm:" + tenant }

func (s *Server) getLLM(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	l, err := s.Store.TenantLLM(r.Context(), tenant)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, llmView{})
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, llmView{Provider: &l.Provider, Model: &l.Model, Endpoint: l.Endpoint, HasAPIKey: len(l.Key) > 0})
}

func (s *Server) putLLM(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	var in struct {
		Provider string  `json:"provider"`
		Model    string  `json:"model"`
		Endpoint *string `json:"endpoint"`
		APIKey   string  `json:"api_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Model, in.APIKey = strings.TrimSpace(in.Model), strings.TrimSpace(in.APIKey)
	if in.Model == "" || len(in.Model) > 100 || len(in.APIKey) > 300 || strings.ContainsAny(in.APIKey+in.Model, "\r\n") {
		fail(w, http.StatusUnprocessableEntity, "invalid", "model is required; the key has at most 300 characters")
		return
	}
	l := store.LLM{Provider: in.Provider, Model: in.Model}
	switch in.Provider {
	case "anthropic", "openai":
	case "local":
		if in.Endpoint == nil || !validEndpoint(*in.Endpoint) {
			fail(w, http.StatusUnprocessableEntity, "invalid", "endpoint: the https URL of an OpenAI-compatible server")
			return
		}
		e := strings.TrimRight(strings.TrimSpace(*in.Endpoint), "/")
		l.Endpoint = &e
	default:
		fail(w, http.StatusUnprocessableEntity, "invalid", "provider: anthropic, openai or local")
		return
	}
	if in.APIKey != "" {
		l.Key = s.MFA.SealFor(llmLabel(tenant), []byte(in.APIKey))
	} else if in.Provider != "local" {
		old, err := s.Store.TenantLLM(r.Context(), tenant)
		if err != nil || len(old.Key) == 0 {
			fail(w, http.StatusUnprocessableEntity, "invalid", "api_key is required for this provider")
			return
		}
	}
	if err := s.Store.SetTenantLLM(r.Context(), tenant, l); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "llm.set", in.Provider, map[string]any{"model": in.Model, "key_changed": in.APIKey != ""})
	s.getLLM(w, r, u)
}

func (s *Server) deleteLLM(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	if err := s.Store.DeleteTenantLLM(r.Context(), tenant); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "llm.delete", tenant, nil)
	w.WriteHeader(http.StatusNoContent)
}

// validEndpoint accepts https URLs without credentials. The agent checks
// again, when connecting, that the host is not a private address.
func validEndpoint(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && len(raw) <= 300
}
