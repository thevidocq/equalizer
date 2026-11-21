package store

import (
	"context"
	"time"

	"github.com/lib/pq"

	"github.com/thevidocq/equalizer/panel-api/internal/certs"
)

// SiteTLS is how visitors reach a site and its certificate (contract:
// SiteTLS in panel-api.yaml).
type SiteTLS struct {
	PublicScheme string       `json:"public_scheme"`
	Status       string       `json:"status"` // ok, missing, incomplete, expiring, expired, not_needed, at_proxy
	Certificate  *Certificate `json:"certificate"`
}

// Certificate describes an uploaded certificate, never its key.
type Certificate struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	Domains     []string  `json:"domains"`
	Missing     []string  `json:"missing,omitempty"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Fingerprint string    `json:"fingerprint_sha256"`
	UploadedAt  time.Time `json:"uploaded_at"`
	UploadedBy  *string   `json:"uploaded_by,omitempty"`
	ChainPEM    string    `json:"-"`
}

// ExpiringWithin is when a certificate starts to show as expiring.
const ExpiringWithin = 30 * 24 * time.Hour

// NewCertificate is what PutCertificate keeps.
type NewCertificate struct {
	ChainPEM, Subject, Issuer, Fingerprint string
	KeySealed                              []byte
	Domains                                []string
	NotBefore, NotAfter                    time.Time
}

// SetPublicScheme sets how visitors reach the site: https or http.
func (s *Store) SetPublicScheme(ctx context.Context, app, scheme string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE apps SET public_scheme = $2 WHERE id = $1`, app, scheme)
	return affected(res, err)
}

// PutCertificate keeps a site's certificate, replacing the previous one.
func (s *Store) PutCertificate(ctx context.Context, app string, c NewCertificate, by int64) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO app_certificates (app_id, chain_pem, key_sealed, subject, issuer, domains, not_before, not_after,
			fingerprint, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (app_id) DO UPDATE SET chain_pem = $2, key_sealed = $3, subject = $4, issuer = $5, domains = $6,
			not_before = $7, not_after = $8, fingerprint = $9, uploaded_by = $10, uploaded_at = now()`,
		app, c.ChainPEM, c.KeySealed, c.Subject, c.Issuer, pq.Array(c.Domains), c.NotBefore, c.NotAfter, c.Fingerprint, by)
	return err
}

// DeleteCertificate removes a site's certificate.
func (s *Store) DeleteCertificate(ctx context.Context, app string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM app_certificates WHERE app_id = $1`, app)
	return err
}

// SiteCertificate is a certificate with its sealed key, for the gateways.
type SiteCertificate struct {
	App, ChainPEM string
	Domains       []string // the site's domains: public domain first, then aliases
	KeySealed     []byte
}

// PublishedCertificates lists the certificates of published https sites.
func (s *Store) PublishedCertificates(ctx context.Context) ([]SiteCertificate, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, c.chain_pem, c.key_sealed,
			ARRAY[a.public_domain] || ARRAY(SELECT x.domain FROM app_aliases x WHERE x.app_id = a.id ORDER BY x.domain)
		FROM apps a JOIN app_certificates c ON c.app_id = a.id
		WHERE a.public_scheme = 'https'
			AND EXISTS (SELECT 1 FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published')
		ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteCertificate
	for rows.Next() {
		var c SiteCertificate
		if err := rows.Scan(&c.App, &c.ChainPEM, &c.KeySealed, pq.Array(&c.Domains)); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fillTLS sets the status of a site's certificate and the domains it misses.
func fillTLS(a *App, now time.Time) {
	t := &a.TLS
	switch {
	case t.PublicScheme == "http":
		t.Status = "not_needed"
	case t.PublicScheme == "https_proxy":
		t.Status = "at_proxy"
	case t.Certificate == nil:
		t.Status = "missing"
	case now.After(t.Certificate.NotAfter):
		t.Status = "expired"
	default:
		t.Status = "ok"
		if leaf, err := certs.LeafFromChain(t.Certificate.ChainPEM); err == nil {
			t.Certificate.Missing = certs.Covers(leaf, append([]string{a.PublicDomain}, a.Aliases...))
		}
		if len(t.Certificate.Missing) > 0 {
			t.Status = "incomplete"
		} else if t.Certificate.NotAfter.Sub(now) < ExpiringWithin {
			t.Status = "expiring"
		}
	}
}
