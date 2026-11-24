// Package panel is the source reader's side of the panel's source API
// (schema/api/panel-api.yaml, tag source): it says it runs, takes reads and
// sends their summaries. The reader only ever calls out.
package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client talks to the panel with the site's token.
type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

// Job is a read to do (SourceJob).
type Job struct {
	ID       int64   `json:"id"`
	Method   string  `json:"method"`
	Host     *string `json:"host"`
	Port     *int    `json:"port"`
	Username *string `json:"username"`
	Path     string  `json:"path"`
	Share    *string `json:"share"`
	Password *string `json:"password"`
}

// ErrUnauthorized: the token is wrong or was replaced.
var ErrUnauthorized = errors.New("the panel refused the token: take a new one from the site's Source tab")

// Hello says the reader runs, with its version and ssh public key.
func (c *Client) Hello(ctx context.Context, version, publicKey string) error {
	return c.post(ctx, "/source/hello", map[string]string{"version": version, "public_key": publicKey}, nil)
}

// Claim takes the next read, nil when there is none.
func (c *Client) Claim(ctx context.Context) (*Job, error) {
	var j Job
	err := c.post(ctx, "/source/claim", nil, &j)
	if errors.Is(err, errEmpty) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// Result sends a read's summary, or its error.
func (c *Client) Result(ctx context.Context, id int64, summary any, failure string) error {
	body := map[string]any{}
	if failure != "" {
		body["error"] = failure
	} else {
		body["summary"] = summary
	}
	return c.post(ctx, "/source/reads/"+strconv.FormatInt(id, 10)+"/result", body, nil)
}

var errEmpty = errors.New("nothing to do")

func (c *Client) post(ctx context.Context, path string, in, out any) error {
	var body io.Reader = http.NoBody
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusNoContent && out != nil:
		return errEmpty
	case resp.StatusCode >= 300:
		return fmt.Errorf("the panel answered %d: %.200s", resp.StatusCode, data)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
