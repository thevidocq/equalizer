package server

import (
	"net/http"
)

// optionsToLegacy reports whether an OPTIONS request is the application's
// own (WordPress's block editor asks the REST API what the user may do this
// way) and its area lists OPTIONS: then it is rebuilt and sent on like any
// other request. CORS preflights are never sent on.
func (p *Public) optionsToLegacy(r *http.Request) bool {
	if r.Header.Get("Access-Control-Request-Method") != "" {
		return false
	}
	active := p.Maps.For(r.Host)
	if active == nil {
		return false
	}
	a := active.Model.AreaFor(r.URL.Path)
	return a != nil && a.Allows(http.MethodOptions)
}
