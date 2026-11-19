// Package site compiles one map version into everything needed to serve it:
// router, validator, legacy client, budget, URL translation and rendering
// pipeline. A Site is immutable: a new map produces a new Site.
package site

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/budget"
	"github.com/thevidocq/equalizer/gateway/internal/formmap"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/legacyurl"
	"github.com/thevidocq/equalizer/gateway/internal/limits"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/oidc"
	"github.com/thevidocq/equalizer/gateway/internal/origin"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/clean"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/forms"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/rewrite"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
	"github.com/thevidocq/equalizer/gateway/internal/upload"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// ErrConnectorUnavailable is returned in connector mode when no relay is
// configured or no tunnel is open.
var ErrConnectorUnavailable = transport.ErrConnectorUnavailable

// Options are the installation settings a map cannot carry.
type Options struct {
	Relay  *transport.RelayConfig // nil: connector mode answers 502
	Limits limits.Store           // nil: counters in this process only
	// Transport replaces the way to the legacy site, whatever the reach
	// mode (the agent's dry-run tries: nothing is forwarded).
	Transport http.RoundTripper
}

// How long to wait when the legacy application is saturated.
const budgetWait = 2 * time.Second

// Site is a compiled map ready to serve requests.
type Site struct {
	Model     *mapmodel.Model
	Router    *router.Router
	Validator *validate.Validator
	Legacy    *legacyclient.Client
	Budget    *budget.Limiter
	URLs      *legacyurl.Rewriter
	Origin    *origin.Rewriter // legacy origin → public one in text bodies; nil: nothing to do
	Inbound   *origin.Rewriter // public origin → legacy one in area queries and bodies
	Forms     *formmap.Map
	Pipeline  *pipeline.Pipeline
	Limits    *limits.Set
	Notes     []string       // map features accepted but not applied yet
	OIDC      *oidc.Provider // nil: the legacy login only (default)

	charset     atomic.Value                 // legacy charset seen in the last rendered page
	unsupported map[string]string            // form id → reason it cannot be served yet
	owned       map[string]map[string]string // entry → role → owned_by_user target
}

// DefaultSteps is the default order of the faithful-rendering steps.
// forms must run before rewrite: it needs the legacy action URLs.
func DefaultSteps() []pipeline.Step {
	return []pipeline.Step{clean.Step{}, forms.Step{}, rewrite.Step{}}
}

// Build compiles the map without installation options (tests, direct mode).
func Build(m *mapmodel.Model) (*Site, error) { return BuildWith(m, Options{}) }

