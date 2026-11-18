// Package legacyclient builds requests to the legacy application from
// scratch and reads its responses. No client data is copied: method, path,
// query and headers come from the map and from already validated values.
package legacyclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
)

// MaxBody is the maximum size of a legacy response (16 MiB).
const MaxBody = 16 << 20

// ErrTooLarge reports a response larger than MaxBody.
var ErrTooLarge = errors.New("legacy response too large")

// Legacy response headers that may reach the client.
// Everything else (Server, X-Powered-By, Set-Cookie, ...) is dropped.
var responseHeaders = []string{
	"Content-Type", "Content-Language", "Content-Disposition",
	"Cache-Control", "Expires", "Last-Modified",
}

// Response is the already filtered legacy response.
type Response struct {
	Status   int
	Header   http.Header
	Body     []byte
	Location string // redirect target, if any (legacy URL)
	Duration time.Duration
}

// Client talks to a legacy application through a RoundTripper (direct or tunnel).
type Client struct {
	base      *url.URL
	rt        http.RoundTripper
	userAgent string
	timeout   time.Duration
}

// New creates a client for the legacy application at baseURL.
func New(baseURL string, rt http.RoundTripper, userAgent string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid base_url: %q", baseURL)
	}
	if userAgent == "" {
		userAgent = "equalizer-gateway"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{base: u, rt: rt, userAgent: userAgent, timeout: timeout}, nil
}

// Build builds the legacy request from legacy_request and the values.
func (c *Client) Build(ctx context.Context, lr mapmodel.LegacyRequest, vals placeholder.Values) (*http.Request, error) {
	path, rawPath, err := resolvePath(lr.Path, vals)
	if err != nil {
		return nil, err
	}
	u := *c.base
	// RawPath keeps the values encoded: a "/" inside a value never becomes
	// a path separator.
	u.Path = strings.TrimSuffix(c.base.Path, "/") + path
	u.RawPath = strings.TrimSuffix(c.base.EscapedPath(), "/") + rawPath
	u.RawQuery = ""
	u.Fragment = ""

	q := url.Values{}
	for _, key := range sortedKeys(lr.Query) {
		v, absent, err := placeholder.Resolve(lr.Query[key], vals)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", key, err)
		}
		if !absent {
			q.Set(key, v)
		}
	}
	u.RawQuery = q.Encode()

	method := strings.ToUpper(lr.Method)
	var body io.Reader
	contentType := ""
	switch method {
	case http.MethodGet:
		if len(lr.Form) > 0 {
			return nil, errors.New("a GET request cannot carry a form body")
		}
	case http.MethodPost:
		form := url.Values{}
		for _, key := range sortedKeys(lr.Form) {
			v, absent, err := placeholder.Resolve(lr.Form[key], vals)
			if err != nil {
				return nil, fmt.Errorf("form %q: %w", key, err)
			}
			if !absent {
				form.Set(key, v)
			}
		}
		// Values arrive already encoded in the legacy charset: Encode
		// percent-encodes their bytes as they are.
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		return nil, fmt.Errorf("method %q towards the legacy application is not supported yet", method)
	}
	if len(lr.Files) > 0 {
		return nil, errors.New("file uploads are not supported yet")
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = http.Header{}
	for _, key := range sortedKeys(lr.Headers) {
		v, absent, err := placeholder.Resolve(lr.Headers[key], vals)
		if err != nil {
			return nil, fmt.Errorf("header %q: %w", key, err)
		}
		if !absent {
			req.Header.Set(key, v)
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	return req, nil
}

// Do sends the request and reads the response. Redirects are not followed.
func (c *Client) Do(req *http.Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), c.timeout)
	defer cancel()
	start := time.Now()
	resp, err := c.rt.RoundTrip(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if len(body) > MaxBody {
		return nil, ErrTooLarge
	}
	out := &Response{Status: resp.StatusCode, Header: http.Header{}, Body: body, Duration: time.Since(start)}
	for _, h := range responseHeaders {
		if v := resp.Header.Get(h); v != "" {
			out.Header.Set(h, v)
		}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc, err := resp.Location(); err == nil {
			out.Location = loc.String()
		}
	}
	return out, nil
}

// Base returns the legacy base URL (to recognise internal redirects).
func (c *Client) Base() *url.URL { u := *c.base; return &u }

// resolvePath returns the decoded and the encoded path.
func resolvePath(tpl string, vals placeholder.Values) (string, string, error) {
	escaped := placeholder.Values{}
	for ns, m := range vals {
		escaped[ns] = make(map[string]string, len(m))
		for k, v := range m {
			escaped[ns][k] = url.PathEscape(v)
		}
	}
	raw, absent, err := placeholder.Resolve(tpl, escaped)
	if err != nil {
		return "", "", fmt.Errorf("path: %w", err)
	}
	if absent || !strings.HasPrefix(raw, "/") {
		return "", "", fmt.Errorf("invalid path: %q", tpl)
	}
	plain, _, err := placeholder.Resolve(tpl, vals)
	if err != nil {
		return "", "", fmt.Errorf("path: %w", err)
	}
	return plain, raw, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
