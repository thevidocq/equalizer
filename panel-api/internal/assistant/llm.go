// Package assistant answers a tenant's questions about one protected site:
// its map, its discovery report, its logs and its history. It talks to the
// tenant's own AI provider (Settings → AI) with tools that only read, and
// with one that proposes a change, which a person turns into a draft. It
// never publishes, and what the provider sees is masked (mask.go).
package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
)

// Call is one tool the model asks for.
type Call struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Result is what a tool answered, for the call of the same ID.
type Result struct {
	ID      string
	Content string
}

// Turn is one step of a conversation: a person's or the model's text, the
// model's tool calls, or the tools' results.
type Turn struct {
	Role    string // user, assistant, tool
	Text    string
	Calls   []Call
	Results []Result
}

// Tool is a tool the model may call: its input is a JSON schema.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
}

// Reply is the model's answer: text, or calls to make first. Cut: the
// provider stopped it at its length limit.
type Reply struct {
	Text  string
	Calls []Call
	Cut   bool
}

// Model answers a conversation.
type Model interface {
	Chat(ctx context.Context, system string, turns []Turn, tools []Tool) (Reply, error)
}

// Config is the tenant's provider, as Settings → AI saved it.
type Config struct {
	Provider string // anthropic, openai, local
	Model    string
	Endpoint string // local: an https OpenAI-compatible server
	Key      string
	// AnthropicURL, OpenAIURL replace the public APIs (tests).
	AnthropicURL, OpenAIURL string
	Client                  *http.Client // nil: public addresses only, no redirects
}

// ErrProvider is a refusal or failure of the provider, in words a person
// can act on (never the provider's own message).
type ErrProvider struct{ Msg string }

func (e *ErrProvider) Error() string { return e.Msg }

// New returns the tenant's model.
func New(c Config) (Model, error) {
	if c.Client == nil {
		c.Client = publicClient()
	}
	switch c.Provider {
	case "anthropic":
		return &anthropic{c}, nil
	case "openai":
		return &openai{c, strings.TrimSuffix(or(c.OpenAIURL, "https://api.openai.com"), "/") + "/v1"}, nil
	case "local":
		if !strings.HasPrefix(c.Endpoint, "https://") {
			return nil, &ErrProvider{"the model's endpoint must be an https address (Settings → AI)"}
		}
		return &openai{c, strings.TrimSuffix(c.Endpoint, "/")}, nil
	}
	return nil, &ErrProvider{"no AI model is set for this tenant (Settings → AI)"}
}

// post sends body as JSON and decodes the answer into out.
func post(ctx context.Context, c *http.Client, url string, headers map[string]string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &ErrProvider{"the AI provider took too long to answer: try again"}
		}
		return &ErrProvider{"the AI provider could not be reached: try again later"}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return &ErrProvider{explain(resp.StatusCode, raw)}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &ErrProvider{"unexpected answer from the AI provider"}
	}
	return nil
}

// explain says what kind of refusal it is, as the agent does.
func explain(status int, body []byte) string {
	text := strings.ToLower(string(body[:min(len(body), 4096)]))
	switch {
	case status == 401 || status == 403:
		return "the AI provider refused the API key: check it in Settings → AI"
	case status == 402 || strings.Contains(text, "credit balance") || strings.Contains(text, "insufficient_quota") || strings.Contains(text, "billing"):
		return "no credit left on the AI account: top it up on the provider's billing page"
	case status == 404 || strings.Contains(text, "model_not_found"):
		return "the AI model does not exist for this key: choose it again in Settings → AI"
	case status == 429:
		return "the AI provider limits the requests: try again in a minute"
	case status >= 500:
		return "the AI provider is not answering right now: try again later"
	case status >= 300 && status < 400:
		return fmt.Sprintf("the AI provider answered %d (redirects are not followed)", status)
	}
	return fmt.Sprintf("the AI provider refused the request (%d)", status)
}

// publicClient connects only to public addresses and follows no redirect:
// a tenant's endpoint must not become a way into the platform's network.
func publicClient() *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if _, ok := exposure.PublicIP(host); !ok {
			return fmt.Errorf("refused to connect to %s: not a public address", host)
		}
		return nil
	}}
	return &http.Client{Timeout: 90 * time.Second, Transport: &http.Transport{DialContext: d.DialContext,
		TLSHandshakeTimeout: 5 * time.Second, ForceAttemptHTTP2: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
