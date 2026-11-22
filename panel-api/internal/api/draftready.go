package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/alerts"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// draftReady tells the tenant that a discovery's draft is checked and
// waits to be published: once the agent's verification (and correction)
// is over, and only while the version is still a draft.
func (s *Server) draftReady(ctx context.Context, a store.App, version int, score json.RawMessage, failure *string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	v, err := s.Store.Version(ctx, a.ID, version)
	if err != nil || v.Status != "draft" {
		return
	}
	tenants, err := s.Store.Tenants(ctx, &a.Tenant)
	if err != nil || len(tenants) == 0 {
		return
	}
	d := alerts.Draft{Tenant: tenants[0], App: a, Version: version, Measure: measure(score, failure)}
	if list, err := s.Store.Discoveries(ctx, a.ID); err == nil {
		for _, disc := range list {
			if disc.DraftVersion != nil && *disc.DraftVersion == version {
				if full, err := s.Store.Discovery(ctx, a.ID, disc.ID); err == nil {
					d.Lines = draftLines(full.Report) // the list leaves the report out
				}
				break
			}
		}
	}
	alerts.DraftReady(ctx, s.Store, s.Mail, s.PanelURL, d, time.Now(), s.Log)
}

// draftLines says in words what a discovery's report changes.
func draftLines(raw json.RawMessage) []string {
	var r struct {
		Routes   []json.RawMessage `json:"routes_added"`
		Forms    []json.RawMessage `json:"forms_added"`
		Visitors []struct {
			Kind  string `json:"kind"`
			Path  string `json:"path"`
			Route string `json:"route"`
			Key   string `json:"key"`
			Why   string `json:"why"`
		} `json:"visitors_added"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	var out []string
	if len(r.Routes) > 0 || len(r.Forms) > 0 {
		out = append(out, fmt.Sprintf("%d pages and %d forms with precise checks", len(r.Routes), len(r.Forms)))
	}
	for i, v := range r.Visitors {
		if i == 20 {
			out = append(out, fmt.Sprintf("and %d more from visitors' requests", len(r.Visitors)-i))
			break
		}
		what := v.Path
		if v.Kind == "key" {
			what = v.Route + " ?" + v.Key + "="
		}
		out = append(out, fmt.Sprintf("from visitors' requests: %s (%s)", what, v.Why))
	}
	return out
}

// measure is the agent's check of the draft, in words.
func measure(score json.RawMessage, failure *string) string {
	if failure != nil {
		return "it could not finish (" + *failure + "): look at the draft before publishing"
	}
	var sc struct {
		Total     int `json:"functions_total"`
		Identical int `json:"functions_identical"`
	}
	if json.Unmarshal(score, &sc) != nil || sc.Total == 0 {
		return "done"
	}
	return fmt.Sprintf("%d of %d pages work as on the site", sc.Identical, sc.Total)
}