// BuildWith compiles the map. It rejects inconsistent maps instead of
// serving them half-working.
func BuildWith(m *mapmodel.Model, opts Options) (*Site, error) {
	rt, err := router.New(Entries(m))
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	if m.Roles.Anonymous != "" {
		if _, ok := m.Roles.List[m.Roles.Anonymous]; !ok {
			return nil, fmt.Errorf("roles.anonymous %q is not in roles.list", m.Roles.Anonymous)
		}
	}

	idp, err := checkOIDC(m)
	if err != nil {
		return nil, err
	}
	if err := checkEntryRoles(m); err != nil {
		return nil, err
	}
	if err := checkAreas(m); err != nil {
		return nil, err
	}

	v := validate.New(m.Types, m.Defaults.String)
	for _, r := range m.Routes {
		if err := v.Check(r.Params); err != nil {
			return nil, fmt.Errorf("route %q, params: %w", r.ID, err)
		}
		if err := v.Check(r.Query); err != nil {
			return nil, fmt.Errorf("route %q, query: %w", r.ID, err)
		}
		if err := checkTemplates(r.LegacyRequest); err != nil {
			return nil, fmt.Errorf("route %q: %w", r.ID, err)
		}
		if err := checkAuthz(m, r.Authz, map[string]map[string]mapmodel.FieldSpec{"params": r.Params, "query": r.Query}); err != nil {
			return nil, fmt.Errorf("route %q: %w", r.ID, err)
		}
	}

	for _, a := range m.API {
		if err := checkAPI(v, a); err != nil {
			return nil, fmt.Errorf("api %q: %w", a.ID, err)
		}
		if err := checkAuthz(m, a.Authz, map[string]map[string]mapmodel.FieldSpec{"params": a.Params, "query": a.Query, "body": a.Body}); err != nil {
			return nil, fmt.Errorf("api %q: %w", a.ID, err)
		}
	}

	unsupported := map[string]string{}
	for _, f := range m.Forms {
		if err := checkForm(m, v, f); err != nil {
			return nil, fmt.Errorf("form %q: %w", f.ID, err)
		}
		if reason := formUnsupported(f); reason != "" {
			unsupported[f.ID] = reason
		}
	}

	timeout := time.Duration(m.Legacy.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var tr http.RoundTripper
	switch m.Legacy.Reach.Mode {
	case "direct":
		tr, err = transport.Direct(m.Legacy.Reach.Allow, m.Legacy.Reach.Address, timeout)
		if err != nil {
			return nil, fmt.Errorf("legacy.reach: %w", err)
		}
	case "connector":
		if opts.Relay == nil {
			tr = unavailable{}
			break
		}
		tr, err = transport.Relay(opts.Relay, m.Legacy.Reach.ConnectorID, m.Meta.Tenant, m.Legacy.Reach.Allow, m.Legacy.Reach.Address, timeout)
		if err != nil {
			return nil, fmt.Errorf("legacy.reach: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown legacy.reach.mode: %q", m.Legacy.Reach.Mode)
	}
	if opts.Transport != nil {
		tr = opts.Transport
	}
	lc, err := legacyclient.New(m.Legacy.BaseURL, tr, m.Legacy.UserAgent, timeout)
	if err != nil {
		return nil, err
	}

	urls, err := legacyurl.New(lc.Base(), m)
	if err != nil {
		return nil, fmt.Errorf("links: %w", err)
	}
	urls.Fits = func(route *mapmodel.Route, params map[string]string) bool {
		vals := url.Values{}
		for k, val := range params {
			vals.Set(k, val)
		}
		_, err := v.Fields(route.Params, vals)
		return err == nil
	}

	store := opts.Limits
	if store == nil {
		store = limits.NewMemory()
	}
	lim, notes, err := limits.Compile(m, store)
	if err != nil {
		return nil, err
	}

	owned := ownedTargets(m)
	for _, a := range m.API {
		if err := checkIssues(owned, a.Response); err != nil {
			return nil, fmt.Errorf("api %q: %w", a.ID, err)
		}
	}

	return &Site{
		Limits:    lim,
		Notes:     notes,
		Model:     m,
		Router:    rt,
		Validator: v,
		Legacy:    lc,
		Budget:    budget.New(m.Defaults.LegacyBudget.MaxConcurrency, budgetWait),
		URLs:      urls,
		Origin:    origin.New(lc.Base(), m.Meta.Scheme(), m.Meta.PublicDomain),
		Inbound:   origin.NewReverse(lc.Base(), m.Meta.Scheme(), m.Meta.PublicDomain),
		Forms:     formmap.New(lc.Base(), m),
		Pipeline:  pipeline.New(DefaultSteps()...),

		unsupported: unsupported,
		owned:       owned,
		OIDC:        idp,
	}, nil
}

// SessionPolicy returns the map's session limits, with defaults.
func (s *Site) SessionPolicy() session.Policy {
	p := session.DefaultPolicy
	if m := s.Model.Defaults.Session.IdleMinutes; m > 0 {
		p.Idle = time.Duration(m) * time.Minute
	}
	if h := s.Model.Defaults.Session.AbsoluteHours; h > 0 {
		p.Absolute = time.Duration(h) * time.Hour
	}
	if d := s.Model.Defaults.Session.RememberDays; d != nil {
		p.Remember = time.Duration(*d) * 24 * time.Hour
	}
	return p
}

// FormUnsupported returns why a declared form cannot be served yet ("" if it can).
func (s *Site) FormUnsupported(id string) string { return s.unsupported[id] }

func checkForm(m *mapmodel.Model, v *validate.Validator, f mapmodel.Form) error {
	if err := v.Check(f.Params); err != nil {
		return fmt.Errorf("params: %w", err)
	}
	if err := v.Check(f.Fields); err != nil {
		return fmt.Errorf("fields: %w", err)
	}
	if err := checkTemplates(f.LegacyRequest); err != nil {
		return err
	}
	for name, spec := range f.Fields {
		if spec.Type != "file" {
			continue
		}
		if len(spec.MIME) == 0 || spec.MaxSizeKB <= 0 {
			return fmt.Errorf("field %q: a file needs mime and max_size_kb", name)
		}
		if spec.Scan == "cdr" {
			for _, t := range spec.MIME {
				if !slices.Contains(upload.CDRTypes, t) {
					return fmt.Errorf("field %q: cdr rebuilds only %v, not %s", name, upload.CDRTypes, t)
				}
			}
		}
	}
	if f.Session != nil && f.Session.Start != "" {
		if _, ok := m.Roles.List[f.Session.Start]; !ok {
			return fmt.Errorf("session.start: role %q is not in roles.list", f.Session.Start)
		}
	}
	if f.OnSuccess != nil && f.OnSuccess.Redirect != "" {
		if _, ok := m.RouteByID(f.OnSuccess.Redirect); !ok {
			return fmt.Errorf("on_success.redirect: route %q does not exist", f.OnSuccess.Redirect)
		}
	}
	return nil
}

func checkAPI(v *validate.Validator, a mapmodel.APIRoute) error {
	for part, specs := range map[string]map[string]mapmodel.FieldSpec{"params": a.Params, "query": a.Query, "body": a.Body, "response.fields": a.Response.Fields} {
		if err := v.Check(specs); err != nil {
			return fmt.Errorf("%s: %w", part, err)
		}
	}
	if len(a.Response.Fields) == 0 {
		return fmt.Errorf("response.fields: declare at least one field")
	}
	if len(a.Body) > 0 && (a.Method == http.MethodGet || a.Method == http.MethodDelete) {
		return fmt.Errorf("body: %s requests carry no body", a.Method)
	}
	if len(a.LegacyRequest.Files) > 0 {
		return fmt.Errorf("legacy_request.files: not supported for APIs")
	}
	if err := checkTemplates(a.LegacyRequest); err != nil {
		return err
	}
	return legacyclient.CheckJSON(map[string]any(a.LegacyRequest.JSON))
}

// formUnsupported lists what a form needs and this installation lacks.
// Such forms are recognised in pages but answered with 503.
func formUnsupported(f mapmodel.Form) string {
	if !strings.EqualFold(f.LegacyRequest.Method, http.MethodPost) {
		return "legacy_method"
	}
	return ""
}

// AssetByPrefix returns the asset entry with the given prefix.
func (s *Site) AssetByPrefix(prefix string) (*mapmodel.Asset, bool) {
	for i := range s.Model.Assets {
		if s.Model.Assets[i].Prefix == prefix {
			return &s.Model.Assets[i], true
		}
	}
	return nil, false
}

// RoleAllowed reports whether the role may access an entry with those roles.
func RoleAllowed(role string, allowed []string) bool {
	return role != "" && slices.Contains(allowed, role)
}

// Entries turns the map's routes, forms, APIs and assets into router entries.
func Entries(m *mapmodel.Model) []router.Entry {
	var out []router.Entry
	for _, r := range m.Routes {
		out = append(out, router.Entry{ID: r.ID, Kind: router.KindRoute, Method: http.MethodGet, Pattern: r.Path})
	}
	for _, f := range m.Forms {
		out = append(out, router.Entry{ID: f.ID, Kind: router.KindForm, Method: http.MethodPost, Pattern: f.Route})
	}
	for _, a := range m.API {
		out = append(out, router.Entry{ID: a.ID, Kind: router.KindAPI, Method: a.Method, Pattern: a.Path})
	}
	for _, a := range m.Assets {
		out = append(out, router.Entry{ID: AssetID(a.Prefix), Kind: router.KindAsset, Method: http.MethodGet, Pattern: a.Prefix})
	}
	return out
}

// AssetID is the routing id of an asset prefix.
func AssetID(prefix string) string { return "asset:" + prefix }

func checkTemplates(lr mapmodel.LegacyRequest) error {
	if err := placeholder.Check(lr.Path); err != nil {
		return fmt.Errorf("legacy_request.path: %w", err)
	}
	for _, group := range []map[string]string{lr.Query, lr.Form, lr.Files, lr.Headers} {
		for k, tpl := range group {
			if err := placeholder.Check(tpl); err != nil {
				return fmt.Errorf("legacy_request %q: %w", k, err)
			}
		}
	}
	return nil
}

type unavailable struct{}

func (unavailable) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, ErrConnectorUnavailable
}
