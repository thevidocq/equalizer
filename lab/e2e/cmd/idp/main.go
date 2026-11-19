// Command idp is a fake OpenID Connect provider for the local lab: it logs
// in, without a password, the user picked on its page (with their groups),
// and signs ID tokens with an ES256 key made at start. Never outside the lab.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type user struct {
	Sub, Email, Name string
	Groups           []string
}

var users = map[string]user{
	"alice": {"u-alice", "alice@acme.test", "Alice Editor", []string{"Blog editors"}},
	"bob":   {"u-bob", "bob@acme.test", "Bob Outsider", []string{"Visitors"}},
}

type grant struct {
	user                  user
	nonce, challenge, aud string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18095", "listen address")
	issuer := flag.String("issuer", "https://idp.lab.test", "issuer")
	flag.Parse()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var mu sync.Mutex
	codes := map[string]grant{}
	b64 := base64.RawURLEncoding.EncodeToString
	pad := func(n *big.Int) []byte { b := make([]byte, 32); n.FillBytes(b); return b }

	http.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": *issuer, "authorization_endpoint": *issuer + "/authorize",
			"token_endpoint": *issuer + "/token", "jwks_uri": *issuer + "/jwks", "end_session_endpoint": *issuer + "/logout"})
	})
	http.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "EC", "crv": "P-256", "kid": "lab", "use": "sig",
			"x": b64(pad(key.PublicKey.X)), "y": b64(pad(key.PublicKey.Y))}}})
	})
	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		name := r.FormValue("as")
		u, ok := users[name]
		if !ok { // the login page: pick a user
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<h1>Lab identity provider</h1>")
			for n, u := range users {
				q.Set("as", n)
				fmt.Fprintf(w, `<p><a id="as-%s" href="/authorize?%s">Log in as %s (%s)</a></p>`, n, html.EscapeString(q.Encode()), u.Name, strings.Join(u.Groups, ", "))
			}
			return
		}
		code := b64(randomBytes())
		mu.Lock()
		codes[code] = grant{u, q.Get("nonce"), q.Get("code_challenge"), q.Get("client_id")}
		mu.Unlock()
		back, _ := url.Parse(q.Get("redirect_uri"))
		bq := back.Query()
		bq.Set("code", code)
		bq.Set("state", q.Get("state"))
		back.RawQuery = bq.Encode()
		http.Redirect(w, r, back.String(), http.StatusFound)
	})
	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		g, ok := codes[r.FormValue("code")]
		delete(codes, r.FormValue("code"))
		mu.Unlock()
		sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
		if !ok || b64(sum[:]) != g.challenge {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		head, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": "lab", "typ": "JWT"})
		now := time.Now().Unix()
		claims, _ := json.Marshal(map[string]any{"iss": *issuer, "aud": g.aud, "sub": g.user.Sub, "email": g.user.Email,
			"name": g.user.Name, "groups": g.user.Groups, "nonce": g.nonce, "iat": now, "exp": now + 600})
		signed := b64(head) + "." + b64(claims)
		digest := sha256.Sum256([]byte(signed))
		rr, ss, _ := ecdsa.Sign(rand.Reader, key, digest[:])
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed + "." + b64(append(pad(rr), pad(ss)...)), "token_type": "Bearer"})
	})
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		back := r.URL.Query().Get("post_logout_redirect_uri")
		if !strings.HasPrefix(back, "https://") {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusFound)
	})
	log.Printf("lab identity provider on %s (%s)", *addr, *issuer)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func randomBytes() []byte {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return b
}
