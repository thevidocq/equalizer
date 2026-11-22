package server

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// droppedKey is a query key worth reporting: a name, as a map can hold it.
var droppedKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// maxDropped bounds what one request may report.
const maxDropped = 5

// tracking keys are dropped on purpose and never reported.
var tracking = map[string]bool{"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "yclid": true,
	"mc_cid": true, "mc_eid": true, "_ga": true, "_gl": true, "igshid": true, "ref": true}

// reportDropped tells discovery which query keys a route dropped because the
// map does not declare them: a sort or a filter the study never saw, which
// would otherwise change the page in silence. Only the key's name is sent,
// never its value (/shop/?orderby=).
//
// The route's path pattern is sent, not the request's path: /users/{name}
// never carries a visitor's name, and one route is one report.
func (p *Public) reportDropped(r *http.Request, st *site.Site, route *mapmodel.Route, query url.Values) {
	if p.Unmapped == nil || len(query) == 0 {
		return
	}
	declared := route.Query
	keys := make([]string, 0, len(query))
	for k := range query {
		if _, ok := declared[k]; ok || tracking[k] || strings.HasPrefix(k, "utm_") || !droppedKey.MatchString(k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > maxDropped {
		keys = keys[:maxDropped]
	}
	for _, k := range keys {
		p.Unmapped.Record(st.Model.Meta.App, http.MethodGet, route.Path+"?"+k+"=", p.IPs.IP(r))
	}
}
