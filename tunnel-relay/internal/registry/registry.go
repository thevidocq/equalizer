// Package registry holds the connectors allowed to open a tunnel, read from
// connectors.json. Only the SHA-256 of each token is stored.
package registry

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"
	"time"
)

// Connector is a registered connector.
type Connector struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant"`
	TokenSHA256 string `json:"token_sha256"`
}

type file struct {
	Connectors []Connector `json:"connectors"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}$`)

// ValidID reports whether id is a well-formed connector id.
func ValidID(id string) bool { return idRe.MatchString(id) }

// Registry is the active set of connectors, swapped atomically on reload.
// It reads connectors.json, or panel-api when a URL is set.
type Registry struct {
	path    string
	url     string
	token   string
	client  *http.Client
	current atomic.Pointer[map[string]Connector]
	modTime atomic.Int64
	digest  atomic.Value // string: SHA-256 of the last document from the panel
}

// New creates a registry for the file at path. Call Reload to load it.
func New(path string) *Registry {
	r := &Registry{path: path}
	empty := map[string]Connector{}
	r.current.Store(&empty)
	r.digest.Store("")
	return r
}

// NewFromPanel creates a registry that reads GET /v1/internal/connectors.
func NewFromPanel(url, token string) *Registry {
	r := New("")
	r.url, r.token, r.client = url, token, &http.Client{Timeout: 10 * time.Second}
	return r
}

// Source describes where the connectors come from.
func (r *Registry) Source() string {
	if r.url != "" {
		return "panel " + r.url
	}
	return "file " + r.path
}

// Reload reads the connectors if they changed. It returns true when the set
// changed. On error the previous set stays active.
func (r *Registry) Reload() (bool, error) {
	if r.url != "" {
		return r.reloadPanel()
	}
	info, err := os.Stat(r.path)
	if err != nil {
		return false, fmt.Errorf("reading connectors: %w", err)
	}
	if info.ModTime().UnixNano() == r.modTime.Load() {
		return false, nil
	}
	raw, err := os.ReadFile(r.path)
	if err != nil {
		return false, fmt.Errorf("reading connectors: %w", err)
	}
	set, err := Parse(raw)
	if err != nil {
		return false, err
	}
	r.current.Store(&set)
	r.modTime.Store(info.ModTime().UnixNano())
	return true, nil
}

func (r *Registry) reloadPanel() (bool, error) {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := r.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("panel unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("panel answered %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if digest == r.digest.Load().(string) {
		return false, nil
	}
	set, err := Parse(raw)
	if err != nil {
		return false, err
	}
	r.current.Store(&set)
	r.digest.Store(digest)
	return true, nil
}

// Parse validates the connectors document.
func Parse(raw []byte) (map[string]Connector, error) {
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("connectors file is not valid JSON: %w", err)
	}
	set := make(map[string]Connector, len(f.Connectors))
	for _, c := range f.Connectors {
		if !ValidID(c.ID) {
			return nil, fmt.Errorf("invalid connector id %q", c.ID)
		}
		if _, dup := set[c.ID]; dup {
			return nil, fmt.Errorf("duplicate connector id %q", c.ID)
		}
		if b, err := hex.DecodeString(c.TokenSHA256); err != nil || len(b) != sha256.Size {
			return nil, fmt.Errorf("connector %q: token_sha256 must be 64 hex digits", c.ID)
		}
		set[c.ID] = c
	}
	return set, nil
}

// Authenticate returns the connector if id and token match.
func (r *Registry) Authenticate(id, token string) (Connector, bool) {
	c, ok := (*r.current.Load())[id]
	if !ok || token == "" {
		// Compare anyway, so timing does not reveal which ids exist.
		subtle.ConstantTimeCompare([]byte(Hash(token)), []byte(Hash("")))
		return Connector{}, false
	}
	if subtle.ConstantTimeCompare([]byte(Hash(token)), []byte(c.TokenSHA256)) != 1 {
		return Connector{}, false
	}
	return c, true
}

// Get returns a registered connector.
func (r *Registry) Get(id string) (Connector, bool) {
	c, ok := (*r.current.Load())[id]
	return c, ok
}

// Hash returns the hex SHA-256 of a token.
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
