package api_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// siteCert issues a site certificate from a company CA: the upload carries
// the CA too (internal sites), as PEM text.
func siteCert(t *testing.T, names ...string) string {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Acme CA"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour)}
	caDER, _ := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	ca, _ = x509.ParseCertificate(caDER)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(20 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := x509.MarshalPKCS8PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})) +
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})) +
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: k}))
}

func TestSiteCertificate(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "approver"}, nil)
	ap := e.join("ap@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "www.acme.test"}, nil)
	admin.must(200, "PUT", "/v1/apps/portal/aliases", map[string]any{"aliases": []string{"acme.test", "old.acme.test"}}, nil)

	type tlsView struct {
		TLS struct {
			PublicScheme string `json:"public_scheme"`
			Status       string
			Certificate  *struct {
				Domains, Missing []string
				KeyPEM           string `json:"key_pem"`
			}
		}
	}
	var a tlsView
	admin.must(200, "GET", "/v1/apps/portal", nil, &a)
	if a.TLS.PublicScheme != "https" || a.TLS.Status != "missing" {
		t.Fatalf("new site: %+v", a.TLS)
	}
	// Only an administrator, only the site's own certificate.
	ap.must(403, "PUT", "/v1/apps/portal/certificate", map[string]string{"pem": siteCert(t, "www.acme.test")}, nil)
	var refused struct{ Message string }
	admin.must(422, "PUT", "/v1/apps/portal/certificate", map[string]string{"pem": siteCert(t, "shop.example")}, &refused)
	if !strings.Contains(refused.Message, "not for www.acme.test") {
		t.Errorf("another site's certificate: %q", refused.Message)
	}
	admin.must(422, "PUT", "/v1/apps/portal/certificate", map[string]string{"pem": "garbage"}, nil)

	sent := len(e.mail.Messages())
	admin.must(200, "PUT", "/v1/apps/portal/certificate", map[string]string{"pem": siteCert(t, "www.acme.test", "acme.test")}, &a)
	if a.TLS.Status != "incomplete" || a.TLS.Certificate == nil || strings.Join(a.TLS.Certificate.Missing, ",") != "old.acme.test" ||
		a.TLS.Certificate.KeyPEM != "" {
		t.Fatalf("uploaded: %+v", a.TLS)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(e.mail.Messages()) == sent && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if m := e.mail.Messages(); len(m) == sent || m[len(m)-1].To != "boss@acme.test" || !strings.Contains(m[len(m)-1].Subject, "Certificate") {
		t.Errorf("the administrators are told: %d emails", len(m)-sent)
	}

	// Gateways get it, key in clear, once the site is published, for the
	// domains it covers.
	certsFeed := func() []struct {
		App     string
		Domains []string
		KeyPEM  string `json:"key_pem"`
	} {
		var out struct {
			Items []struct {
				App     string
				Domains []string
				KeyPEM  string `json:"key_pem"`
			}
		}
		_ = json.Unmarshal(internalGet(t, e, "/v1/internal/certificates", internalToken), &out)
		return out.Items
	}
	if len(certsFeed()) != 0 {
		t.Error("certificate of an unpublished site given to the gateways")
	}
	if internalStatus(t, e, "/v1/internal/certificates", "wrong-token") != 401 {
		t.Error("certificates without the service token")
	}
	admin.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)
	feed := certsFeed()
	if len(feed) != 1 || strings.Join(feed[0].Domains, ",") != "www.acme.test,acme.test" || !strings.Contains(feed[0].KeyPEM, "PRIVATE KEY") {
		t.Fatalf("feed: %+v", feed)
	}

	// Served in http: the published map says so at once, no certificate given.
	admin.must(200, "PATCH", "/v1/apps/portal", map[string]string{"public_scheme": "http"}, &a)
	if a.TLS.Status != "not_needed" || len(certsFeed()) != 0 {
		t.Errorf("http: %+v, feed %d", a.TLS, len(certsFeed()))
	}
	var signed struct{ Payload string }
	_ = json.Unmarshal(internalGet(t, e, "/v1/internal/apps/portal/map", internalToken), &signed)
	raw, _ := base64.StdEncoding.DecodeString(signed.Payload)
	if !strings.Contains(string(raw), `"public_scheme":"http"`) {
		t.Error("public_scheme not in the published map")
	}
	admin.must(422, "PATCH", "/v1/apps/portal", map[string]string{"public_scheme": "ftp"}, nil)
	admin.must(200, "PATCH", "/v1/apps/portal", map[string]string{"public_scheme": "https"}, nil)
	admin.must(200, "DELETE", "/v1/apps/portal/certificate", nil, &a)
	if a.TLS.Status != "missing" || len(certsFeed()) != 0 {
		t.Errorf("removed: %+v", a.TLS)
	}
}
