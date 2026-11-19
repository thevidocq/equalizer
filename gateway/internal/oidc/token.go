package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	_ "crypto/sha256" // hash registration
	_ "crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// ErrToken means the ID token is not valid: never trust it.
var ErrToken = errors.New("invalid ID token")

const skew = time.Minute

type jwks struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	} `json:"keys"`
}

func b64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// parse keeps the signing keys it understands: RSA and EC P-256/P-384.
func (j jwks) parse() map[string]any {
	out := map[string]any{}
	for _, k := range j.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		switch k.Kty {
		case "RSA":
			n, err1 := b64(k.N)
			e, err2 := b64(k.E)
			if err1 != nil || err2 != nil || len(n) < 256 || len(e) == 0 || len(e) > 4 {
				continue // RSA keys under 2048 bits are refused
			}
			out[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		case "EC":
			var c elliptic.Curve
			switch k.Crv {
			case "P-256":
				c = elliptic.P256()
			case "P-384":
				c = elliptic.P384()
			default:
				continue
			}
			x, err1 := b64(k.X)
			y, err2 := b64(k.Y)
			if err1 != nil || err2 != nil {
				continue
			}
			pub := &ecdsa.PublicKey{Curve: c, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
			if !c.IsOnCurve(pub.X, pub.Y) {
				continue
			}
			out[k.Kid] = pub
		}
	}
	return out
}

// Claims are the ID token's claims the gateway uses.
type Claims struct {
	Subject string
	Email   string
	Groups  []string
}

// Verify checks an ID token for this client and nonce.
func (p *Provider) Verify(ctx context.Context, raw, clientID, nonce, groupsClaim string) (Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: not a JWS", ErrToken)
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	hb, err := b64(parts[0])
	if err != nil || json.Unmarshal(hb, &hdr) != nil {
		return Claims{}, fmt.Errorf("%w: header", ErrToken)
	}
	sig, err := b64(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: signature encoding", ErrToken)
	}
	key, err := p.key(ctx, hdr.Kid)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if err := verifySig(hdr.Alg, key, []byte(parts[0]+"."+parts[1]), sig); err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	pb, err := b64(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: payload", ErrToken)
	}
	var c map[string]any
	if err := json.Unmarshal(pb, &c); err != nil {
		return Claims{}, fmt.Errorf("%w: payload", ErrToken)
	}
	return p.checkClaims(c, clientID, nonce, groupsClaim)
}

func (p *Provider) checkClaims(c map[string]any, clientID, nonce, groupsClaim string) (Claims, error) {
	if iss, _ := c["iss"].(string); iss != p.Issuer {
		return Claims{}, fmt.Errorf("%w: issuer", ErrToken)
	}
	var aud []string
	switch a := c["aud"].(type) {
	case string:
		aud = []string{a}
	case []any:
		for _, x := range a {
			if s, ok := x.(string); ok {
				aud = append(aud, s)
			}
		}
	}
	found := false
	for _, a := range aud {
		found = found || a == clientID
	}
	if azp, _ := c["azp"].(string); !found || (len(aud) > 1 && azp != clientID) {
		return Claims{}, fmt.Errorf("%w: audience", ErrToken)
	}
	now := p.now()
	exp, ok := c["exp"].(float64)
	if !ok || now.After(time.Unix(int64(exp), 0).Add(skew)) {
		return Claims{}, fmt.Errorf("%w: expired", ErrToken)
	}
	if iat, ok := c["iat"].(float64); ok && time.Unix(int64(iat), 0).After(now.Add(skew)) {
		return Claims{}, fmt.Errorf("%w: issued in the future", ErrToken)
	}
	if n, _ := c["nonce"].(string); subtle.ConstantTimeCompare([]byte(n), []byte(nonce)) != 1 || nonce == "" {
		return Claims{}, fmt.Errorf("%w: nonce", ErrToken)
	}
	out := Claims{}
	out.Subject, _ = c["sub"].(string)
	if out.Subject == "" {
		return Claims{}, fmt.Errorf("%w: no subject", ErrToken)
	}
	if v, _ := c["email_verified"].(bool); v || c["email_verified"] == nil {
		out.Email, _ = c["email"].(string)
	}
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	switch g := c[groupsClaim].(type) {
	case string:
		out.Groups = []string{g}
	case []any:
		for _, x := range g {
			if s, ok := x.(string); ok {
				out.Groups = append(out.Groups, s)
			}
		}
	}
	return out, nil
}

func verifySig(alg string, key any, signed, sig []byte) error {
	var h crypto.Hash
	switch alg {
	case "RS256", "PS256", "ES256":
		h = crypto.SHA256
	case "RS384", "PS384", "ES384":
		h = crypto.SHA384
	case "RS512", "PS512":
		h = crypto.SHA512
	default:
		return fmt.Errorf("algorithm %q not accepted", alg) // never "none" or HMAC
	}
	d := h.New()
	d.Write(signed)
	sum := d.Sum(nil)
	switch k := key.(type) {
	case *rsa.PublicKey:
		if alg[0] == 'R' {
			return rsa.VerifyPKCS1v15(k, h, sum, sig)
		}
		if alg[0] == 'P' {
			return rsa.VerifyPSS(k, h, sum, sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		}
	case *ecdsa.PublicKey:
		size := (k.Curve.Params().BitSize + 7) / 8
		if alg[0] != 'E' || len(sig) != 2*size || (alg == "ES256") != (size == 32) {
			return errors.New("EC signature does not match the key")
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if ecdsa.Verify(k, sum, r, s) {
			return nil
		}
		return errors.New("bad signature")
	}
	return errors.New("algorithm does not match the key")
}
