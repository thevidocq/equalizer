package legacyclient

import (
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
)

// ErrEncoding reports a legacy answer compressed in a way the gateway cannot
// read (it always asks for identity; some applications compress anyway).
var ErrEncoding = errors.New("unsupported content encoding from the legacy application")

// decode returns the body without its content encoding. The legacy
// application is asked for identity; gzip and deflate are undone when it
// compresses anyway, so pages can be rendered and files keep their bytes.
func decode(resp *http.Response) (io.Reader, bool, error) {
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "", "identity":
		return resp.Body, false, nil
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			if errors.Is(err, io.EOF) { // empty body (HEAD, 204, 304)
				return resp.Body, true, nil
			}
			return nil, false, ErrEncoding
		}
		return zr, true, nil
	case "deflate":
		return flate.NewReader(resp.Body), true, nil
	}
	return nil, false, ErrEncoding
}

// readCloser reads the decoded body and closes the connection's.
type readCloser struct {
	io.Reader
	io.Closer
}
