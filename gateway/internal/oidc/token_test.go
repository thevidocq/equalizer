package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func enc(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestVerify(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Unix(1_800_000_000, 0)
	p := &Provider{Issuer: "https://idp.example", Now: func() time.Time { return now },
		keys: map[string]any{"e1": &k.PublicKey}, keysAt: now}
	sign := func(hdr, claims map[string]any) string {
		s := enc(hdr) + "." + enc(claims)
		sum := sha256.Sum256([]byte(s))
		r, sv, _ := ecdsa.Sign(rand.Reader, k, sum[:])
		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		sv.FillBytes(sig[32:])
		return s + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	good := map[string]any{"iss": "https://idp.example", "aud": []any{"app"}, "sub": "u1", "exp": float64(now.Add(time.Hour).Unix()),
		"nonce": "n1", "roles": []any{"a", "b"}}
	hdr := map[string]any{"alg": "ES256", "kid": "e1"}
	c, err := p.Verify(context.Background(), sign(hdr, good), "app", "n1", "roles")
	if err != nil || c.Subject != "u1" || len(c.Groups) != 2 {
		t.Fatalf("good token: %+v %v", c, err)
	}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range good {
			m[a] = b
		}
		m[k] = v
		return m
	}
	for name, tok := range map[string]string{
		"wrong issuer":   sign(hdr, with("iss", "https://evil")),
		"wrong audience": sign(hdr, with("aud", "other")),
		"two audiences":  sign(hdr, with("aud", []any{"app", "other"})),
		"expired":        sign(hdr, with("exp", float64(now.Add(-time.Hour).Unix()))),
		"wrong nonce":    sign(hdr, with("nonce", "n2")),
		"no subject":     sign(hdr, with("sub", "")),
		"alg none":       enc(map[string]any{"alg": "none", "kid": "e1"}) + "." + enc(good) + ".",
		"HMAC":           strings.Replace(sign(hdr, good), enc(hdr), enc(map[string]any{"alg": "HS256", "kid": "e1"}), 1),
		"unknown key":    sign(map[string]any{"alg": "ES256", "kid": "zz"}, good),
		"payload edited": strings.Replace(sign(hdr, good), enc(good), enc(with("sub", "admin")), 1),
	} {
		if _, err := p.Verify(context.Background(), tok, "app", "n1", "roles"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRoleFor(t *testing.T) {
	rules := []RoleRule{{"admins", "admin"}, {"*", "user"}}
	if RoleFor(rules, []string{"admins"}) != "admin" || RoleFor(rules, nil) != "user" || RoleFor(rules[:1], []string{"x"}) != "" {
		t.Error("roles")
	}
}
