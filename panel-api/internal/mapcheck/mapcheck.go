// Package mapcheck asks a gateway whether a map would load: the gateway's
// own checks (roles, types, authorisation, issues, login), so the panel
// never approves or publishes a version the gateways would refuse.
package mapcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnavailable means no gateway answered: the map was not checked.
var ErrUnavailable = errors.New("no gateway answered the map check")

// Inconsistent is the gateway's reason to refuse a map.
type Inconsistent struct{ Reason string }

func (e *Inconsistent) Error() string { return "the gateway would refuse this map: " + e.Reason }

// Checker calls POST /maps/check on the gateways' admin endpoint.
type Checker struct {
	Addr   string // host:port, e.g. gateway:9090
	Client *http.Client
}

// Check returns nil, *Inconsistent or ErrUnavailable.
func (c *Checker) Check(ctx context.Context, doc json.RawMessage) error {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+c.Addr+"/maps/check", bytes.NewReader(doc))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return fmt.Errorf("%w: unreadable answer (%d)", ErrUnavailable, resp.StatusCode)
	}
	switch {
	case resp.StatusCode == http.StatusOK && out.OK:
		return nil
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return &Inconsistent{Reason: out.Error}
	default:
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
}
