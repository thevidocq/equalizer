package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapschema"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// adoptIn: which listed item to add; the change itself comes from the report.
type adoptIn struct {
	Kind  string `json:"kind"`
	Path  string `json:"path"`
	Route string `json:"route"`
	Key   string `json:"key"`
}

// adoptReport is the part of a discovery report the person can adopt.
type adoptReport struct {
	Files []struct {
		Path string `json:"path"`
		Add  *struct {
			Prefix string `json:"prefix"`
			Type   string `json:"type"`
		} `json:"add"`
	} `json:"files_refused"`
	Keys []struct {
		Route string `json:"route"`
		Key   string `json:"key"`
		Add   *struct {
			RouteID string `json:"route_id"`
			Key     string `json:"key"`
		} `json:"add"`
	} `json:"query_dropped"`
}

// A key a visitor asked for, added by a person: text without markup or
// template code; verification narrows it from the values it sees.
var adoptedKey = map[string]any{"type": "string", "max_length": 200, "required": false,
	"rules": []any{"no_html", "no_template_meta"}}

var adoptName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var errNotListed = errors.New("not listed")

// adoptDiscovery adds one item of a discovery's visitor lists to a new
// draft: the discovery's draft if it is still one, else the published map.
func (s *Server) adoptDiscovery(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in adoptIn
	if !decode(w, r, &in) {
		return
	}
	d, err := s.Store.Discovery(r.Context(), a.ID, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	var rep adoptReport
	if err := json.Unmarshal(d.Report, &rep); err != nil {
		fail(w, http.StatusNotFound, "not_found", "this discovery has no report")
		return
	}
	base, err := s.adoptBase(r, a.ID, d)
	if err != nil {
		fail(w, http.StatusConflict, "no_base", "no draft of this discovery nor published map to add it to")
		return
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		s.storeError(w, r, err)
		return
	}
	note, err := adoptApply(doc, rep, in)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "not_listed", "this item is not one the discovery listed with a change to add")
		return
	}
	raw, _ := json.Marshal(doc)
	out, err := store.SetMeta(raw, map[string]any{
		"schema": "gateway-map/1.0", "tenant": a.Tenant, "app": a.ID, "version": 1, "status": "draft",
		"created_by": "user:" + u.Email, "public_domain": a.PublicDomain,
	})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if err := mapschema.Validate(out); err != nil {
		var ie *mapschema.ErrInvalid
		if errors.As(err, &ie) {
			fail(w, http.StatusUnprocessableEntity, "invalid_map", "the map does not respect the schema", ie.Details...)
			return
		}
		s.storeError(w, r, err)
		return
	}
	if !s.checkMap(w, r, out, false) {
		return
	}
	note = fmt.Sprintf("From version %d, discovery #%d: %s", base.Version, d.ID, note)
	version, err := s.Store.CreateDraft(r.Context(), a.ID, out, note, u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map.adopted", a.ID,
		map[string]any{"version": version, "from": base.Version, "discovery": d.ID, "kind": in.Kind})
	writeJSON(w, http.StatusCreated, map[string]any{"app": a.ID, "version": version, "status": "draft"})
}

// adoptBase is the version the change goes on: the discovery's draft while
// it is one, else the published map.
func (s *Server) adoptBase(r *http.Request, app string, d store.Discovery) (store.MapVersion, error) {
	if d.DraftVersion != nil {
		if v, err := s.Store.Version(r.Context(), app, *d.DraftVersion); err == nil && v.Status == "draft" {
			return v, nil
		}
	}
	return s.Store.Published(r.Context(), app)
}

// adoptApply changes doc as the report's item says; the note describes it.
func adoptApply(doc map[string]any, rep adoptReport, in adoptIn) (string, error) {
	switch in.Kind {
	case "file":
		for _, f := range rep.Files {
			if f.Path != in.Path || f.Add == nil {
				continue
			}
			assets, _ := doc["assets"].([]any)
			for _, x := range assets {
				asset, _ := x.(map[string]any)
				if asset == nil || asset["prefix"] != f.Add.Prefix {
					continue
				}
				types, _ := asset["types"].([]any)
				if !slices.Contains(types, any(f.Add.Type)) {
					asset["types"] = append(types, f.Add.Type)
				}
				return fmt.Sprintf("%s added to %s, for %s (asked by visitors)", f.Add.Type, f.Add.Prefix, f.Path), nil
			}
		}
	case "key":
		for _, k := range rep.Keys {
			if k.Route != in.Route || k.Key != in.Key || k.Add == nil || !adoptName.MatchString(k.Add.Key) {
				continue
			}
			routes, _ := doc["routes"].([]any)
			for _, x := range routes {
				route, _ := x.(map[string]any)
				if route == nil || route["id"] != k.Add.RouteID {
					continue
				}
				lr, _ := route["legacy_request"].(map[string]any)
				if lr == nil || (route["method"] != nil && route["method"] != "GET") || (lr["method"] != nil && lr["method"] != "GET") {
					break
				}
				q, _ := route["query"].(map[string]any)
				if q == nil {
					q = map[string]any{}
				}
				if _, taken := q[k.Add.Key]; !taken {
					q[k.Add.Key] = adoptedKey
				}
				route["query"] = q
				lq, _ := lr["query"].(map[string]any)
				if lq == nil {
					lq = map[string]any{}
				}
				lq[k.Add.Key] = "{{query." + k.Add.Key + "}}"
				lr["query"] = lq
				return fmt.Sprintf("query key %s added to route %s (asked by visitors)", k.Add.Key, k.Add.RouteID), nil
			}
		}
	}
	return "", errNotListed
}
