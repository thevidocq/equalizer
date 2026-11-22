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
// A report item with a verdict comes from an agent before 0.2.11, which
// listed visitors' requests without checking them on the site: never added.
type adoptReport struct {
	Files []struct {
		Path    string  `json:"path"`
		Verdict *string `json:"verdict"`
		Add     *struct {
			Prefix string `json:"prefix"`
			Type   string `json:"type"`
		} `json:"add"`
	} `json:"files_refused"`
	Keys []struct {
		Route   string  `json:"route"`
		Key     string  `json:"key"`
		Verdict *string `json:"verdict"`
		Add     *struct {
			RouteID string `json:"route_id"`
			Key     string `json:"key"`
		} `json:"add"`
	} `json:"query_dropped"`
}

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
	base, fromDraft, code := s.adoptBase(r, a.ID, d)
	if code != "" {
		fail(w, http.StatusConflict, code, adoptConflicts[code])
		return
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		s.storeError(w, r, err)
		return
	}
	note, changed, err := adoptApply(doc, rep, in)
	switch {
	case errors.Is(err, errNotListed):
		fail(w, http.StatusUnprocessableEntity, "not_listed", "this item is not one the discovery listed with a change to add")
		return
	case err != nil:
		fail(w, http.StatusConflict, "not_in_map", err.Error())
		return
	case !changed:
		fail(w, http.StatusConflict, "already_there", "version "+strconv.Itoa(base.Version)+" already has it")
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
	// the new draft takes the discovery's place, whatever it was built on:
	// the next click adds to it, and the person reviews one draft with every change
	var moved error
	if fromDraft {
		moved = s.Store.Supersede(r.Context(), a.ID, base.Version, version)
	} else {
		moved = s.Store.PointDiscovery(r.Context(), a.ID, d.ID, d.DraftVersion, version)
	}
	if moved != nil {
		_ = s.Store.RetireDraft(r.Context(), a.ID, version)
		fail(w, http.StatusConflict, "draft_taken", adoptConflicts["draft_taken"])
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map.adopted", a.ID,
		map[string]any{"version": version, "from": base.Version, "discovery": d.ID, "kind": in.Kind})
	writeJSON(w, http.StatusCreated, map[string]any{"app": a.ID, "version": version, "status": "draft"})
}

var adoptConflicts = map[string]string{
	"checking":    "the agent is still checking and correcting its draft: add it once the draft to review is shown",
	"in_review":   "the discovery's draft is under review: add it once the review is decided",
	"no_base":     "no draft of this discovery nor published map to add it to",
	"draft_taken": "the draft was taken on meanwhile (submitted or replaced): open the discovery again",
}

// adoptBase is the version the change goes on: the discovery's draft while
// it is one (fromDraft), else the published map; or a conflict code.
func (s *Server) adoptBase(r *http.Request, app string, d store.Discovery) (store.MapVersion, bool, string) {
	if d.DraftVersion != nil {
		if v, err := s.Store.Version(r.Context(), app, *d.DraftVersion); err == nil {
			switch v.Status {
			case "draft":
				if vf, err := s.Store.LatestVerification(r.Context(), app, v.Version); err == nil &&
					(vf.Status == "queued" || vf.Status == "running") {
					return v, false, "checking"
				}
				return v, true, ""
			case "in_review", "approved":
				return v, false, "in_review"
			}
		}
	}
	v, err := s.Store.Published(r.Context(), app)
	if err != nil {
		return v, false, "no_base"
	}
	return v, false, ""
}

// adoptApply changes doc as the report's item says: the note describing it,
// whether anything changed, errNotListed for an item the report does not
// propose, another error when the map no longer has its prefix or route.
func adoptApply(doc map[string]any, rep adoptReport, in adoptIn) (string, bool, error) {
	switch in.Kind {
	case "file":
		for _, f := range rep.Files {
			if f.Path != in.Path || f.Add == nil || f.Verdict != nil {
				continue
			}
			assets, _ := doc["assets"].([]any)
			for _, x := range assets {
				asset, _ := x.(map[string]any)
				if asset == nil || asset["prefix"] != f.Add.Prefix {
					continue
				}
				types, _ := asset["types"].([]any)
				if slices.Contains(types, any(f.Add.Type)) {
					return "", false, nil
				}
				asset["types"] = append(types, f.Add.Type)
				return fmt.Sprintf("%s added to %s, for %s (asked by visitors)", f.Add.Type, f.Add.Prefix, f.Path), true, nil
			}
			return "", false, fmt.Errorf("the map no longer has the asset prefix %s", f.Add.Prefix)
		}
	case "key":
		for _, k := range rep.Keys {
			if k.Route != in.Route || k.Key != in.Key || k.Add == nil || k.Verdict != nil || !adoptName.MatchString(k.Add.Key) {
				continue
			}
			routes, _ := doc["routes"].([]any)
			for _, x := range routes {
				route, _ := x.(map[string]any)
				if route == nil || route["id"] != k.Add.RouteID {
					continue
				}
				return adoptKey(route, k.Add.Key)
			}
			return "", false, fmt.Errorf("the map no longer has the route %s", k.Add.RouteID)
		}
	}
	return "", false, errNotListed
}

// adoptKey declares key on a GET route whose legacy request is a GET; never
// over a path parameter nor a legacy query name already mapped.
func adoptKey(route map[string]any, key string) (string, bool, error) {
	lr, _ := route["legacy_request"].(map[string]any)
	if lr == nil || (route["method"] != nil && route["method"] != "GET") || (lr["method"] != nil && lr["method"] != "GET") {
		return "", false, fmt.Errorf("route %v does not take query keys", route["id"])
	}
	q, _ := route["query"].(map[string]any)
	params, _ := route["params"].(map[string]any)
	lq, _ := lr["query"].(map[string]any)
	if _, ok := q[key]; ok {
		return "", false, nil
	}
	if _, ok := params[key]; ok {
		return "", false, fmt.Errorf("%s is a path parameter of route %v", key, route["id"])
	}
	if _, ok := lq[key]; ok {
		return "", false, fmt.Errorf("the site already gets a %s query key on route %v", key, route["id"])
	}
	if q == nil {
		q = map[string]any{}
	}
	if lq == nil {
		lq = map[string]any{}
	}
	q[key] = map[string]any{"type": "string", "max_length": 200, "required": false, "rules": []any{"no_html", "no_template_meta"}}
	lq[key] = "{{query." + key + "}}"
	route["query"], lr["query"] = q, lq
	return fmt.Sprintf("query key %s added to route %v (asked by visitors)", key, route["id"]), true, nil
}
