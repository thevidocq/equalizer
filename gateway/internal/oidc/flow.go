package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Random returns a URL-safe random string (state, nonce, PKCE verifier).
func Random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Challenge is the PKCE S256 challenge of a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthURL is where the browser goes to log in.
func (p *Provider) AuthURL(ctx context.Context, clientID, redirect, state, nonce, verifier string, scopes []string) (string, error) {
	m, err := p.Metadata(ctx)
	if err != nil {
		return "", err
	}
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}
	hasOpenID := false
	for _, s := range scopes {
		hasOpenID = hasOpenID || s == "openid"
	}
	if !hasOpenID {
		scopes = append([]string{"openid"}, scopes...)
	}
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"scope": {strings.Join(scopes, " ")}, "state": {state}, "nonce": {nonce},
		"code_challenge": {Challenge(verifier)}, "code_challenge_method": {"S256"},
	}
	sep := "?"
	if strings.Contains(m.AuthURL, "?") {
		sep = "&"
	}
	return m.AuthURL + sep + q.Encode(), nil
}

// Exchange trades the code for the ID token (secret optional: public
// clients rely on PKCE alone).
func (p *Provider) Exchange(ctx context.Context, clientID, secret, code, redirect, verifier string) (string, error) {
	m, err := p.Metadata(ctx)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect},
		"client_id": {clientID}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if secret != "" {
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(secret))
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()
	var out struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out)
	if resp.StatusCode != http.StatusOK || out.IDToken == "" {
		return "", fmt.Errorf("%w: token endpoint answered %d %s", ErrProvider, resp.StatusCode, out.Error)
	}
	return out.IDToken, nil
}

// LogoutURL is the provider's end-session address, or "" if it has none.
func (p *Provider) LogoutURL(ctx context.Context, clientID, back string) string {
	m, err := p.Metadata(ctx)
	if err != nil || m.EndSession == "" || !strings.HasPrefix(m.EndSession, "https://") {
		return ""
	}
	q := url.Values{"client_id": {clientID}, "post_logout_redirect_uri": {back}}
	sep := "?"
	if strings.Contains(m.EndSession, "?") {
		sep = "&"
	}
	return m.EndSession + sep + q.Encode()
}

// RoleFor maps the user's groups to a role of the map: the first rule whose
// group the user has wins; "*" matches every authenticated user.
func RoleFor(rules []RoleRule, groups []string) string {
	for _, r := range rules {
		if r.Group == "*" {
			return r.Role
		}
		for _, g := range groups {
			if g == r.Group {
				return r.Role
			}
		}
	}
	return ""
}

// RoleRule is one group → role mapping of the map.
type RoleRule struct{ Group, Role string }
