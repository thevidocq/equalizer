package assistant

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Cards are what the person sees next to the answer, built by the panel
// from the logs and the map: the model only says which one to show, so a
// number, a path or a rule in a card is never the model's.

// MaxCards per answer.
const MaxCards = 4

// EventCard is one security event.
type EventCard struct {
	Kind      string    `json:"kind"` // event
	TS        time.Time `json:"ts"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Status    int       `json:"status"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Route     string    `json:"route"`
	Field     string    `json:"field"`
	RequestID string    `json:"request_id"`
	Visitor   string    `json:"visitor"` // the code the answer uses
}

// RefusalRow is one reason of refusal.
type RefusalRow struct {
	Type     string `json:"type"`
	Reason   string `json:"reason"`
	Requests int    `json:"requests"`
	Visitors int    `json:"visitors"`
}

// RefusalsCard sums up the refusals of a period.
type RefusalsCard struct {
	Kind    string       `json:"kind"` // refusals
	Since   string       `json:"since"`
	Total   int          `json:"total"`
	Partial bool         `json:"partial"` // more events than were read
	Rows    []RefusalRow `json:"rows"`
}

// Check is what a route checks on one value.
type Check struct {
	Name  string `json:"name"`
	Where string `json:"where"` // query, path, form
	Rule  string `json:"rule"`
}

// RouteCard is one entry of the map.
type RouteCard struct {
	Kind    string  `json:"kind"` // route
	Version int     `json:"version"`
	ID      string  `json:"id"`
	Method  string  `json:"method"`
	Path    string  `json:"path"`
	Checks  []Check `json:"checks"`
}

