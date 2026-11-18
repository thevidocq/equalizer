package mapstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// ErrUnchanged reports that the source has nothing new.
var ErrUnchanged = errors.New("map unchanged")

// Source provides the signed document. Fetch returns ErrUnchanged when the
// document did not change since the last successful fetch.
type Source interface {
	Fetch(ctx context.Context) ([]byte, error)
	String() string
}

// FileSource reads the document from a file (prototype without panel).
type FileSource struct {
	Path string
	mu   sync.Mutex
	last time.Time
}

// Fetch implements Source.
func (f *FileSource) Fetch(context.Context) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fi, err := os.Stat(f.Path)
	if err != nil {
		return nil, fmt.Errorf("reading map: %w", err)
	}
	if fi.ModTime().Equal(f.last) {
		return nil, ErrUnchanged
	}
	doc, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("reading map: %w", err)
	}
	f.last = fi.ModTime()
	return doc, nil
}

// Forget makes the next Fetch read the file again (SIGHUP).
func (f *FileSource) Forget() {
	f.mu.Lock()
	f.last = time.Time{}
	f.mu.Unlock()
}

func (f *FileSource) String() string { return "file " + f.Path }

// HTTPSource reads the published map from panel-api
// (GET /v1/internal/apps/{app}/map), with ETag so an unchanged map costs
// a 304.
type HTTPSource struct {
	URL    string
	Token  string
	Client *http.Client
	mu     sync.Mutex
	etag   string
}

// MaxDocument bounds the signed document size.
const MaxDocument = 8 << 20

// Fetch implements Source.
func (h *HTTPSource) Fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.Token)
	h.mu.Lock()
	if h.etag != "" {
		req.Header.Set("If-None-Match", h.etag)
	}
	h.mu.Unlock()
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panel unreachable: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, ErrUnchanged
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, errors.New("no published map for this app in the panel")
	default:
		return nil, fmt.Errorf("panel answered %d", resp.StatusCode)
	}
	doc, err := io.ReadAll(io.LimitReader(resp.Body, MaxDocument+1))
	if err != nil {
		return nil, err
	}
	if len(doc) > MaxDocument {
		return nil, errors.New("signed map too large")
	}
	h.mu.Lock()
	h.etag = resp.Header.Get("ETag")
	h.mu.Unlock()
	return doc, nil
}

// Forget makes the next Fetch download the map again.
func (h *HTTPSource) Forget() {
	h.mu.Lock()
	h.etag = ""
	h.mu.Unlock()
}

func (h *HTTPSource) String() string { return "panel " + h.URL }
