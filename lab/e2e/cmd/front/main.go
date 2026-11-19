// Command front stands in for Caddy in the local lab: HTTPS on :443 with a
// certificate signed by a lab CA, the panel (static UI and /v1 API), the
// relay's tunnel path, the fake identity provider, and every other host to
// the gateway, with the client address in X-Forwarded-For. It never runs
// outside the lab.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", ":443", "HTTPS address")
	plain := flag.String("http", ":80", "HTTP address (redirects to HTTPS)")
	dir := flag.String("dir", ".run", "where the CA is written (ca.pem)")
	panelUI := flag.String("panel-ui", "", "built panel UI (dist)")
	panelAPI := flag.String("panel-api", "http://127.0.0.1:18085", "panel API")
	gateway := flag.String("gateway", "http://127.0.0.1:18080", "gateway public address")
	relay := flag.String("relay", "http://127.0.0.1:18081", "relay tunnel address")
	idp := flag.String("idp", "http://127.0.0.1:18095", "fake identity provider")
	domain := flag.String("domain", "lab.test", "lab domain (panel., relay., idp.)")
	sites := flag.String("sites", "acme.test", "comma-separated domains of the protected sites (and their subdomains)")
	flag.Parse()

	names := []string{*domain, "*." + *domain}
	for _, d := range strings.Split(*sites, ",") {
		if d = strings.TrimSpace(d); d != "" {
			names = append(names, d, "*."+d)
		}
	}
	cert, err := certificate(*dir, names)
	if err != nil {
		log.Fatal(err)
	}
	panel := panelHandler(*panelUI, proxy(*panelAPI, false))
	routes := map[string]http.Handler{
		"panel." + *domain: panel,
		"relay." + *domain: relayHandler(proxy(*relay, false)),
		"idp." + *domain:   proxy(*idp, false),
	}
	gw := proxy(*gateway, true)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if i := strings.LastIndexByte(host, ':'); i > 0 {
			host = host[:i]
		}
		if h := routes[host]; h != nil {
			h.ServeHTTP(w, r)
			return
		}
		gw.ServeHTTP(w, r)
	})
	go func() {
		log.Fatal(http.ListenAndServe(*plain, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusMovedPermanently)
		})))
	}()
	srv := &http.Server{Addr: *addr, Handler: h, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second}
	log.Printf("lab front on %s", *addr)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

// proxy forwards to target; for the gateway, X-Forwarded-For carries the
// client address as Caddy does (any value sent by the client is replaced).
func proxy(target string, forwarded bool) http.Handler {
	u, _ := url.Parse(target)
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Del("X-Forwarded-For")
			if forwarded {
				ip, _, _ := net.SplitHostPort(pr.In.RemoteAddr)
				pr.Out.Header.Set("X-Forwarded-For", ip)
			}
		},
		FlushInterval: -1,
	}
}

func relayHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tunnel" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// panelHandler serves the built UI and sends /v1 to the API, like panel-ui.
func panelHandler(dist string, api http.Handler) http.Handler {
	files := http.FileServer(http.Dir(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/internal/"):
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/v1/"):
			api.ServeHTTP(w, r)
		default:
			if _, err := os.Stat(filepath.Join(dist, filepath.Clean(r.URL.Path))); err != nil || r.URL.Path == "/" {
				w.Header().Set("Cache-Control", "no-store")
				http.ServeFile(w, r, filepath.Join(dist, "index.html"))
				return
			}
			files.ServeHTTP(w, r)
		}
	})
}

// certificate makes a lab CA and a certificate for the lab's names.
func certificate(dir string, names []string) (tls.Certificate, error) {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Equalizer lab CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour), IsCA: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true,
		SubjectKeyId: randomID()} // strict verifiers want key ids
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	ca, _ := x509.ParseCertificate(caDER)
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o644); err != nil {
		return tls.Certificate{}, err
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: names[0]},
		DNSNames: names, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der, caDER}, PrivateKey: key}, nil
}

func randomID() []byte {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b
}
