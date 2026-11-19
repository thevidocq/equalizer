package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
)

// tryAddr is the visitor address of the agent's tries (TEST-NET-1): it never
// matches a real visitor's limits or sessions.
const tryAddr = "192.0.2.10:1"

// Try serves POST /maps/try (schema/api/gateway-admin.yaml): a request
// served with a draft map signed for the purpose, for the agent's checks.
// dry_run forwards nothing; preview serves GET and HEAD through the
// connector. Its events stay in the answer: an attack set tried by the
// agent never reaches the security logs.
type Try struct {
	Token    string // the panel's service token; empty: tries are off
	Pub      ed25519.PublicKey
	KeyID    string
	Relay    *transport.RelayConfig
	Sessions *session.Manager // try sessions (preview)
	Frozen   *session.Manager // the same store, read-only (dry-run)
	Forms    *formtoken.Sealer
	Log      *slog.Logger
	sites    trySites
	now      func() time.Time
}

type tryRequest struct {
	Map     mapstore.TryMap `json:"map"`
	Mode    string          `json:"mode"`
	Session *struct {
		Role          string            `json:"role"`
		LegacyCookies map[string]string `json:"legacy_cookies"`
	} `json:"session"`
	Request struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	} `json:"request"`
}

type tryRefusal struct {
	Status int     `json:"status"`
	Reason string  `json:"reason"`
	Field  *string `json:"field"`
}

type tryResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type tryResult struct {
	Accepted bool         `json:"accepted"`
	Entry    *string      `json:"entry"`
	Area     bool         `json:"area"`
	Refusal  *tryRefusal  `json:"refusal"`
	Cookies  []string     `json:"cookies"`
	Response *tryResponse `json:"response"`
}

func (t *Try) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

// ServeHTTP answers one try.
func (t *Try) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if t.Token == "" {
		http.NotFound(w, r)
		return
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(t.Token)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var in tryRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 24<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unreadable request"})
		return
	}
	m, payload, err := mapstore.VerifyTry(in.Map, t.Pub, t.KeyID, t.clock())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}
	method := strings.ToUpper(in.Request.Method)
	switch {
	case in.Mode != "dry_run" && in.Mode != "preview":
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode must be dry_run or preview"})
		return
	case in.Mode == "preview" && method != http.MethodGet && method != http.MethodHead:
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "preview serves GET and HEAD only"})
		return
	}
	st, err := t.sites.get(payload, in.Mode, func() (*site.Site, error) {
		opts := site.Options{Limits: noLimits, Relay: t.Relay}
		if in.Mode == "dry_run" {
			opts.Transport = dryTransport{}
		}
		return site.BuildWith(m, opts)
	})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error()})
		return
	}
	req, err := t.request(r.Context(), st, &in, method)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var cookies []string
	if in.Session != nil {
		c, err := t.seed(r.Context(), st, in.Session.Role, in.Session.LegacyCookies)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		req.AddCookie(c)
		cookies = append(cookies, c.String())
	}
	writeJSON(w, http.StatusOK, t.serve(st, req, in.Mode, cookies))
}

