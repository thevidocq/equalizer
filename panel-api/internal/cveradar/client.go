// Package cveradar talks to the CVE Radar API, a vulnerability scanner a
// tenant connects with its own address and key. It creates targets, starts
// scans and reads their findings; nothing else.
package cveradar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
)

// Client is one tenant's connection.
type Client struct {
	Endpoint string       // https base URL of the API, without the final slash
	Key      string       // never logged
	HTTP     *http.Client // nil: a public-only client
}

// Error is an answer of the scanner that is not a success.
type Error struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration // with 429
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("the scanner answered %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("the scanner answered %d", e.Status)
}

// Target is the scanner's record of a domain.
type Target struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	RiskScore *int   `json:"risk_score"`
}

// Scan is one run of the scanner on a target.
type Scan struct {
	ID            string     `json:"id"`
	TargetID      string     `json:"target_id"`
	ScanType      string     `json:"scan_type"`
	Status        string     `json:"status"` // pending, running, completed, failed
	FindingsCount *int       `json:"findings_count"`
	Error         *string    `json:"error"`
	CompletedAt   *time.Time `json:"completed_at"`
}

// Vulnerability is a finding, normalised by the scanner.
type Vulnerability struct {
	ID          string     `json:"id"`
	TargetID    string     `json:"target_id"`
	Source      string     `json:"source"`
	Title       string     `json:"title"`
	URL         *string    `json:"url"`
	HTTPMethod  *string    `json:"http_method"`
	Parameter   *string    `json:"parameter"`
	Severity    string     `json:"severity"`
	CVSS        *float64   `json:"cvss"`
	CVE         []string   `json:"cve"`
	CWE         []string   `json:"cwe"`
	Evidence    *string    `json:"evidence"`
	Status      string     `json:"status"`
	FirstSeenAt *time.Time `json:"first_seen_at"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
	LastScanID  *string    `json:"last_scan_id"`
}

const (
	maxBody  = 4 << 20  // an answer larger than this is refused
	maxTotal = 32 << 20 // all the pages of one list together
	pageSize = 200      // the API's largest page
	maxPages = 50       // 10 000 findings or targets
)

// ErrTooMany is returned when a list goes past maxPages or maxTotal.
var ErrTooMany = errors.New("the scanner's list is longer than the panel reads (10 000 entries)")

// Check tries the address and key by listing one target.
func (c *Client) Check(ctx context.Context) error {
	var out struct {
		Data []Target `json:"data"`
	}
	_, err := c.do(ctx, http.MethodGet, "/targets?limit=1", nil, &out)
	return err
}

// FindTarget returns the target of a domain, or nil when there is none.
func (c *Client) FindTarget(ctx context.Context, domain string) (*Target, error) {
	cursor := ""
	for page := 0; page < maxPages; page++ {
		q := url.Values{"limit": {strconv.Itoa(pageSize)}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var out struct {
			Data       []Target `json:"data"`
			NextCursor *string  `json:"next_cursor"`
		}
		if _, err := c.do(ctx, http.MethodGet, "/targets?"+q.Encode(), nil, &out); err != nil {
			return nil, err
		}
		for i := range out.Data {
			if strings.EqualFold(out.Data[i].Domain, domain) {
				return &out.Data[i], nil
			}
		}
		if out.NextCursor == nil || *out.NextCursor == "" {
			return nil, nil
		}
		cursor = *out.NextCursor
	}
	return nil, ErrTooMany // never a second target for the same domain
}

// EnsureTarget returns the target of a domain, creating it when missing.
// Ownership is not verified (the scanner makes it optional): the panel asks
// its user to confirm instead.
func (c *Client) EnsureTarget(ctx context.Context, domain string) (Target, error) {
	if t, err := c.FindTarget(ctx, domain); err != nil || t != nil {
		if t == nil {
			return Target{}, err
		}
		return *t, nil
	}
	var out struct {
		Data Target `json:"data"`
	}
	_, err := c.do(ctx, http.MethodPost, "/targets", map[string]string{"domain": domain, "notes": "Equalizer"}, &out)
	return out.Data, err
}

// GetTarget reads a target (its risk score after a scan).
func (c *Client) GetTarget(ctx context.Context, id string) (Target, error) {
	var out struct {
		Data Target `json:"data"`
	}
	_, err := c.do(ctx, http.MethodGet, "/targets/"+url.PathEscape(id), nil, &out)
	return out.Data, err
}

// StartScan starts a scan of a target.
func (c *Client) StartScan(ctx context.Context, target, scanType string) (Scan, error) {
	var out struct {
		Data Scan `json:"data"`
	}
	_, err := c.do(ctx, http.MethodPost, "/targets/"+url.PathEscape(target)+"/scans",
		map[string]any{"scan_type": scanType, "require_verification": false}, &out)
	return out.Data, err
}

// GetScan reads a scan's status.
func (c *Client) GetScan(ctx context.Context, id string) (Scan, error) {
	var out struct {
		Data Scan `json:"data"`
	}
	_, err := c.do(ctx, http.MethodGet, "/scans/"+url.PathEscape(id), nil, &out)
	return out.Data, err
}

// OpenVulnerabilities lists the target's open findings, every page; past
// 10 000 findings or 32 MiB it returns those read and ErrTooMany.
func (c *Client) OpenVulnerabilities(ctx context.Context, target string) ([]Vulnerability, error) {
	var all []Vulnerability
	cursor, total := "", 0
	for page := 0; ; page++ {
		if page == maxPages || total > maxTotal {
			return all, ErrTooMany
		}
		q := url.Values{"target_id": {target}, "status": {"open"}, "limit": {strconv.Itoa(pageSize)}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var out struct {
			Data       []Vulnerability `json:"data"`
			NextCursor *string         `json:"next_cursor"`
		}
		n, err := c.do(ctx, http.MethodGet, "/vulnerabilities?"+q.Encode(), nil, &out)
		if err != nil {
			return nil, err
		}
		total += n
		all = append(all, out.Data...)
		if out.NextCursor == nil || *out.NextCursor == "" {
			return all, nil
		}
		cursor = *out.NextCursor
	}
}

// do sends one request and decodes the answer into out; it returns the
// answer's size.
func (c *Client) do(ctx context.Context, method, path string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return 0, fmt.Errorf("the scanner could not be reached: %w", scrub(err))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return 0, fmt.Errorf("the scanner's answer was cut: %w", err)
	}
	if len(data) > maxBody {
		return 0, errors.New("the scanner's answer is too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		e := &Error{Status: resp.StatusCode}
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &env) == nil {
			e.Code, e.Message = clip(env.Error.Code, 60), clip(env.Error.Message, 300)
		}
		e.RetryAfter = retryAfter(resp.Header.Get("Retry-After"))
		return len(data), e
	}
	if out == nil {
		return len(data), nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return len(data), fmt.Errorf("the scanner's answer is not the expected JSON: %w", err)
	}
	return len(data), nil
}

// retryAfter reads Retry-After in seconds or as an HTTP date.
func retryAfter(v string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && time.Until(t) > 0 {
		return time.Until(t).Round(time.Second)
	}
	return 0
}

// scrub drops the URL from a transport error (it carries no key, but the
// message is shown to the user: the reason is enough).
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return public
}

// public is the one client of every tenant's scanner: public addresses
// only, no redirects, no proxy, idle connections closed.
var public = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{
	DialContext:         (&net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}).DialContext,
	TLSHandshakeTimeout: 5 * time.Second, Proxy: nil,
	MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 20 * time.Second,
}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// publicOnly refuses to connect to a private, loopback or reserved address:
// a tenant's scanner address must not become a way into the platform.
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

// ValidEndpoint accepts https URLs without credentials, query or fragment.
func ValidEndpoint(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" &&
		u.Fragment == "" && len(raw) <= 300
}

// OnHTTPSPort reports whether a valid endpoint uses port 443: the panel
// connects to no other port of a tenant's address.
func OnHTTPSPort(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (u.Port() == "" || u.Port() == "443")
}
