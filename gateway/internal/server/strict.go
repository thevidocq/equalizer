package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// firstFit picks, among the entries the path has the shape of, the first
// whose path values fit their types: /wp-includes/js/x.js is not a dated
// post but a file of /wp-includes/. fits is false when none does; the most
// specific is then returned, for the checks that still apply to it.
func firstFit(st *site.Site, cands []router.Match) (router.Match, bool) {
	for _, m := range cands {
		if pathFits(st, m) {
			return m, true
		}
	}
	if len(cands) == 0 {
		return router.Match{}, false
	}
	return cands[0], false
}

// areaKeepsChecks applies, to a request the area is about to serve because
// it does not fit a precise entry exactly, what that entry still knows, so
// that stepping aside never steps round a check (docs/map-format.md,
// "Areas"):
//
//   - a path value with characters no page name has (quotes, angle
//     brackets, braces, backticks, pipes, backslashes, carets, control
//     characters) is refused: /{slug}/ against /1'OR'1'='1/;
//   - the query keys the route declares are checked as on the route:
//     /?p=<attack>&zz=1 is refused like /?p=<attack>; the keys it does not
//     declare go on to the area, and are reported for the discovery.
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
	if _, err := st.Validator.Fields(present(route.Query, query), query); err != nil {
		p.reject(w, r, st, route.ID, err)
		return false
	}
	p.reportDropped(r, st, route, query)
	return true
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
// page name has: RFC 3986 leaves them out of paths, browsers encode them,
// and they are what injections need.
func unsafeInPath(v string) bool {
	return strings.ContainsAny(v, "'\"<>`{}|\\^") || strings.IndexFunc(v, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0
}
