package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/thirdparty/pkcs12"
)

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

type issued struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func issue(t *testing.T, cn string, ca *issued, isCA bool, dns ...string) issued {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(90 * 24 * time.Hour), DNSNames: dns,
		BasicConstraintsValid: true, IsCA: isCA}
	if isCA {
		tpl.KeyUsage = x509.KeyUsageCertSign
	} else {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		tpl.KeyUsage = x509.KeyUsageDigitalSignature
	}
	parent, signer := tpl, key
	if ca != nil {
		parent, signer = ca.cert, ca.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, parent, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return issued{c, key}
}

func pemOf(certs ...*x509.Certificate) string {
	var b strings.Builder
	for _, c := range certs {
		_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	return b.String()
}

func keyPEM(k any) string {
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestParse(t *testing.T) {
	root := issue(t, "Acme Root CA", nil, true)
	inter := issue(t, "Acme Issuing CA", &root, true)
	leaf := issue(t, "www.acme.test", &inter, false, "www.acme.test", "acme.test")
	public := x509.NewCertPool()
	public.AddCert(root.cert)

	// Leaf, key and intermediate in any order, against a trusted root.
	p, err := Parse(keyPEM(leaf.key)+pemOf(inter.cert, leaf.cert), nil, "", public, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Info.PrivateCA || strings.Count(p.ChainPEM, "BEGIN CERTIFICATE") != 2 || !strings.Contains(p.KeyPEM, "PRIVATE KEY") {
		t.Errorf("chain or key: %+v", p.Info)
	}
	if m := Covers(p.Leaf, []string{"www.acme.test", "acme.test", "old.acme.test"}); len(m) != 1 || m[0] != "old.acme.test" {
		t.Errorf("missing: %v", m)
	}
	// A company's own CA, uploaded with the chain: accepted, the root not served.
	p, err = Parse(pemOf(leaf.cert, inter.cert, root.cert)+keyPEM(leaf.key), nil, "", x509.NewCertPool(), now)
	if err != nil || !p.Info.PrivateCA || strings.Count(p.ChainPEM, "BEGIN CERTIFICATE") != 2 {
		t.Fatalf("private CA: %v %+v", err, p)
	}
	// PKCS#12, modern and legacy (old IIS exports).
	for _, enc := range []*pkcs12.Encoder{pkcs12.Modern2023, pkcs12.LegacyRC2} {
		pfx, err := enc.Encode(leaf.key, leaf.cert, []*x509.Certificate{inter.cert}, "S3cret!")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse("", pfx, "S3cret!", public, now); err != nil {
			t.Errorf("pfx: %v", err)
		}
		if _, err := Parse("", pfx, "wrong", public, now); err == nil || !strings.Contains(err.Error(), "wrong password") {
			t.Errorf("pfx with a wrong password: %v", err)
		}
	}

	other := issue(t, "other", &inter, false, "other.test")
	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	for what, c := range map[string]struct {
		pem, want string
	}{
		"no intermediate":  {keyPEM(leaf.key) + pemOf(leaf.cert), "chain is incomplete"},
		"key of another":   {keyPEM(other.key) + pemOf(leaf.cert, inter.cert), "no certificate matches"},
		"no key":           {pemOf(leaf.cert, inter.cert), "no private key"},
		"two keys":         {keyPEM(leaf.key) + keyPEM(other.key) + pemOf(leaf.cert), "more than one private key"},
		"weak key":         {keyPEM(weak) + pemOf(leaf.cert), "at least 2048"},
		"encrypted pkcs#8": {"-----BEGIN ENCRYPTED PRIVATE KEY-----\nAAAA\n-----END ENCRYPTED PRIVATE KEY-----\n" + pemOf(leaf.cert), "encrypted"},
	} {
		_, err := Parse(c.pem, nil, "", public, now)
		var ce *Error
		if !errors.As(err, &ce) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", what, err, c.want)
		}
	}
	if _, err := Parse(keyPEM(leaf.key)+pemOf(leaf.cert, inter.cert), nil, "", public, now.Add(200*24*time.Hour)); err == nil ||
		!strings.Contains(err.Error(), "not valid now") {
		t.Errorf("expired: %v", err)
	}
}
