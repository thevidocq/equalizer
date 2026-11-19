// Package oidc logs visitors in with the organisation's identity provider
// (OpenID Connect, authorization code flow with PKCE) before the gateway
// serves a site. Only the standard library: the ID token is verified here
// (signature with the provider's published keys, issuer, audience, expiry,
// nonce). Tokens and claims are never logged.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrProvider means the identity provider could not be used.
var ErrProvider = errors.New("identity provider unavailable")

const (
	metaTTL     = time.Hour
	keysRefetch = 5 * time.Minute // at most once per this, on an unknown key id
	maxBody     = 1 << 20
)

// Metadata is the part of the discovery document the gateway uses.
type Metadata struct {
	Issuer     string `json:"issuer"`
	AuthURL    string `json:"authorization_endpoint"`
	TokenURL   string `json:"token_endpoint"`
	JWKSURL    string `json:"jwks_uri"`
	EndSession string `json:"end_session_endpoint"`
}

// Provider is one issuer with its cached metadata and keys.
type Provider struct {
	Issuer string
	Client *http.Client
	Now    func() time.Time

	mu     sync.Mutex
	meta   *Metadata
	metaAt time.Time
	keys   map[string]any // kid → *rsa.PublicKey or *ecdsa.PublicKey
	keysAt time.Time
}

func (p *Provider) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (p *Provider) getJSON(ctx context.Context, url string, out any) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("%w: %s is not https", ErrProvider, url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s answered %d", ErrProvider, url, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out)
}

// Metadata returns the discovery document, cached for an hour.
func (p *Provider) Metadata(ctx context.Context) (*Metadata, error) {
	p.mu.Lock()
	if p.meta != nil && p.now().Sub(p.metaAt) < metaTTL {
		m := p.meta
		p.mu.Unlock()
		return m, nil
	}
	p.mu.Unlock()
	var m Metadata
	if err := p.getJSON(ctx, strings.TrimRight(p.Issuer, "/")+"/.well-known/openid-configuration", &m); err != nil {
		return nil, err
	}
	if m.Issuer != p.Issuer {
		return nil, fmt.Errorf("%w: discovery issuer %q is not the configured one", ErrProvider, m.Issuer)
	}
	for _, u := range []string{m.AuthURL, m.TokenURL, m.JWKSURL} {
		if !strings.HasPrefix(u, "https://") {
			return nil, fmt.Errorf("%w: endpoint %q is not https", ErrProvider, u)
		}
	}
	p.mu.Lock()
	p.meta, p.metaAt = &m, p.now()
	p.mu.Unlock()
	return &m, nil
}

// key returns the public key with that id, fetching the keys again when it
// is unknown (keys rotate), at most once every few minutes.
func (p *Provider) key(ctx context.Context, kid string) (any, error) {
	p.mu.Lock()
	k, ok := p.keys[kid]
	fresh := p.now().Sub(p.keysAt) < keysRefetch
	p.mu.Unlock()
	if ok {
		return k, nil
	}
	if fresh {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	m, err := p.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	var set jwks
	if err := p.getJSON(ctx, m.JWKSURL, &set); err != nil {
		return nil, err
	}
	keys := set.parse()
	p.mu.Lock()
	p.keys, p.keysAt = keys, p.now()
	p.mu.Unlock()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}