// request builds the visitor's request towards the draft's public domain.
func (t *Try) request(ctx context.Context, st *site.Site, in *tryRequest, method string) (*http.Request, error) {
	if !strings.HasPrefix(in.Request.Path, "/") {
		return nil, errors.New("request.path must start with /")
	}
	u, err := url.ParseRequestURI(in.Request.Path)
	if err != nil {
		return nil, errors.New("request.path is not a path")
	}
	var body []byte
	if in.Request.Body != "" {
		if in.Mode == "preview" {
			return nil, errors.New("preview takes no body")
		}
		if body, err = base64.StdEncoding.DecodeString(in.Request.Body); err != nil {
			return nil, errors.New("request.body is not base64")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = st.Model.Meta.PublicDomain
	req.RemoteAddr = tryAddr
	for k, v := range in.Request.Headers {
		switch http.CanonicalHeaderKey(k) {
		case "Host", "Content-Length", "Transfer-Encoding", "Connection":
			continue
		}
		req.Header.Set(k, v)
	}
	return req, nil
}

// seed opens a try session holding a test account's legacy cookies: the
// agent logged in to the legacy site itself.
func (t *Try) seed(ctx context.Context, st *site.Site, role string, legacy map[string]string) (*http.Cookie, error) {
	if t.Sessions == nil {
		return nil, errors.New("sessions are not available")
	}
	if _, ok := st.Model.Roles.List[role]; !ok {
		return nil, errors.New("session.role is not a role of the map")
	}
	if len(legacy) > maxLegacyCookies {
		return nil, errors.New("too many legacy cookies")
	}
	s := &session.Session{Tenant: st.Model.Meta.Tenant, App: st.Model.Meta.App, Role: role,
		Cookies: map[string]string{}, Private: map[string]bool{}}
	for name, value := range legacy {
		s.Cookies[name], s.Private[name] = value, true
	}
	id := session.NewID()
	if err := t.Sessions.Save(ctx, id, s, st.SessionPolicy()); err != nil {
		return nil, err
	}
	plain := plainSite(st)
	return &http.Cookie{Name: sessionCookieName(plain), Value: id, Path: "/", HttpOnly: true, Secure: !plain}, nil
}

// serve runs the request through the same code as a visitor's, with events
// kept aside.
func (t *Try) serve(st *site.Site, req *http.Request, mode string, cookies []string) tryResult {
	var events bytes.Buffer
	capt := &tryCapture{}
	p := &Public{
		Maps:   singleSite{st},
		IPs:    clientip.New(nil),
		Log:    t.Log,
		Events: telemetry.NewEmitter(&events, "try"),
		Forms:  t.Forms,
	}
	p.Sessions = t.Sessions
	if mode == "dry_run" {
		p.Sessions = t.Frozen
	}
	req = req.WithContext(context.WithValue(req.Context(), tryCaptureKey{}, capt))
	rec := httptest.NewRecorder()
	entry := p.handle(rec, req)

	res := tryResult{Cookies: cookies}
	if mode == "preview" {
		// A dry-run changes no session: its cookies would point at nothing.
		res.Cookies = append(res.Cookies, rec.Result().Header.Values("Set-Cookie")...)
	}
	if entry != "" {
		res.Entry = &entry
		res.Area = isArea(st, entry)
	}
	res.Refusal = refusalOf(&events, rec.Code)
	switch {
	case res.Refusal != nil:
	case mode == "dry_run":
		res.Accepted = capt.forwarded.Load() || rec.Code < http.StatusBadRequest
	default:
		res.Accepted = true
	}
	if mode == "preview" {
		res.Response = previewOf(rec)
	}
	return res
}

// refusalOf reads the last refusal among the try's events.
func refusalOf(events *bytes.Buffer, status int) *tryRefusal {
	var out *tryRefusal
	sc := bufio.NewScanner(events)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		var ev telemetry.Event
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "request.blocked", "field.rejected", "ratelimit.hit", "parse.failed":
		default:
			continue
		}
		ref := &tryRefusal{Status: status, Reason: ev.Type}
		if ev.Reason != nil {
			ref.Reason = *ev.Reason
		}
		if f, ok := ev.Detail["field"].(string); ok && f != "" {
			ref.Field = &f
		}
		out = ref
	}
	return out
}

func previewOf(rec *httptest.ResponseRecorder) *tryResponse {
	h := map[string]string{}
	for k, v := range rec.Result().Header {
		if k != "Set-Cookie" && len(v) > 0 {
			h[k] = v[0]
		}
	}
	body := rec.Body.Bytes()
	if len(body) > legacyclient.MaxBody {
		body = body[:legacyclient.MaxBody]
	}
	return &tryResponse{Status: rec.Code, Headers: h, Body: base64.StdEncoding.EncodeToString(body)}
}
