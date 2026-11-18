// Package mapmodel defines the map types used by the gateway.
package mapmodel

// Model is the subset of the map (schema/map/gateway-map.schema.json) used so
// far by the gateway. Full validation against the schema happens in the panel
// before signing; the gateway trusts only the signature.
//
// TODO: generate these types from schema/ once code generation is available
// (see CHANGELOG).
type Model struct {
	Meta     Meta               `json:"meta"`
	Legacy   Legacy             `json:"legacy"`
	Roles    Roles              `json:"roles"`
	Defaults Defaults           `json:"defaults"`
	Types    map[string]TypeDef `json:"types"`
	Routes   []Route            `json:"routes"`
	Forms    []Form             `json:"forms"`
	API      []APIRoute         `json:"api"`
	Compat   []Compat           `json:"compat"`
	Assets   []Asset            `json:"assets"`
	Behavior []BehaviorRule     `json:"behavior"`
}

// RateLimit holds limits in the form "N/unit" (60/min, 5/hour).
type RateLimit struct {
	PerIP      string `json:"per_ip"`
	PerSession string `json:"per_session"`
	PerUser    string `json:"per_user"`
}

// BehaviorRule reacts to a pattern of requests (enumeration, brute force).
type BehaviorRule struct {
	ID        string    `json:"id"`
	Route     string    `json:"route"` // route or form id; empty: every entry
	Signal    string    `json:"signal"`
	Param     string    `json:"param"`
	Threshold Threshold `json:"threshold"`
	Action    []string  `json:"action"`
}

// Threshold of a behaviour rule.
type Threshold struct {
	Count  int    `json:"count"`
	Window string `json:"window"` // e.g. 10m
	Per    string `json:"per"`    // ip, session, user
}

// Asset is a prefix of legacy static files served by the gateway.
type Asset struct {
	Prefix    string   `json:"prefix"`
	Types     []string `json:"types"`
	MaxSizeKB int      `json:"max_size_kb"`
	Roles     []string `json:"roles"`
}

// Meta identifies tenant, app and version.
type Meta struct {
	Schema  string `json:"schema"`
	Tenant  string `json:"tenant"`
	App     string `json:"app"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	// PublicDomain is the host this map is served on (set by the panel).
	PublicDomain string `json:"public_domain"`
}

// Legacy describes how to reach the protected application.
type Legacy struct {
	BaseURL        string          `json:"base_url"`
	Reach          Reach           `json:"reach"`
	UserAgent      string          `json:"user_agent"`
	TimeoutMS      int             `json:"timeout_ms"`
	SessionExpired *SessionExpired `json:"session_expired"`
}

// SessionExpired tells how the legacy application signals that its own
// session ended (the gateway then drops the role granted at login).
type SessionExpired struct {
	RedirectTo   string `json:"redirect_to"`
	Status       []int  `json:"status"`
	BodyContains string `json:"body_contains"`
}

// Reach says whether the legacy application is reached through a connector or directly.
type Reach struct {
	Mode        string   `json:"mode"`
	ConnectorID string   `json:"connector_id"`
	Allow       []string `json:"allow"`
	// Address to connect to instead of base_url's host (virtual hosts,
	// internal-only listeners); base_url's host still goes in Host and SNI.
	Address string `json:"address"`
}

// Roles lists the roles and the one given to unauthenticated visitors.
type Roles struct {
	Anonymous string         `json:"anonymous"`
	List      map[string]any `json:"list"`
}

// Defaults holds the map's default values.
type Defaults struct {
	String       StringDefaults `json:"string"`
	LegacyBudget LegacyBudget   `json:"legacy_budget"`
	Session      SessionPolicy  `json:"session"`
	RateLimit    RateLimit      `json:"rate_limit"`
}

// SessionPolicy limits the life of gateway sessions.
type SessionPolicy struct {
	IdleMinutes   int `json:"idle_minutes"`
	AbsoluteHours int `json:"absolute_hours"`
}

// StringDefaults applies to string fields without constraints of their own.
type StringDefaults struct {
	MaxLength int    `json:"max_length"`
	Charset   string `json:"charset"`
}

// LegacyBudget limits the load sent to the legacy application.
type LegacyBudget struct {
	MaxConcurrency int     `json:"max_concurrency"`
	MaxRPS         float64 `json:"max_rps"`
}

// TypeDef is a reusable type defined in "types".
type TypeDef struct {
	Base      string   `json:"base"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Values    []string `json:"values"`
	Deny      []string `json:"deny"` // refused values, without case
	Pattern   string   `json:"pattern"`
	Charset   string   `json:"charset"`
	MaxLength int      `json:"max_length"`
}

