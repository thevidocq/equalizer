package server

import (
	"crypto/sha256"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// noLimits: tries measure the map, never the rate limits.
var noLimits limits.Store = limits.None{}

// How many compiled try maps are kept (a verification uses one or two).
const trySitesKept = 16

// trySites keeps compiled try maps by payload and mode: a verification
// sends thousands of tries with the same draft.
type trySites struct {
	mu    sync.Mutex
	sites map[[32]byte]*site.Site
	order [][32]byte
}

func (c *trySites) get(payload []byte, mode string, build func() (*site.Site, error)) (*site.Site, error) {
	k := sha256.Sum256(append([]byte(mode+"\n"), payload...))
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.sites[k]; ok {
		return s, nil
	}
	s, err := build()
	if err != nil {
		return nil, err
	}
	if c.sites == nil {
		c.sites = map[[32]byte]*site.Site{}
	}
	if len(c.order) >= trySitesKept {
		delete(c.sites, c.order[0])
		c.order = c.order[1:]
	}
	c.sites[k] = s
	c.order = append(c.order, k)
	return s, nil
}

// singleSite serves the try's draft on its own domain.
type singleSite struct{ s *site.Site }

func (x singleSite) For(string) *site.Site { return x.s }
func (singleSite) Ready() bool             { return true }

func isArea(st *site.Site, id string) bool {
	for _, a := range st.Model.Areas {
		if a.ID == id {
			return true
		}
	}
	return false
}

// tryCapture records whether the gateway would have reached the legacy
// site: the dry-run transport finds it in the request's context.
type tryCapture struct{ forwarded atomic.Bool }

type tryCaptureKey struct{}

// dryTransport stands in for the legacy site in dry-run: it reads the
// rebuilt request to the end (so streamed checks, such as uploads, run) and
// answers an empty 204. Nothing leaves the gateway.
type dryTransport struct{}

func (dryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, err := io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}
	if c, ok := req.Context().Value(tryCaptureKey{}).(*tryCapture); ok {
		c.forwarded.Store(true)
	}
	return &http.Response{StatusCode: http.StatusNoContent, Status: "204 No Content", Proto: "HTTP/1.1", ProtoMajor: 1,
		ProtoMinor: 1, Header: http.Header{}, Body: http.NoBody, Request: req}, nil
}
