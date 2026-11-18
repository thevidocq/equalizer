// Package session keeps gateway sessions: the role granted at login and the
// legacy application's cookies, which never reach the browser.
//
// The browser holds only a random id (cookie __Host-eq_session). The store
// key is the SHA-256 of that id, and the content is encrypted with
// AES-256-GCM: neither a copy of the store nor its logs reveal sessions or
// legacy cookies.
package session

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CookieName is the browser cookie carrying the session id.
const CookieName = "__Host-eq_session"

// ErrNotFound reports a missing, expired or unreadable session.
var ErrNotFound = errors.New("session not found")

// Store keeps encrypted session blobs with a time to live.
type Store interface {
	Load(ctx context.Context, key string) ([]byte, error) // ErrNotFound if absent
	Save(ctx context.Context, key string, blob []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Session is what the gateway remembers about one browser.
type Session struct {
	Tenant   string            `json:"t"`
	App      string            `json:"a"`
	Role     string            `json:"r,omitempty"` // empty: the map's anonymous role
	Cookies  map[string]string `json:"c,omitempty"` // legacy cookies by name
	Created  time.Time         `json:"cr"`
	LastSeen time.Time         `json:"ls"`
}

// Policy limits a session's life.
type Policy struct {
	Idle     time.Duration
	Absolute time.Duration
}

// DefaultPolicy applies when the map says nothing.
var DefaultPolicy = Policy{Idle: 30 * time.Minute, Absolute: 12 * time.Hour}

// Manager loads and saves sessions. It is safe for concurrent use.
type Manager struct {
	store Store
	aead  cipher.AEAD
	now   func() time.Time
}

// NewManager creates a manager. key is the gateway's 32-byte secret: the
// session key is derived from it, so it never equals the form-token key.
func NewManager(store Store, key []byte) (*Manager, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("session key must be 32 bytes, got %d", len(key))
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("equalizer-session-v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store, aead: aead, now: time.Now}, nil
}

// NewID returns a new random session id.
func NewID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ValidID reports whether a cookie value has the shape of a session id.
func ValidID(id string) bool {
	b, err := base64.RawURLEncoding.DecodeString(id)
	return err == nil && len(b) == 32
}

// Hash is the session's identifier in logs and events (never the id itself).
func Hash(id string) string {
	sum := sha256.Sum256([]byte("log:" + id))
	return hex.EncodeToString(sum[:8])
}

func key(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "eq:sess:" + hex.EncodeToString(sum[:])
}

// Load returns the session for id. Expired sessions (absolute limit) and
// sessions of another tenant or app are ErrNotFound.
func (m *Manager) Load(ctx context.Context, id, tenant, app string, p Policy) (*Session, error) {
	if !ValidID(id) {
		return nil, ErrNotFound
	}
	blob, err := m.store.Load(ctx, key(id))
	if err != nil {
		return nil, err
	}
	n := m.aead.NonceSize()
	if len(blob) < n {
		return nil, ErrNotFound
	}
	plain, err := m.aead.Open(nil, blob[:n], blob[n:], []byte(id))
	if err != nil {
		return nil, ErrNotFound
	}
	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, ErrNotFound
	}
	if s.Tenant != tenant || s.App != app || m.now().Sub(s.Created) > p.Absolute {
		return nil, ErrNotFound
	}
	return &s, nil
}

// Save stores the session for the idle time, never past the absolute limit.
func (m *Manager) Save(ctx context.Context, id string, s *Session, p Policy) error {
	now := m.now()
	if s.Created.IsZero() {
		s.Created = now
	}
	s.LastSeen = now
	ttl := min(p.Idle, p.Absolute-now.Sub(s.Created))
	if ttl <= 0 {
		return m.Delete(ctx, id)
	}
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	// The id is the additional data: a blob copied under another key does not open.
	return m.store.Save(ctx, key(id), m.aead.Seal(nonce, nonce, plain, []byte(id)), ttl)
}

// Delete removes a session.
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, key(id))
}
