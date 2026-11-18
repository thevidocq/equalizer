package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/jsonshape"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// Authorisation by "issued values": with owned_by_user, a session may open
// only the objects the legacy site itself showed it (links in its pages,
// its redirects, the fields of its JSON answers named in response.issues). A user who changes /permits/42 into /permits/43 gets 403
// unless the site listed 43 for them. Forms need no rule: their parameters
// travel sealed in the form token.

const (
	maxIssuedKeys   = 64
	maxIssuedValues = 2000
)

// issue records that the site showed this session value for key.
func (ss *sessionState) issue(key, value string) {
	if ss.broken || ss.ended || value == "" || len(value) > 200 {
		return
	}
	if ss.s == nil {
		ss.s = newSession(ss)
	}
	if ss.s.Issued == nil {
		ss.s.Issued = map[string][]string{}
	}
	vals, known := ss.s.Issued[key]
	if !known && len(ss.s.Issued) >= maxIssuedKeys {
		return
	}
	for _, v := range vals {
		if v == value {
			return
		}
	}
	if len(vals) >= maxIssuedValues {
		vals = vals[1:] // the oldest goes
	}
	ss.s.Issued[key] = append(vals, value)
	ss.dirty = true
}

func (ss *sessionState) issued(key, value string) bool {
	if ss.s == nil {
		return false
	}
	for _, v := range ss.s.Issued[key] {
		if v == value {
			return true
		}
	}
	return false
}

// authorize applies the entry's rule for the visitor's role.
func (p *Public) authorize(w http.ResponseWriter, r *http.Request, st *site.Site, entry string,
	rules map[string]mapmodel.AuthzRule, vals placeholder.Values, api bool) bool {
	ss := sessionOf(r)
	rule, ok := rules[ss.role(st)]
	if !ok || rule.Check == "always" {
		return true
	}
	reason := "authz_never"
	if rule.Check == "owned_by_user" {
		ns, name, _ := strings.Cut(rule.Target, ".")
		v, present := vals[ns][name]
		if !present || ss.issued(site.IssueKey(entry, rule.Target), v) {
			return true
		}
		reason = "authz_not_owned"
	}
	ev := p.event(r, st, "request.blocked", reason, http.StatusForbidden, telemetry.Ptr(entry))
	p.Events.Emit(ev)
	if api {
		writeAPIError(w, http.StatusForbidden, "forbidden")
	} else {
		writePage(w, http.StatusForbidden, pageForbidden)
	}
	return false
}

// issueFromJSON issues the values an API answer shows (response.issues),
// before the session is written with the headers.
func issueFromJSON(ss *sessionState, api *mapmodel.APIRoute, out []byte) {
	for field, to := range api.Response.Issues {
		entry, target, _ := strings.Cut(to, ".")
		for _, v := range jsonshape.Values(out, field) {
			ss.issue(site.IssueKey(entry, target), v)
		}
	}
}

// issuer wraps the page's URL mapper: every public link it produces towards
// an entry with owned_by_user issues the value to the session.
type issuer struct {
	pipeline.URLMapper
	st *site.Site
	ss *sessionState
}

func (p *Public) issuing(st *site.Site, ss *sessionState) pipeline.URLMapper {
	return &issuer{URLMapper: st.URLs, st: st, ss: ss}
}

func (i *issuer) Public(u *url.URL) (string, bool) {
	out, ok := i.URLMapper.Public(u)
	if ok {
		i.note(out)
	}
	return out, ok
}

// note issues the values of a public URL that the visitor will be able to open.
func (i *issuer) note(public string) {
	u, err := url.Parse(public)
	if err != nil {
		return
	}
	m, err := i.st.Router.Match(http.MethodGet, u.Path)
	if err != nil {
		return
	}
	q := u.Query()
	for _, target := range i.st.Owned(m.Entry.ID) {
		ns, name, _ := strings.Cut(target, ".")
		v := ""
		switch ns {
		case "params":
			v = m.Params[name]
		case "query":
			v = q.Get(name)
		}
		i.ss.issue(site.IssueKey(m.Entry.ID, target), v)
	}
}
