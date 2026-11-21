package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/certs"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The site's own certificate: the legacy site's, uploaded by a tenant
// administrator. The platform never issues one for a protected site.

func certLabel(app string) string { return "site-certificate:" + app }

func (s *Server) putCertificate(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		PEM      string `json:"pem"`
		PFX      string `json:"pfx"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	var pfx []byte
	if in.PFX != "" {
		var err error
		if pfx, err = base64.StdEncoding.DecodeString(in.PFX); err != nil {
			fail(w, http.StatusUnprocessableEntity, "invalid", "pfx: the file in base64")
			return
		}
	}
	p, err := certs.Parse(in.PEM, pfx, in.Password, s.CertRoots, time.Now())
	var ce *certs.Error
	if errors.As(err, &ce) {
		fail(w, http.StatusUnprocessableEntity, "certificate_refused", ce.Msg)
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if missing := certs.Covers(p.Leaf, []string{a.PublicDomain}); len(missing) > 0 {
		fail(w, http.StatusUnprocessableEntity, "certificate_refused",
			fmt.Sprintf("the certificate is for %s, not for %s", strings.Join(p.Info.Domains, ", "), a.PublicDomain))
		return
	}
	err = s.Store.PutCertificate(r.Context(), a.ID, store.NewCertificate{
		ChainPEM: p.ChainPEM, KeySealed: s.MFA.SealFor(certLabel(a.ID), []byte(p.KeyPEM)), Subject: p.Info.Subject,
		Issuer: p.Info.Issuer, Domains: p.Info.Domains, NotBefore: p.Info.NotBefore, NotAfter: p.Info.NotAfter,
		Fingerprint: p.Info.Fingerprint,
	}, u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.certificate", a.ID,
		map[string]any{"fingerprint_sha256": p.Info.Fingerprint, "not_after": p.Info.NotAfter, "private_ca": p.Info.PrivateCA})
	go s.tellAdmins(context.WithoutCancel(r.Context()), a, fmt.Sprintf(
		"%s uploaded a new certificate for %s.\n\nIssued by: %s\nValid until: %s\nSHA-256 fingerprint: %s\n",
		u.Email, a.PublicDomain, p.Info.Issuer, p.Info.NotAfter.UTC().Format(time.DateOnly), p.Info.Fingerprint))
	s.writeApp(w, r, a.ID)
}

func (s *Server) deleteCertificate(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if a.TLS.Certificate == nil {
		s.writeApp(w, r, a.ID)
		return
	}
	if err := s.Store.DeleteCertificate(r.Context(), a.ID); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.certificate_removed", a.ID, nil)
	go s.tellAdmins(context.WithoutCancel(r.Context()), a, fmt.Sprintf(
		"%s removed the certificate of %s: visitors no longer reach it in HTTPS until a new one is uploaded.\n",
		u.Email, a.PublicDomain))
	s.writeApp(w, r, a.ID)
}

// tellAdmins emails every administrator of the tenant about a change of the
// site's certificate: it decides whether visitors can reach the site.
func (s *Server) tellAdmins(ctx context.Context, a store.App, body string) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	users, err := s.Store.Users(ctx, a.Tenant)
	if err != nil {
		return
	}
	for _, x := range users {
		if x.TenantRole == nil || *x.TenantRole != "admin" {
			continue
		}
		msg := mail.Message{To: x.Email, Subject: "Certificate of " + a.PublicDomain + " changed",
			Body: "Hello " + x.Name + ",\n\n" + body + "\nIf this was not expected, check the site in the Equalizer panel.\n"}
		if err := s.Mail.Send(ctx, msg); err != nil && !errors.Is(err, mail.ErrDisabled) {
			s.Log.WarnContext(ctx, "certificate email not sent", slog.String("error", err.Error()))
		}
	}
}

// setPublicScheme changes how visitors reach the site and signs the
// published map again with it (the gateway reads it from the map).
func (s *Server) setPublicScheme(r *http.Request, a store.App, scheme string) error {
	if scheme != "https" && scheme != "https_proxy" && scheme != "http" {
		return &inputError{"public_scheme: https, https_proxy or http"}
	}
	if err := s.republishMeta(r, a.ID, map[string]any{"public_scheme": scheme}); err != nil {
		return err
	}
	return s.Store.SetPublicScheme(r.Context(), a.ID, scheme)
}

// inputError is a value the client can fix (422).
type inputError struct{ msg string }

func (e *inputError) Error() string { return e.msg }

// internalCertificates gives the gateways the certificates of published
// https sites, with their keys, for Caddy at TLS handshake.
func (s *Server) internalCertificates(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.PublishedCertificates(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	type item struct {
		App      string   `json:"app"`
		Domains  []string `json:"domains"`
		ChainPEM string   `json:"chain_pem"`
		KeyPEM   string   `json:"key_pem"`
	}
	out := make([]item, 0, len(all))
	for _, c := range all {
		key, err := s.MFA.OpenFor(certLabel(c.App), c.KeySealed)
		if err != nil {
			s.Log.ErrorContext(r.Context(), "site certificate key cannot be opened", "app", c.App)
			continue
		}
		leaf, err := certs.LeafFromChain(c.ChainPEM)
		if err != nil {
			continue
		}
		var covered []string
		for _, d := range c.Domains {
			if len(certs.Covers(leaf, []string{d})) == 0 {
				covered = append(covered, d)
			}
		}
		out = append(out, item{App: c.App, Domains: covered, ChainPEM: c.ChainPEM, KeyPEM: string(key)})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
