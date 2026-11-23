package server

import (
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// firstFit picks, among the entries the path has the shape of, the first
// whose path values fit their types: /page/abc/ is no page number but may
// be a child page. It falls through only to an entry that guards the same
// way as the first (the same roles, no authz): a value the first refuses
// never reaches an entry that would answer 403 where the area served it.
// An asset prefix is a candidate only for a path that has no page's shape,
// as before 0.2.14. fits is false when none fits; the most specific is then
// returned, for the checks that still apply to it.
func firstFit(st *site.Site, cands []router.Match) (router.Match, bool) {
	if len(cands) == 0 {
		return router.Match{}, false
	}
	pages := cands[0].Entry.Kind != router.KindAsset
	roles, authz := guards(st, cands[0])
	for i, m := range cands {
		if i > 0 {
			if pages && m.Entry.Kind == router.KindAsset {
				break
			}
			r, a := guards(st, m)
			if a || authz || !sameRoles(r, roles) {
				break
			}
		}
		if pathFits(st, m) {
			return m, true
		}
	}
	return cands[0], false
}

// guards are an entry's roles and whether it decides who opens which object.
func guards(st *site.Site, m router.Match) ([]string, bool) {
	switch m.Entry.Kind {
	case router.KindRoute:
		if r, ok := st.Model.RouteByID(m.Entry.ID); ok {
			return r.Roles, len(r.Authz) > 0
		}
	case router.KindAPI:
		if a, ok := st.Model.APIByID(m.Entry.ID); ok {
			return a.Roles, len(a.Authz) > 0
		}
	case router.KindForm:
		if f, ok := st.Model.FormByID(m.Entry.ID); ok {
			return f.Roles, false
		}
	}
	return nil, false
}

func sameRoles(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, r := range a {
		seen[r] = true
	}
	for _, r := range b {
		if !seen[r] {
			return false
		}
	}
	return true
}

// areaKeepsChecks applies, to a request the area is about to serve because
// it does not fit a precise entry exactly, what that entry still knows, so
// that stepping aside never steps round a check (docs/map-format.md,
// "Areas"):
//
//   - a path value that fits no entry and holds characters no page name has
//     (unsafeInPath) is refused: /{slug}/ against /1'OR'1'='1/;
//   - the query keys the route declares are checked as on the route:
//     /?p=<attack>&zz=1 is refused like /?p=<attack>;
//   - a key the legacy platform would read as a declared one is checked as
//     that one: PHP reads page.id, "page id", " page_id" and page_id[] as
//     page_id, and some servers read P as p (aliasKey).
//
// It answers false when it has refused the request.
func (p *Public) areaKeepsChecks(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match, fits bool) bool {
	if m.Entry.ID == "" {
		return true
	}
	if !fits {
		for name, v := range m.Params {
			if unsafeInPath(v) {
				p.reject(w, r, st, m.Entry.ID, &validate.Error{Field: name, Reason: validate.ReasonCharset})
				return false
			}
		}
		return true
	}
	if m.Entry.Kind != router.KindRoute {
		return true
	}
	route, ok := st.Model.RouteByID(m.Entry.ID)
	if !ok {
		return true
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return true // the area refuses a malformed query itself
	}
	declared := map[string]string{}
	for k := range route.Query {
		declared[aliasKey(k)] = k
	}
	for k, vals := range query {
		d, ok := declared[aliasKey(k)]
		if !ok || d == k {
			continue
		}
		for _, v := range vals { // read as d by the site: checked as d (phpBB's own fid[]=2 still passes)
			if _, _, err := st.Validator.Value(d, route.Query[d], v, true); err != nil {
				p.reject(w, r, st, route.ID, err)
				return false
			}
		}
	}
	if _, err := st.Validator.Fields(present(route.Query, query), query); err != nil {
		p.reject(w, r, st, route.ID, err)
		return false
	}
	return true
}

// reportAreaKeys tells the discovery the names of the keys a precise route
// does not declare, once the area has accepted the request: a refused
// request teaches nothing.
func (p *Public) reportAreaKeys(r *http.Request, st *site.Site, m router.Match, fits bool) {
	if !fits || m.Entry.Kind != router.KindRoute {
		return
	}
	route, ok := st.Model.RouteByID(m.Entry.ID)
	if !ok {
		return
	}
	if query, err := url.ParseQuery(r.URL.RawQuery); err == nil {
		p.reportDropped(r, st, route, query)
	}
}

// aliasKey is a query key as PHP and case-blind servers read it: leading
// spaces gone, "." and " " as "_", cut at its first "[", lower case.
func aliasKey(k string) string {
	k = strings.TrimLeft(k, " ")
	if i := strings.IndexByte(k, '['); i > 0 {
		k = k[:i]
	}
	return strings.ToLower(strings.NewReplacer(".", "_", " ", "_").Replace(k))
}

// present is the part of specs the query has: a key the request lacks is
// the area's business, not a missing required value.
func present(specs map[string]mapmodel.FieldSpec, query url.Values) map[string]mapmodel.FieldSpec {
	out := make(map[string]mapmodel.FieldSpec, len(specs))
	for k, s := range specs {
		if _, ok := query[k]; ok {
			out[k] = s
		}
	}
	return out
}

// unsafeInPath reports whether a decoded path segment holds a character no
// page name has: quotes, angle brackets, braces, backticks, pipes,
// backslashes, carets, control characters. An apostrophe between two
// letters is a name (/wiki/Don't_Stop, /l'evento/), as no_sql_meta allows.
func unsafeInPath(v string) bool {
	if strings.ContainsAny(v, "\"<>`{}|\\^") || strings.IndexFunc(v, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0 {
		return true
	}
	rs := []rune(v)
	for i, r := range rs {
		if r == '\'' && (i == 0 || i == len(rs)-1 || !unicode.IsLetter(rs[i-1]) || !unicode.IsLetter(rs[i+1])) {
			return true
		}
	}
	return false
}
