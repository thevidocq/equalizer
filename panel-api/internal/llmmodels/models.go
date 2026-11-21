// Package llmmodels asks an AI provider which models a key may use, so the
// panel offers a list instead of a free text field. It reads only the list:
// no prompt, no data of the tenant is sent.
package llmmodels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
)

// Model is one model the key may use.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Errors the panel tells apart.
var (
	ErrKeyRefused  = errors.New("the provider refused the key")
	ErrUnreachable = errors.New("the provider could not be reached")
)

// Lister asks the providers. The zero value uses the public APIs and a client
// that never connects to a private address (a local endpoint is the tenant's
// own server, reached from the platform's network).
type Lister struct {
	Client       *http.Client
	AnthropicURL string // default https://api.anthropic.com
	OpenAIURL    string // default https://api.openai.com
}

// List returns the models, sorted, newest names first where the provider
// gives no order. endpoint is the base URL of an OpenAI-compatible server
// (local provider), e.g. https://llm.example.com/v1.
func (l Lister) List(ctx context.Context, provider, endpoint, key string) ([]Model, error) {
	var req *http.Request
	var err error
	switch provider {
	case "anthropic":
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, or(l.AnthropicURL, "https://api.anthropic.com")+"/v1/models?limit=100", nil)
		if err == nil {
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case "openai", "local":
		base := or(l.OpenAIURL, "https://api.openai.com") + "/v1"
		if provider == "local" {
			base = strings.TrimRight(endpoint, "/")
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
		if err == nil && key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	default:
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
	if err != nil {
		return nil, err
	}
	resp, err := l.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrKeyRefused
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: status %d", ErrUnreachable, resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: unreadable answer", ErrUnreachable)
	}
	out := make([]Model, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" || (provider == "openai" && !chatModel(m.ID)) {
			continue
		}
		out = append(out, Model{ID: m.ID, Name: or(m.DisplayName, m.ID)})
	}
	if provider != "anthropic" { // Anthropic lists newest first already
		sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	}
	return out, nil
}

// chatModel keeps OpenAI's text models: the list also has embeddings, audio,
// images and moderation models the agent cannot use.
func chatModel(id string) bool {
	for _, skip := range []string{"embedding", "whisper", "tts", "dall-e", "moderation", "audio", "realtime", "image", "transcribe", "search"} {
		if strings.Contains(id, skip) {
			return false
		}
	}
	return strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "o") || strings.HasPrefix(id, "chatgpt-")
}

func (l Lister) client() *http.Client {
	if l.Client != nil {
		return l.Client
	}
	d := &net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		DialContext: d.DialContext, TLSHandshakeTimeout: 5 * time.Second,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// publicOnly refuses to connect to a private, loopback or reserved address:
// a tenant's endpoint must not become a way into the platform's network.
func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if _, ok := exposure.PublicIP(host); !ok {
		return fmt.Errorf("refused to connect to %s: not a public address", host)
	}
	return nil
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
