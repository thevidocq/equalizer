// Package legacyclient costruisce da zero le richieste verso il legacy e ne
// legge le risposte. Nessun dato del client viene copiato: metodo, percorso,
// query e header nascono dalla mappa e dai valori già validati.
package legacyclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
)

// MaxBody è la dimensione massima di una risposta del legacy (16 MiB).
const MaxBody = 16 << 20

// ErrTooLarge indica una risposta oltre MaxBody.
var ErrTooLarge = errors.New("risposta del legacy troppo grande")

// Header della risposta del legacy che possono arrivare al client.
// Tutto il resto (Server, X-Powered-By, Set-Cookie, ...) viene scartato.
var responseHeaders = []string{
	"Content-Type", "Content-Language", "Content-Disposition",
	"Cache-Control", "Expires", "Last-Modified",
}

// Response è la risposta del legacy già filtrata.
type Response struct {
	Status   int
	Header   http.Header
	Body     []byte
	Location string // destinazione del redirect, se presente (URL del legacy)
	Duration time.Duration
}

// Client parla con un legacy attraverso un RoundTripper (diretto o tunnel).
type Client struct {
	base      *url.URL
	rt        http.RoundTripper
	userAgent string
	timeout   time.Duration
}

// New crea un client per il legacy in baseURL.
func New(baseURL string, rt http.RoundTripper, userAgent string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base_url non valido: %q", baseURL)
	}
	if userAgent == "" {
		userAgent = "equalizer-gateway"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{base: u, rt: rt, userAgent: userAgent, timeout: timeout}, nil
}

// Build costruisce la richiesta verso il legacy da legacy_request e dai valori.
func (c *Client) Build(ctx context.Context, lr mapmodel.LegacyRequest, vals placeholder.Values) (*http.Request, error) {
	path, rawPath, err := resolvePath(lr.Path, vals)
	if err != nil {
		return nil, err
	}
	u := *c.base
	// RawPath tiene i valori codificati: un "/" in un valore non diventa
	// mai un separatore di percorso.
	u.Path = strings.TrimSuffix(c.base.Path, "/") + path
	u.RawPath = strings.TrimSuffix(c.base.EscapedPath(), "/") + rawPath
	u.RawQuery = ""
	u.Fragment = ""

	q := url.Values{}
	for _, key := range sortedKeys(lr.Query) {
		v, absent, err := placeholder.Resolve(lr.Query[key], vals)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", key, err)
		}
		if !absent {
			q.Set(key, v)
		}
	}
	u.RawQuery = q.Encode()

	method := strings.ToUpper(lr.Method)
	if method != http.MethodGet {
		return nil, fmt.Errorf("metodo %q non ancora supportato verso il legacy", method)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = http.Header{}
	for _, key := range sortedKeys(lr.Headers) {
		v, absent, err := placeholder.Resolve(lr.Headers[key], vals)
		if err != nil {
			return nil, fmt.Errorf("header %q: %w", key, err)
		}
		if !absent {
			req.Header.Set(key, v)
		}
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	return req, nil
}

// Do invia la richiesta e legge la risposta. I redirect non vengono seguiti.
func (c *Client) Do(req *http.Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), c.timeout)
	defer cancel()
	start := time.Now()
	resp, err := c.rt.RoundTrip(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("lettura risposta: %w", err)
	}
	if len(body) > MaxBody {
		return nil, ErrTooLarge
	}
	out := &Response{Status: resp.StatusCode, Header: http.Header{}, Body: body, Duration: time.Since(start)}
	for _, h := range responseHeaders {
		if v := resp.Header.Get(h); v != "" {
			out.Header.Set(h, v)
		}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc, err := resp.Location(); err == nil {
			out.Location = loc.String()
		}
	}
	return out, nil
}

// Base restituisce l'URL base del legacy (per riconoscere i redirect interni).
func (c *Client) Base() *url.URL { u := *c.base; return &u }

// resolvePath restituisce il percorso decodificato e quello codificato.
func resolvePath(tpl string, vals placeholder.Values) (string, string, error) {
	escaped := placeholder.Values{}
	for ns, m := range vals {
		escaped[ns] = make(map[string]string, len(m))
		for k, v := range m {
			escaped[ns][k] = url.PathEscape(v)
		}
	}
	raw, absent, err := placeholder.Resolve(tpl, escaped)
	if err != nil {
		return "", "", fmt.Errorf("path: %w", err)
	}
	if absent || !strings.HasPrefix(raw, "/") {
		return "", "", fmt.Errorf("path non valido: %q", tpl)
	}
	plain, _, err := placeholder.Resolve(tpl, vals)
	if err != nil {
		return "", "", fmt.Errorf("path: %w", err)
	}
	return plain, raw, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
