package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/signing"
)

// internal requires the service token of gateway and tunnel-relay.
func (s *Server) internal(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.InternalToken == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(s.InternalToken)) != 1 {
			fail(w, http.StatusUnauthorized, "unauthorized", "")
			return
		}
		h(w, r)
	}
}

func (s *Server) internalMap(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.Published(r.Context(), r.PathValue("app"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	etag := `"v` + strconv.Itoa(v.Version) + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if v.Payload == nil || v.Signature == nil || v.KeyID == nil {
		fail(w, http.StatusInternalServerError, "internal", "published version without signature")
		return
	}
	writeJSON(w, http.StatusOK, signing.SignedMap{Version: v.Version, Payload: *v.Payload, Signature: *v.Signature, KeyID: *v.KeyID})
}

type relayConnector struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant"`
	App         string `json:"app"`
	TokenSHA256 string `json:"token_sha256"`
}

func (s *Server) internalConnectors(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.Connectors(r.Context(), "")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	out := make([]relayConnector, 0, len(cs))
	for _, c := range cs {
		out = append(out, relayConnector{ID: c.ID, Tenant: c.Tenant, App: c.App, TokenSHA256: c.TokenSHA256})
	}
	writeJSON(w, http.StatusOK, map[string]any{"connectors": out})
}
