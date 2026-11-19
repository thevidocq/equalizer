package legacyclient

import (
	"context"
	"io"
	"net/http"

	"strings"
	"time"
)

// AreaTimeout bounds a request served by an area, body included: large
// downloads are streamed, not buffered.
const AreaTimeout = 10 * time.Minute

// Stream is a legacy response whose body is still to be read.
type Stream struct {
	Status   int
	Header   http.Header // filtered, like Response
	Location string
	Cookies  []*http.Cookie
	Body     io.ReadCloser
	Start    time.Time
	cancel   context.CancelFunc
}

// Close releases the response.
func (s *Stream) Close() {
	_ = s.Body.Close()
	s.cancel()
}

// ReadAll reads a body of at most MaxBody (pages to render).
func (s *Stream) ReadAll() ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(s.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBody {
		return nil, ErrTooLarge
	}
	return body, nil
}

// BuildRaw builds a request for an area: the same path and the rebuilt
// query and body, under the legacy base URL.
// size is the body's length (body may be nil when it is 0).
func (c *Client) BuildRaw(ctx context.Context, method, path, rawQuery string, body io.Reader, size int64, contentType string, h http.Header) (*http.Request, error) {
	u := *c.base
	u.Path = strings.TrimSuffix(c.base.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = rawQuery
	u.Fragment = ""
	if size == 0 {
		body = nil
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	req.Header = http.Header{}
	for k, vs := range h {
		req.Header[k] = vs
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Encoding", "identity")
	return req, nil
}

// Open sends the request and returns the response without reading its body.
func (c *Client) Open(req *http.Request) (*Stream, error) {
	ctx, cancel := context.WithTimeout(req.Context(), AreaTimeout)
	start := time.Now()
	resp, err := c.rt.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	body, decoded, err := decode(resp)
	if err != nil {
		resp.Body.Close()
		cancel()
		return nil, err
	}
	s := &Stream{Status: resp.StatusCode, Header: http.Header{}, Cookies: resp.Cookies(), Body: readCloser{body, resp.Body}, Start: start, cancel: cancel}
	for _, h := range areaResponseHeaders {
		for _, v := range resp.Header.Values(h) {
			s.Header.Add(h, v)
		}
	}
	if decoded {
		s.Header.Del("Content-Length")
		s.Header.Del("Content-Range")
		s.Header.Del("Accept-Ranges")
	}
	// The application's own headers (X-WP-Total, X-CSRF-Token, …): never
	// those that name the legacy software or its internal address.
	for k, vs := range resp.Header {
		if !appHeader(k) {
			continue
		}
		for _, v := range vs {
			if !strings.Contains(strings.ToLower(v), strings.ToLower(c.base.Host)) {
				s.Header.Add(k, v)
			}
		}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		s.Location = location(resp)
	}
	return s, nil
}

// areaResponseHeaders are the legacy headers an area passes on: those of
// pages plus what downloads and scripts of the site rely on.
var areaResponseHeaders = append(append([]string{}, responseHeaders...),
	"Content-Length", "ETag", "Accept-Ranges", "Content-Range", "Vary", "Pragma",
	"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers",
	"Access-Control-Allow-Credentials", "Access-Control-Expose-Headers", "Access-Control-Max-Age", "Allow",
	"Link", "Retry-After")

// revealing headers name the legacy software or its version.
var revealing = map[string]bool{
	"X-Powered-By": true, "X-Aspnet-Version": true, "X-Aspnetmvc-Version": true, "X-Generator": true,
	"X-Drupal-Cache": true, "X-Drupal-Dynamic-Cache": true, "X-Runtime": true, "X-Debug-Token": true,
	"X-Debug-Token-Link": true, "X-Sourcefiles": true, "X-Backend-Server": true, "X-Server": true,
}

// appHeader reports whether a response header is the application's own
// (X-…) and may reach the browser.
func appHeader(k string) bool {
	k = http.CanonicalHeaderKey(k)
	if !strings.HasPrefix(k, "X-") || revealing[k] || k == "X-Frame-Options" || k == "X-Content-Type-Options" {
		return false
	}
	for _, p := range internalPrefixes {
		if strings.HasPrefix(k, p) {
			return false
		}
	}
	return true
}

// internalPrefixes are response headers meant for a front proxy or for
// debugging: internal paths (X-Accel-Redirect, X-Sendfile), servers, traces.
var internalPrefixes = []string{
	"X-Eq-", "X-Forwarded", "X-Accel-", "X-Sendfile", "X-Lighttpd-", "X-Litespeed-", "X-Debug", "X-Backend",
	"X-Server", "X-Upstream", "X-Varnish", "X-Php", "X-Aspnet", "X-Sourcemap", "X-Chromelogger", "X-Wf-",
}
