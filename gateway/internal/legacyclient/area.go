package legacyclient

import (
	"bytes"
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
func (c *Client) BuildRaw(ctx context.Context, method, path, rawQuery string, body []byte, contentType string, h http.Header) (*http.Request, error) {
	u := *c.base
	u.Path = strings.TrimSuffix(c.base.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = rawQuery
	u.Fragment = ""
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.Header = http.Header{}
	for k, vs := range h {
		req.Header[k] = vs
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)
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
	s := &Stream{Status: resp.StatusCode, Header: http.Header{}, Cookies: resp.Cookies(), Body: resp.Body, Start: start, cancel: cancel}
	for _, h := range areaResponseHeaders {
		for _, v := range resp.Header.Values(h) {
			s.Header.Add(h, v)
		}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc, err := resp.Location(); err == nil {
			s.Location = loc.String()
		}
	}
	return s, nil
}

// areaResponseHeaders are the legacy headers an area passes on: those of
// pages plus what downloads and scripts of the site rely on.
var areaResponseHeaders = append(append([]string{}, responseHeaders...),
	"Content-Length", "ETag", "Accept-Ranges", "Content-Range", "Vary", "Pragma",
	"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers",
	"Access-Control-Allow-Credentials", "Access-Control-Expose-Headers", "Access-Control-Max-Age", "Allow")