// FieldSpec describes a value coming from the client.
type FieldSpec struct {
	Type      string   `json:"type"`
	Required  *bool    `json:"required"`
	Default   any      `json:"default"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Values    []string `json:"values"`
	Deny      []string `json:"deny"`
	Pattern   string   `json:"pattern"`
	Charset   string   `json:"charset"`
	MaxLength int      `json:"max_length"`
	Rules     []string `json:"rules"`
	MIME      []string `json:"mime"`        // file: allowed real types
	MaxSizeKB int      `json:"max_size_kb"` // file: size limit
	Scan      string   `json:"scan"`        // file: cdr (rebuild images) or none
}

// LegacyRequest is the request the gateway builds towards the legacy application.
type LegacyRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Form    map[string]string `json:"form"`
	Files   map[string]string `json:"files"`
	Headers map[string]string `json:"headers"`
	// JSON is a template of the JSON body: strings like "{{body.x}}" are
	// replaced by the validated value, with its JSON type.
	JSON map[string]any `json:"json"`
}

// Route is a read-only page.
type Route struct {
	ID            string               `json:"id"`
	Path          string               `json:"path"`
	Method        string               `json:"method"`
	Roles         []string             `json:"roles"`
	Render        string               `json:"render"`
	Params        map[string]FieldSpec `json:"params"`
	Query         map[string]FieldSpec `json:"query"`
	LegacyRequest LegacyRequest        `json:"legacy_request"`
	Links         []Link               `json:"links"`
	Session       *SessionAction       `json:"session"`
	RateLimit     RateLimit            `json:"rate_limit"`
	Authz         map[string]AuthzRule `json:"authz"` // by role
}

// AuthzRule says which objects a role may open on an entry.
type AuthzRule struct {
	Check  string `json:"check"`  // owned_by_user, always, never
	Target string `json:"target"` // e.g. params.id: the value checked
}

// Link maps a legacy URL found in a page to a public route.
// Legacy format: "/path?key={name}", where {name} is a parameter or a query
// value of the target route.
type Link struct {
	Legacy string `json:"legacy"`
	Route  string `json:"route"`
}

// Form is a write action (POST) on a public path.
type Form struct {
	ID            string               `json:"id"`
	Route         string               `json:"route"`
	Roles         []string             `json:"roles"`
	Params        map[string]FieldSpec `json:"params"`
	Fields        map[string]FieldSpec `json:"fields"`
	LegacyRequest LegacyRequest        `json:"legacy_request"`
	Success       *SuccessCheck        `json:"success"`
	OnSuccess     *OnSuccess           `json:"on_success"`
	Session       *SessionAction       `json:"session"`
	RateLimit     RateLimit            `json:"rate_limit"`
}

// SessionAction starts (login: the role granted) or ends (logout) the
// gateway session.
type SessionAction struct {
	Start string `json:"start"`
	End   bool   `json:"end"`
}

// SuccessCheck tells whether the legacy application accepted a submission.
type SuccessCheck struct {
	Cookie       string `json:"cookie"`
	Status       []int  `json:"status"`
	BodyContains string `json:"body_contains"`
	NotContains  string `json:"not_contains"`
	RedirectTo   string `json:"redirect_to"`
}

// OnSuccess says what the visitor sees after a successful submission.
type OnSuccess struct {
	Redirect string `json:"redirect"` // route id
}

// APIRoute is a JSON endpoint for dynamic parts of a page.
type APIRoute struct {
	ID            string               `json:"id"`
	Path          string               `json:"path"`
	Method        string               `json:"method"`
	Roles         []string             `json:"roles"`
	Params        map[string]FieldSpec `json:"params"`
	Query         map[string]FieldSpec `json:"query"`
	Body          map[string]FieldSpec `json:"body"`
	LegacyRequest LegacyRequest        `json:"legacy_request"`
	Response      APIResponse          `json:"response"`
	RateLimit     RateLimit            `json:"rate_limit"`
	Authz         map[string]AuthzRule `json:"authz"`
}

// APIResponse says which part of the legacy JSON reaches the client: only
// the declared fields, with their types.
type APIResponse struct {
	List   bool                 `json:"list"`
	Path   string               `json:"path"` // e.g. data.items; empty: the whole document
	Fields map[string]FieldSpec `json:"fields"`
}

// APIByID returns the API route with the given id.
func (m *Model) APIByID(id string) (*APIRoute, bool) {
	for i := range m.API {
		if m.API[i].ID == id {
			return &m.API[i], true
		}
	}
	return nil, false
}

// Compat is a historical legacy URL.
type Compat struct {
	Legacy string `json:"legacy"`
	Action string `json:"action"`
	Route  string `json:"route"`
}

// FormByID returns the form with the given id.
func (m *Model) FormByID(id string) (*Form, bool) {
	for i := range m.Forms {
		if m.Forms[i].ID == id {
			return &m.Forms[i], true
		}
	}
	return nil, false
}

// RouteByID returns the route with the given id.
func (m *Model) RouteByID(id string) (*Route, bool) {
	for i := range m.Routes {
		if m.Routes[i].ID == id {
			return &m.Routes[i], true
		}
	}
	return nil, false
}
