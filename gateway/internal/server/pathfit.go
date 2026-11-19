package server

import (
	"net/url"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// pathFits reports whether the path parameters of a match have the types
// of their entry. When they do not, the request is not for that entry,
// unless the entry decides who opens which object: then it always answers
// (and refuses what does not fit), so that no value gets round it.
func pathFits(st *site.Site, m router.Match) bool {
	if len(m.Params) == 0 {
		return true
	}
	var spec map[string]mapmodel.FieldSpec
	switch m.Entry.Kind {
	case router.KindRoute:
		if r, ok := st.Model.RouteByID(m.Entry.ID); ok {
			if len(r.Authz) > 0 {
				return true // never bypassed: /permits/5abc must not reach object 5 through the area
			}
			spec = r.Params
		}
	case router.KindAPI:
		if a, ok := st.Model.APIByID(m.Entry.ID); ok {
			if len(a.Authz) > 0 {
				return true
			}
			spec = a.Params
		}
	case router.KindForm:
		if f, ok := st.Model.FormByID(m.Entry.ID); ok {
			spec = f.Params
		}
	default:
		return true
	}
	_, err := st.Validator.Fields(spec, single(m.Params))
	return err == nil
}

// queryFits reports whether a route declares every query key the request
// has. A key it does not declare would be dropped and the page would change
// meaning (Joomla's /?option=com_finder&q=… is not its home page): such a
// request is not for that route.
func queryFits(st *site.Site, m router.Match, rawQuery string) bool {
	if m.Entry.Kind != router.KindRoute || rawQuery == "" {
		return true
	}
	r, ok := st.Model.RouteByID(m.Entry.ID)
	if !ok || len(r.Authz) > 0 {
		return true // a route that decides who opens which object is never bypassed
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return true // the route refuses it as malformed
	}
	for k := range q {
		if _, declared := r.Query[k]; !declared {
			return false
		}
	}
	return true
}
