package server

import (
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// pathFits reports whether the path parameters of a match have the types
// of their entry. When they do not, the request is not for that entry.
func pathFits(st *site.Site, m router.Match) bool {
	if len(m.Params) == 0 {
		return true
	}
	var spec map[string]mapmodel.FieldSpec
	switch m.Entry.Kind {
	case router.KindRoute:
		if r, ok := st.Model.RouteByID(m.Entry.ID); ok {
			spec = r.Params
		}
	case router.KindAPI:
		if a, ok := st.Model.APIByID(m.Entry.ID); ok {
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