var (
	refusalTypes = []string{"request.blocked", "field.rejected", "ratelimit.hit"}
	requestIDRe  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,64}$`)
)

// card builds the card the model asked for; the text is what the model
// learns of it.
func card(ctx context.Context, src Source, m *Masker, in map[string]any) (any, string, error) {
	kind, _ := in["kind"].(string)
	switch kind {
	case "event":
		id, _ := in["request_id"].(string)
		if !requestIDRe.MatchString(id) {
			return nil, "", fmt.Errorf("request_id: one from search_logs (id=…)")
		}
		evs, err := src.Logs(ctx, "30d", nil, id)
		if err != nil {
			return nil, "", logErr(err)
		}
		for _, e := range evs {
			if e.RequestID == id {
				c := EventCard{Kind: "event", TS: e.TS, Type: e.Type, Reason: e.Reason, Status: e.Status, Method: e.Method,
					Path: e.Path, Route: e.Route, Field: e.Field, RequestID: e.RequestID, Visitor: m.Visitor(e.IP)}
				return c, fmt.Sprintf("event %s %s %s", e.Type, e.Method, m.Text(e.Path)), nil
			}
		}
		return nil, "", fmt.Errorf("no event with request id %s in the last 30 days", id)
	case "refusals":
		since, _ := in["since"].(string)
		switch since {
		case "":
			since = "24h"
		case "1h", "24h", "7d", "30d":
		default:
			return nil, "", fmt.Errorf("since: 1h, 24h, 7d or 30d")
		}
		evs, err := src.Logs(ctx, since, refusalTypes, "")
		if err != nil {
			return nil, "", logErr(err)
		}
		return refusals(evs, since), fmt.Sprintf("%d refusals in the last %s", len(evs), since), nil
	case "route":
		id, _ := in["route_id"].(string)
		doc, v, _, err := src.Map(ctx, intOf(in["version"], 0))
		if err != nil {
			return nil, "", err
		}
		for _, x := range list(doc["routes"]) {
			r, _ := x.(map[string]any)
			if r != nil && r["id"] == id {
				c := RouteCard{Kind: "route", Version: v, ID: id, Method: str(r["method"]), Path: str(r["path"]),
					Checks: checks(r, doc)}
				return c, fmt.Sprintf("route %s of version %d", id, v), nil
			}
		}
		return nil, "", fmt.Errorf("version %d has no route %q", v, id)
	}
	return nil, "", fmt.Errorf("kind: event, refusals or route")
}

func logErr(err error) error {
	if err == ErrNoLogs {
		return fmt.Errorf("this platform keeps no logs the assistant can read")
	}
	return fmt.Errorf("the log store does not answer; try again in a moment")
}

func refusals(evs []Event, since string) RefusalsCard {
	type key struct{ t, r string }
	reqs, who := map[key]int{}, map[key]map[string]bool{}
	for _, e := range evs {
		k := key{e.Type, e.Reason}
		reqs[k]++
		if who[k] == nil {
			who[k] = map[string]bool{}
		}
		who[k][e.IP] = true
	}
	c := RefusalsCard{Kind: "refusals", Since: since, Total: len(evs), Partial: len(evs) >= 500, Rows: []RefusalRow{}}
	for k, n := range reqs {
		c.Rows = append(c.Rows, RefusalRow{Type: k.t, Reason: k.r, Requests: n, Visitors: len(who[k])})
	}
	sort.Slice(c.Rows, func(i, j int) bool {
		a, b := c.Rows[i], c.Rows[j]
		return a.Requests > b.Requests || a.Requests == b.Requests && a.Type+a.Reason < b.Type+b.Reason
	})
	if len(c.Rows) > 12 {
		c.Rows = c.Rows[:12]
	}
	return c
}

func types(doc map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	t, _ := doc["types"].(map[string]any)
	for name, v := range t {
		if d, ok := v.(map[string]any); ok {
			out[name] = d
		}
	}
	return out
}

func checks(r, doc map[string]any) []Check {
	named := types(doc)
	out := []Check{}
	add := func(fields map[string]any, where string) {
		names := make([]string, 0, len(fields))
		for n := range fields {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			f, _ := fields[n].(map[string]any)
			out = append(out, Check{Name: n, Where: where, Rule: rule(f, named)})
		}
	}
	params, _ := r["params"].(map[string]any)
	query, _ := r["query"].(map[string]any)
	add(params, "path")
	add(query, "query")
	for _, x := range list(doc["forms"]) {
		form, _ := x.(map[string]any)
		if form != nil && form["route"] == r["id"] {
			fields, _ := form["fields"].(map[string]any)
			add(fields, "form "+str(form["id"]))
		}
	}
	return out
}

// rule says in words what a field spec lets through.
func rule(spec map[string]any, named map[string]map[string]any) string {
	f := map[string]any{}
	for k, v := range spec {
		f[k] = v
	}
	t := str(f["type"])
	base := t
	if d, ok := named[t]; ok { // a type of the map's own: its base and limits
		base = str(d["base"])
		for k, v := range d {
			if _, set := f[k]; !set && k != "base" {
				f[k] = v
			}
		}
	}
	var parts []string
	switch base {
	case "int":
		parts = append(parts, "whole number")
		if f["min"] != nil || f["max"] != nil {
			parts = append(parts, fmt.Sprintf("%v to %v", orAny(f["min"], "…"), orAny(f["max"], "…")))
		}
	case "enum":
		vals := list(f["values"])
		s := make([]string, 0, len(vals))
		for i, v := range vals {
			if i == 8 {
				s = append(s, fmt.Sprintf("+%d more", len(vals)-8))
				break
			}
			s = append(s, str(v))
		}
		parts = append(parts, "one of "+strings.Join(s, ", "))
	case "date", "bool", "email", "password", "file":
		parts = append(parts, base)
	default:
		parts = append(parts, "text")
	}
	if n, ok := f["max_length"].(float64); ok {
		parts = append(parts, fmt.Sprintf("%d characters at most", int(n)))
	}
	if f["pattern"] != nil {
		parts = append(parts, "with a pattern")
	}
	for _, r := range list(f["rules"]) {
		parts = append(parts, strings.ReplaceAll(str(r), "_", " "))
	}
	if t != base {
		parts = append(parts, "type "+t)
	}
	if req, _ := f["required"].(bool); req {
		parts = append(parts, "required")
	}
	return strings.Join(parts, ", ")
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func orAny(v any, fallback string) any {
	if v == nil {
		return fallback
	}
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f)
	}
	return v
}

// cardTool is the tool that shows a card.
func cardTool() Tool {
	return Tool{Name: "show_card", Description: "Shows the person a card built by the panel from the real data, next to your answer: kind event (request_id from search_logs, id=…), refusals (since: a table of the refusals by reason with requests and distinct visitors), route (route_id, optional version: what the entry checks). Show the facts with a card instead of repeating them; at most 4 per answer.",
		Schema: obj(map[string]any{
			"kind":       map[string]any{"type": "string", "enum": []string{"event", "refusals", "route"}},
			"request_id": map[string]any{"type": "string"},
			"since":      map[string]any{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}},
			"route_id":   map[string]any{"type": "string"},
			"version":    map[string]any{"type": "integer"},
		}, "kind")}
}
