package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
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

// internalMaps returns every published map: the gateway serves each one on
// its meta.public_domain. The ETag covers the whole set.
func (s *Server) internalMaps(w http.ResponseWriter, r *http.Request) {
	vs, err := s.Store.AllPublished(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	h := sha256.New()
	items := make([]signing.SignedMap, 0, len(vs))
	for _, v := range vs {
		fmt.Fprintf(h, "%s:%d:%s:%s;", v.App, v.Version, v.KeyID, v.Signature) // a re-signed version changes it too
		items = append(items, signing.SignedMap{Version: v.Version, Payload: v.Payload, Signature: v.Signature, KeyID: v.KeyID})
	}
	etag := `"` + hex.EncodeToString(h.Sum(nil))[:32] + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type relayConnector struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant"`
	TokenSHA256 string `json:"token_sha256"`
}

func (s *Server) internalConnectors(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.AllConnectors(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	out := make([]relayConnector, 0, len(cs))
	for _, c := range cs {
		out = append(out, relayConnector{ID: c.ID, Tenant: c.Tenant, TokenSHA256: c.TokenSHA256})
	}
	writeJSON(w, http.StatusOK, map[string]any{"connectors": out})
}
