package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/signing"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

const maxAliases = 10

// putAliases sets the other domains of a site and signs the published map
// again with them, so the change applies without a new version.
func (s *Server) putAliases(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		Aliases []string `json:"aliases"`
	}
	if !decode(w, r, &in) {
		return
	}
	var out []string
	for _, d := range in.Aliases {
		d = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
		if d == "" || slices.Contains(out, d) {
			continue
		}
		if !domainRe.MatchString(d) || len(d) > 253 || d == a.PublicDomain {
			fail(w, http.StatusUnprocessableEntity, "invalid", "not another domain name of the site: "+d)
			return
		}
		taken, err := s.Store.DomainTaken(r.Context(), d, a.ID)
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		if taken {
			fail(w, http.StatusConflict, "domain_taken", d+" is already used by another site")
			return
		}
		out = append(out, d)
	}
	if len(out) > maxAliases {
		fail(w, http.StatusUnprocessableEntity, "invalid", "at most 10 other domains")
		return
	}
	if err := s.Store.SetAliases(r.Context(), a.ID, out); err != nil {
		s.storeError(w, r, err)
		return
	}
	if out == nil {
		out = []string{}
	}
	if err := s.republishAliases(r, a.ID, out); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.aliases", a.ID, map[string]any{"aliases": out})
	s.writeApp(w, r, a.ID)
}

// republishAliases signs the published version again with the new aliases
// in its meta; nothing to do before the first publication.
func (s *Server) republishAliases(r *http.Request, app string, aliases []string) error {
	return s.republishMeta(r, app, map[string]any{"aliases": aliases})
}

// republishMeta signs the published version again with new meta values
// (aliases, public scheme): settings of the site that apply at once.
func (s *Server) republishMeta(r *http.Request, app string, meta map[string]any) error {
	v, err := s.Store.Published(r.Context(), app)
	if err == store.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	doc, err := store.SetMeta(v.Document, meta)
	if err != nil {
		return err
	}
	if err := s.mapChecked(r.Context(), doc, false); err != nil {
		return err
	}
	signed, err := signing.Sign(doc, v.Version, s.SigningKey, s.KeyID)
	if err != nil {
		return err
	}
	return s.Store.Republish(r.Context(), app, v.Version, doc, signed.Payload, signed.Signature, signed.KeyID)
}
