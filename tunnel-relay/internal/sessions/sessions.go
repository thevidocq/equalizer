// Package sessions keeps the open tunnels of every connector and picks one
// for each request.
package sessions

import (
	"errors"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
)

// MaxPerConnector limits the sessions one connector id may keep open.
const MaxPerConnector = 8

// Errors returned by Pick and Add.
var (
	ErrUnavailable    = errors.New("connector_unavailable")
	ErrNotDeclared    = errors.New("destination_not_declared")
	ErrTooManyTunnels = errors.New("too_many_tunnels")
)

// Session is one open tunnel.
type Session struct {
	ConnectorID string
	Tenant      string // owner: a map of another tenant can never use this tunnel
	Version     string
	Allow       []string // destinations declared by the connector
	Since       time.Time
	RemoteAddr  string
	TokenHash   string // to close the tunnel if the token is revoked
	mux         *yamux.Session
}

// Open opens a new stream on the tunnel.
func (s *Session) Open() (net.Conn, error) { return s.mux.Open() }

// Closed is closed when the tunnel ends.
func (s *Session) Closed() <-chan struct{} { return s.mux.CloseChan() }

// Close ends the tunnel.
func (s *Session) Close() error { return s.mux.Close() }

// Pool holds the sessions by connector id. It is safe for concurrent use.
type Pool struct {
	mu   sync.RWMutex
	byID map[string][]*Session
	next atomic.Uint64
}

// NewPool creates an empty pool.
func NewPool() *Pool { return &Pool{byID: map[string][]*Session{}} }

// Add registers a session and returns it. The session is removed from the
// pool when its tunnel closes.
func (p *Pool) Add(s *Session, mux *yamux.Session) (*Session, error) {
	s.mux, s.Since = mux, time.Now()
	id := s.ConnectorID
	p.mu.Lock()
	if len(p.byID[id]) >= MaxPerConnector {
		p.mu.Unlock()
		return nil, ErrTooManyTunnels
	}
	p.byID[id] = append(p.byID[id], s)
	p.mu.Unlock()
	return s, nil
}

// Remove drops a session. It returns how many sessions the connector still has.
func (p *Pool) Remove(s *Session) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := slices.DeleteFunc(p.byID[s.ConnectorID], func(x *Session) bool { return x == s })
	if len(list) == 0 {
		delete(p.byID, s.ConnectorID)
	} else {
		p.byID[s.ConnectorID] = list
	}
	return len(list)
}

// Pick chooses, round robin, an open session of the connector that declared
// the destination.
func (p *Pool) Pick(id, destination string) (*Session, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	list := p.byID[id]
	if len(list) == 0 {
		return nil, ErrUnavailable
	}
	var eligible []*Session
	for _, s := range list {
		if !s.mux.IsClosed() && slices.Contains(s.Allow, destination) {
			eligible = append(eligible, s)
		}
	}
	if len(eligible) == 0 {
		return nil, ErrNotDeclared
	}
	return eligible[p.next.Add(1)%uint64(len(eligible))], nil
}

// CloseExcept closes the sessions not accepted by keep (for instance after
// a connector was removed from the registry or its token changed).
func (p *Pool) CloseExcept(keep func(s *Session) bool) int {
	p.mu.RLock()
	var drop []*Session
	for _, list := range p.byID {
		for _, s := range list {
			if !keep(s) {
				drop = append(drop, s)
			}
		}
	}
	p.mu.RUnlock()
	for _, s := range drop {
		_ = s.Close()
	}
	return len(drop)
}

// Status describes the open sessions, for the admin endpoint.
type Status struct {
	ConnectorID string    `json:"connector_id"`
	Version     string    `json:"version"`
	Allow       []string  `json:"allow"`
	Since       time.Time `json:"since"`
	RemoteAddr  string    `json:"remote_addr"`
}

// Snapshot lists the open sessions.
func (p *Pool) Snapshot() []Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []Status
	for _, list := range p.byID {
		for _, s := range list {
			out = append(out, Status{ConnectorID: s.ConnectorID, Version: s.Version, Allow: s.Allow, Since: s.Since, RemoteAddr: s.RemoteAddr})
		}
	}
	slices.SortFunc(out, func(a, b Status) int {
		if a.ConnectorID != b.ConnectorID {
			if a.ConnectorID < b.ConnectorID {
				return -1
			}
			return 1
		}
		return a.Since.Compare(b.Since)
	})
	return out
}
