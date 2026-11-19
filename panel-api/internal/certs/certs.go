// Package certs reads and checks the certificate a customer uploads for a
// site: the legacy site's own certificate, as PEM blocks or a PKCS#12 file
// (.pfx, as IIS exports it). Nothing is kept unless the key matches, the
// chain is complete and the certificate is valid now.
package certs

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/thirdparty/pkcs12"
)

// Error is a refusal the customer can act on; its message says what to fix.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func refuse(format string, a ...any) error { return &Error{Msg: fmt.Sprintf(format, a...)} }

// Info describes a certificate, for the panel.
type Info struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	Domains     []string  `json:"domains"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Fingerprint string    `json:"fingerprint_sha256"`
	PrivateCA   bool      `json:"-"` // trusted through a CA of the upload, not a public one
}

// Parsed is a checked certificate ready to keep: the chain to serve (leaf
// first, without the root) and the private key, both in PEM.
type Parsed struct {
	Leaf     *x509.Certificate
	ChainPEM string
	KeyPEM   string
	Info     Info
}

// Parse reads PEM text or PKCS#12 bytes (one of the two) and checks them.
// roots nil means the system's trusted CAs.
func Parse(pemText string, pfx []byte, password string, roots *x509.CertPool, now time.Time) (*Parsed, error) {
	var (
		key   crypto.PrivateKey
		certs []*x509.Certificate
		err   error
	)
	switch {
	case len(pfx) > 0 && pemText == "":
		key, certs, err = fromPFX(pfx, password)
	case pemText != "" && len(pfx) == 0:
		key, certs, err = fromPEM(pemText, password)
	default:
		return nil, refuse("upload either the PEM text or the .pfx file")
	}
	if err != nil {
		return nil, err
	}
	if err := strongKey(key); err != nil {
		return nil, err
	}
	leaf, rest := splitLeaf(key, certs)
	if leaf == nil {
		return nil, refuse("no certificate matches the private key")
	}
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return nil, refuse("the certificate is not valid now (valid from %s to %s)",
			leaf.NotBefore.UTC().Format(time.DateOnly), leaf.NotAfter.UTC().Format(time.DateOnly))
	}
	chain, private, err := verify(leaf, rest, roots, now)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, refuse("unsupported private key")
	}
	var b strings.Builder
	for _, c := range chain {
		_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	sum := sha256.Sum256(leaf.Raw)
	return &Parsed{
		Leaf:     leaf,
		ChainPEM: b.String(),
		KeyPEM:   string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		Info: Info{Subject: leaf.Subject.String(), Issuer: leaf.Issuer.String(), Domains: names(leaf),
			NotBefore: leaf.NotBefore, NotAfter: leaf.NotAfter, Fingerprint: hex.EncodeToString(sum[:]), PrivateCA: private},
	}, nil
}

func fromPFX(data []byte, password string) (crypto.PrivateKey, []*x509.Certificate, error) {
	key, leaf, cas, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		if errors.Is(err, pkcs12.ErrIncorrectPassword) {
			return nil, nil, refuse("wrong password for the .pfx file")
		}
		return nil, nil, refuse("the .pfx file cannot be read: %v", err)
	}
	return key, append([]*x509.Certificate{leaf}, cas...), nil
}

func fromPEM(text, password string) (crypto.PrivateKey, []*x509.Certificate, error) {
	var (
		key   crypto.PrivateKey
		certs []*x509.Certificate
	)
	rest := []byte(text)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch {
		case block.Type == "CERTIFICATE":
			c, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, refuse("a certificate block cannot be read: %v", err)
			}
			certs = append(certs, c)
		case strings.HasSuffix(block.Type, "PRIVATE KEY"):
			if key != nil {
				return nil, nil, refuse("more than one private key: upload the site's key only")
			}
			k, err := parseKey(block, password)
			if err != nil {
				return nil, nil, err
			}
			key = k
		}
	}
	if key == nil {
		return nil, nil, refuse("no private key: add the site's private key (BEGIN PRIVATE KEY)")
	}
	if len(certs) == 0 {
		return nil, nil, refuse("no certificate: add the site's certificate (BEGIN CERTIFICATE)")
	}
	return key, certs, nil
}

func parseKey(block *pem.Block, password string) (crypto.PrivateKey, error) {
	der := block.Bytes
	if block.Type == "ENCRYPTED PRIVATE KEY" {
		return nil, refuse("the private key is encrypted (PKCS#8): upload it decrypted, or the .pfx file with its password")
	}
	//nolint:staticcheck // legacy encrypted PEM keys ("Proc-Type: 4,ENCRYPTED") are still what old servers keep
	if x509.IsEncryptedPEMBlock(block) {
		if password == "" {
			return nil, refuse("the private key is encrypted: enter its password")
		}
		var err error
		//nolint:staticcheck // see above
		if der, err = x509.DecryptPEMBlock(block, []byte(password)); err != nil {
			return nil, refuse("wrong password for the private key")
		}
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	return nil, refuse("the private key cannot be read")
}

// strongKey accepts what browsers accept for a site: RSA from 2048 bits,
// ECDSA on P-256 or P-384.
func strongKey(key crypto.PrivateKey) error {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		if k.N.BitLen() < 2048 {
			return refuse("the RSA key has %d bits: browsers need at least 2048", k.N.BitLen())
		}
	case *ecdsa.PrivateKey:
		if k.Curve != elliptic.P256() && k.Curve != elliptic.P384() {
			return refuse("the ECDSA key must be on P-256 or P-384")
		}
	default:
		return refuse("the key must be RSA or ECDSA")
	}
	return nil
}

// splitLeaf finds the certificate of the key; the others are the chain.
func splitLeaf(key crypto.PrivateKey, certs []*x509.Certificate) (*x509.Certificate, []*x509.Certificate) {
	pub := key.(interface{ Public() crypto.PublicKey }).Public()
	for i, c := range certs {
		if p, ok := c.PublicKey.(interface{ Equal(crypto.PublicKey) bool }); ok && p.Equal(pub) {
			rest := append(append([]*x509.Certificate{}, certs[:i]...), certs[i+1:]...)
			return c, rest
		}
	}
	return nil, nil
}

// verify builds the chain from the leaf to a trusted root and returns what
// to serve: the leaf and the intermediates, never the root. A root of the
// upload itself is accepted (a company's own CA, for internal sites).
func verify(leaf *x509.Certificate, others []*x509.Certificate, roots *x509.CertPool, now time.Time) ([]*x509.Certificate, bool, error) {
	inter := x509.NewCertPool()
	own := x509.NewCertPool()
	hasOwn := false
	for _, c := range others {
		if bytes.Equal(c.RawIssuer, c.RawSubject) && c.CheckSignatureFrom(c) == nil {
			own.AddCert(c)
			hasOwn = true
		} else {
			inter.AddCert(c)
		}
	}
	opts := x509.VerifyOptions{Intermediates: inter, Roots: roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	chains, err := leaf.Verify(opts)
	private := false
	if err != nil && hasOwn {
		opts.Roots = own
		chains, err = leaf.Verify(opts)
		private = err == nil
	}
	if err != nil {
		var unknown x509.UnknownAuthorityError
		if errors.As(err, &unknown) {
			return nil, false, refuse("the chain is incomplete: add the intermediate certificates of %s", leaf.Issuer.CommonName)
		}
		return nil, false, refuse("the certificate is not valid for a web site: %v", err)
	}
	chain := chains[0]
	return chain[:len(chain)-1], private, nil
}

func names(c *x509.Certificate) []string {
	if len(c.DNSNames) > 0 {
		return append([]string{}, c.DNSNames...)
	}
	return []string{c.Subject.CommonName}
}

// Covers lists the domains the certificate does not cover.
func Covers(leaf *x509.Certificate, domains []string) (missing []string) {
	for _, d := range domains {
		if leaf.VerifyHostname(d) != nil {
			missing = append(missing, d)
		}
	}
	return missing
}

// LeafFromChain reads the first certificate of a stored chain.
func LeafFromChain(chainPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(chainPEM))
	if block == nil {
		return nil, errors.New("empty chain")
	}
	return x509.ParseCertificate(block.Bytes)
}
